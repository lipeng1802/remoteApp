//go:build windows

package installrelease

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func platformPolicy(test bool) string {
	if test {
		return "private-current-user-v1"
	}
	return "administrators-write-users-read-execute-v1"
}

func platformProtectDirectory(path string, test bool) error {
	owner := "BA"
	if test {
		token, err := windows.OpenCurrentProcessToken()
		if err != nil {
			return ErrRelease
		}
		defer token.Close()
		user, err := token.GetTokenUser()
		if err != nil || user == nil || user.User.Sid == nil || broadSID(user.User.Sid) {
			return ErrRelease
		}
		owner = user.User.Sid.String()
	}
	access := fmt.Sprintf("(A;OICI;FA;;;%s)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", owner)
	if !test {
		access += "(A;OICI;GRGX;;;BU)"
	}
	descriptor, err := windows.SecurityDescriptorFromString("O:" + owner + "G:" + owner + "D:P" + access)
	if err != nil {
		return ErrRelease
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		return ErrRelease
	}
	ownerSID, _, err := descriptor.Owner()
	if err != nil || ownerSID == nil {
		return ErrRelease
	}
	groupSID, _, err := descriptor.Group()
	if err != nil || groupSID == nil {
		return ErrRelease
	}
	err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		ownerSID, groupSID, dacl, nil)
	runtime.KeepAlive(descriptor)
	if err != nil {
		return ErrRelease
	}
	return nil
}

func broadSID(sid *windows.SID) bool {
	for _, kind := range []windows.WELL_KNOWN_SID_TYPE{windows.WinWorldSid, windows.WinAuthenticatedUserSid, windows.WinBuiltinUsersSid} {
		known, err := windows.CreateWellKnownSid(kind)
		if err != nil {
			return true
		}
		if sid.Equals(known) {
			return true
		}
	}
	return false
}

func protectedDACL(file *os.File, test bool) bool {
	sd, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		return false
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || broadSID(owner) {
		return false
	}
	if !test {
		system, systemErr := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
		administrators, adminErr := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
		if systemErr != nil || adminErr != nil || (!owner.Equals(system) && !owner.Equals(administrators)) {
			return false
		}
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return false
	}
	write := windows.ACCESS_MASK(windows.GENERIC_ALL | windows.GENERIC_WRITE | windows.FILE_GENERIC_WRITE | windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER)
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(dacl, index, &ace) != nil || ace == nil {
			return false
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if broadSID(sid) && ace.Mask&write != 0 {
			return false
		}
	}
	return true
}

func platformIdentity(_ string, file *os.File, _ os.FileInfo, directory, test bool) (fileIdentity, error) {
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info) != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.NumberOfLinks < 1 {
		return fileIdentity{}, ErrRelease
	}
	if directory && !protectedDACL(file, test) {
		return fileIdentity{}, ErrRelease
	}
	return fileIdentity{volume: uint64(info.VolumeSerialNumber), index: uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow), links: uint64(info.NumberOfLinks)}, nil
}
