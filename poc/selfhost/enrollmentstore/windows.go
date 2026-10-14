//go:build windows

package enrollmentstore

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

type winDirectory struct {
	path             string
	sid              *windows.SID
	app, scope, node windows.Handle
}
type aclHeader struct {
	revision, sbz     byte
	size, count, sbz2 uint16
}

func protected(h windows.Handle, sid *windows.SID, root bool) bool {
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return false
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 && info.NumberOfLinks != 1 {
		return false
	}
	sd, e := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if e != nil {
		return false
	}
	owner, _, e := sd.Owner()
	if e != nil || !windows.EqualSid(owner, sid) {
		return false
	}
	control, _, e := sd.Control()
	if e != nil || (root && control&windows.SE_DACL_PROTECTED == 0) {
		return false
	}
	acl, _, e := sd.DACL()
	if e != nil || acl == nil || (*aclHeader)(unsafe.Pointer(acl)).count != 1 {
		return false
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if windows.GetAce(acl, 0, &ace) != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&8 != 0 || ace.Mask != 0x1f01ff {
		return false
	}
	return windows.EqualSid((*windows.SID)(unsafe.Pointer(&ace.SidStart)), sid)
}
func openProtected(path string, sid *windows.SID, root bool) (windows.Handle, error) {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return 0, ErrStorage
	}
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	if !root {
		share |= windows.FILE_SHARE_DELETE
	}
	h, e := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES|windows.READ_CONTROL, share, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if e != nil {
		return 0, ErrStorage
	}
	if !protected(h, sid, root) {
		windows.CloseHandle(h)
		return 0, ErrStorage
	}
	return h, nil
}
func makeProtected(path string, sa *windows.SecurityAttributes, sid *windows.SID, create, exclusive bool) (windows.Handle, error) {
	if create {
		p, e := windows.UTF16PtrFromString(path)
		if e != nil {
			return 0, ErrStorage
		}
		e = windows.CreateDirectory(p, sa)
		if e != nil && (exclusive || e != windows.ERROR_ALREADY_EXISTS) {
			return 0, ErrStorage
		}
	}
	h, e := openProtected(path, sid, true)
	if e != nil {
		return 0, e
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		windows.CloseHandle(h)
		return 0, ErrStorage
	}
	return h, nil
}
func openDirectory(scope string, test, create bool) (directory, error) {
	u, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return nil, ErrStorage
	}
	sid := u.User.Sid
	// Explicit owner and protected inheritable DACL. No Everyone/Admin/System
	// ACE, no ACL repair of pre-existing directories, no APPDATA/HOME override.
	sd, e := windows.SecurityDescriptorFromString("O:" + sid.String() + "D:P(A;OICI;FA;;;" + sid.String() + ")")
	if e != nil {
		return nil, ErrStorage
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	base, e := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, windows.KF_FLAG_DEFAULT)
	if e != nil || !filepath.IsAbs(base) {
		return nil, ErrStorage
	}
	name := "RemoteAppEnrollment-v1"
	if test {
		name = "RemoteAppEnrollmentTest-v1"
	}
	appPath := filepath.Join(base, name)
	a, e := makeProtected(appPath, sa, sid, create, false)
	if e != nil {
		return nil, e
	}
	scopePath := filepath.Join(appPath, scope)
	s, e := makeProtected(scopePath, sa, sid, create, true)
	if e != nil {
		windows.CloseHandle(a)
		return nil, e
	}
	nodePath := filepath.Join(scopePath, "node")
	n, e := makeProtected(nodePath, sa, sid, create, true)
	if e != nil {
		windows.CloseHandle(s)
		windows.CloseHandle(a)
		return nil, e
	}
	return &winDirectory{nodePath, sid, a, s, n}, nil
}
func (d *winDirectory) Path() string { return d.path }
func (d *winDirectory) Check(empty bool) error {
	for _, h := range []windows.Handle{d.app, d.scope, d.node} {
		if !protected(h, d.sid, true) {
			return ErrStorage
		}
	}
	count := 0
	e := filepath.WalkDir(d.path, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return ErrStorage
		}
		count++
		if count > 2048 || (empty && path != d.path) {
			return ErrStorage
		}
		h, e := openProtected(path, d.sid, path == d.path)
		if e != nil {
			return e
		}
		defer windows.CloseHandle(h)
		if path == d.path {
			var a, b windows.ByHandleFileInformation
			if windows.GetFileInformationByHandle(h, &a) != nil || windows.GetFileInformationByHandle(d.node, &b) != nil || a.VolumeSerialNumber != b.VolumeSerialNumber || a.FileIndexHigh != b.FileIndexHigh || a.FileIndexLow != b.FileIndexLow {
				return ErrStorage
			}
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrStorage
		}
		return nil
	})
	if e != nil {
		return ErrStorage
	}
	return nil
}
func (d *winDirectory) Close() error {
	e := windows.CloseHandle(d.node)
	if v := windows.CloseHandle(d.scope); e == nil {
		e = v
	}
	if v := windows.CloseHandle(d.app); e == nil {
		e = v
	}
	return e
}

func (d *winDirectory) ClearTestMaterials() error {
	if d.Check(false) != nil {
		return ErrStorage
	}
	plan, e := sdkCleanupPlan(d.path)
	if e != nil {
		return ErrStorage
	}
	for _, entry := range plan {
		if os.Remove(filepath.Join(d.path, entry.path)) != nil {
			return ErrStorage
		}
	}
	return nil
}
