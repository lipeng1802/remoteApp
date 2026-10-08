//go:build windows

package nativejournal

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var advapi = windows.NewLazySystemDLL("advapi32.dll")
var credRead = advapi.NewProc("CredReadW")
var credWrite = advapi.NewProc("CredWriteW")
var credDelete = advapi.NewProc("CredDeleteW")
var credEnumerate = advapi.NewProc("CredEnumerateW")
var credFree = advapi.NewProc("CredFree")

type credential struct {
	Flags, Type             uint32
	Target, Comment         *uint16
	Written                 windows.Filetime
	Size                    uint32
	Blob                    *byte
	Persist, AttributeCount uint32
	Attributes              uintptr
	Alias, User             *uint16
}
type windowsValues struct {
	prefix string
	mutex  windows.Handle
	test   bool
}

func openNative(scope string, test bool) (keyValues, error) {
	u, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return nil, ErrUnavailable
	}
	sid := u.User.Sid.String()
	// Global namespace serializes the same user's durable credentials across
	// desktop sessions. Other users receive no mutex access, only this SID.
	name, _ := windows.UTF16PtrFromString("Global\\RemoteAppTrustJournal-" + sid + "-" + Digest([]byte(namespace(scope, test))))
	sd, e := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + sid + ")")
	if e != nil {
		return nil, ErrUnavailable
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	m, e := windows.CreateMutex(sa, false, name)
	if e != nil && e != windows.ERROR_ALREADY_EXISTS {
		return nil, ErrUnavailable
	}
	status, e := windows.WaitForSingleObject(m, 0)
	if e != nil || (status != windows.WAIT_OBJECT_0 && status != windows.WAIT_ABANDONED) {
		windows.CloseHandle(m)
		return nil, ErrUnavailable
	}
	return &windowsValues{namespace(scope, test), m, test}, nil
}

func credError(e error) error {
	if e == windows.ERROR_NOT_FOUND {
		return ErrMissing
	}
	if e == windows.ERROR_NO_SUCH_LOGON_SESSION {
		return ErrLogon
	}
	return ErrUnavailable
}

func (w *windowsValues) Read(name string) ([]byte, error) {
	if !validName(name) {
		return nil, ErrUnavailable
	}
	t, _ := windows.UTF16PtrFromString(w.prefix + name)
	var p *credential
	ok, _, e := credRead.Call(uintptr(unsafe.Pointer(t)), 1, 0, uintptr(unsafe.Pointer(&p)))
	if ok == 0 {
		return nil, credError(e)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(p)))
	if p == nil || p.Type != 1 || p.Persist != 2 || p.Size == 0 || p.Size > chunkSize || p.Blob == nil || windows.UTF16PtrToString(p.Target) != w.prefix+name {
		return nil, ErrUnavailable
	}
	return append([]byte(nil), unsafe.Slice(p.Blob, int(p.Size))...), nil
}
func (w *windowsValues) Write(name string, b []byte) error {
	if !validName(name) || len(b) < 1 || len(b) > chunkSize {
		return ErrUnavailable
	}
	t, _ := windows.UTF16PtrFromString(w.prefix + name)
	u, _ := windows.UTF16PtrFromString("RemoteApp Trust Journal")
	c := credential{Type: 1, Target: t, Size: uint32(len(b)), Blob: &b[0], Persist: 2, User: u}
	ok, _, e := credWrite.Call(uintptr(unsafe.Pointer(&c)), 0)
	if ok == 0 {
		return credError(e)
	}
	return nil
}
func (w *windowsValues) Remove(name string) error {
	if !validName(name) {
		return ErrUnavailable
	}
	t, _ := windows.UTF16PtrFromString(w.prefix + name)
	ok, _, e := credDelete.Call(uintptr(unsafe.Pointer(t)), 1, 0)
	if ok == 0 {
		return credError(e)
	}
	return nil
}
func (w *windowsValues) names() ([]string, error) {
	t, _ := windows.UTF16PtrFromString(w.prefix + "*")
	var count uint32
	var list **credential
	ok, _, e := credEnumerate.Call(uintptr(unsafe.Pointer(t)), 0, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&list)))
	if ok == 0 {
		if e == windows.ERROR_NOT_FOUND {
			return nil, nil
		}
		return nil, credError(e)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(list)))
	if count > 128 || (count > 0 && list == nil) {
		return nil, ErrUnavailable
	}
	var names []string
	for _, c := range unsafe.Slice(list, int(count)) {
		if c == nil {
			return nil, ErrUnavailable
		}
		name := windows.UTF16PtrToString(c.Target)
		if !strings.HasPrefix(name, w.prefix) || !validName(strings.TrimPrefix(name, w.prefix)) {
			return nil, ErrUnavailable
		}
		names = append(names, strings.TrimPrefix(name, w.prefix))
	}
	return names, nil
}
func (w *windowsValues) Empty() (bool, error) { names, e := w.names(); return len(names) == 0, e }
func (w *windowsValues) ClearTest() error {
	if !w.test {
		return ErrUnavailable
	}
	names, e := w.names()
	if e != nil {
		return e
	}
	for _, name := range names {
		if w.Remove(name) != nil {
			return ErrUnavailable
		}
	}
	return nil
}
func (w *windowsValues) Close() error {
	e := windows.ReleaseMutex(w.mutex)
	if closed := windows.CloseHandle(w.mutex); e == nil {
		e = closed
	}
	return e
}
