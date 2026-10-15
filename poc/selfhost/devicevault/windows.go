//go:build windows

package devicevault

import (
	"crypto/rand"
	"errors"
	"golang.org/x/sys/windows"
	"unsafe"
)

var ErrUnavailable = errors.New("device_vault_unavailable")
var ErrLogonUnavailable = errors.New("interactive_logon_required")
var advapi = windows.NewLazySystemDLL("advapi32.dll")
var read = advapi.NewProc("CredReadW")
var write = advapi.NewProc("CredWriteW")
var free = advapi.NewProc("CredFree")
var remove = advapi.NewProc("CredDeleteW")

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

func Load(scope string, create bool) ([]byte, error) {
	name, _ := windows.UTF16PtrFromString("Local\\RemoteAppConnectionIdentity-" + scope)
	mutex, e := windows.CreateMutex(nil, false, name)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer windows.CloseHandle(mutex)
	status, e := windows.WaitForSingleObject(mutex, 2000)
	if e != nil || (status != windows.WAIT_OBJECT_0 && status != windows.WAIT_ABANDONED) {
		return nil, ErrUnavailable
	}
	defer windows.ReleaseMutex(mutex)
	target, _ := windows.UTF16PtrFromString("RemoteApp/ConnectionPoC/v1/" + scope)
	var ptr *credential
	ok, _, err := read.Call(uintptr(unsafe.Pointer(target)), 1, 0, uintptr(unsafe.Pointer(&ptr)))
	if ok != 0 {
		defer free.Call(uintptr(unsafe.Pointer(ptr)))
		c := ptr
		if c == nil || c.Size != 32 || c.Blob == nil {
			return nil, ErrUnavailable
		}
		if create {
			return nil, ErrUnavailable
		}
		return append([]byte(nil), unsafe.Slice(c.Blob, 32)...), nil
	}
	if err != windows.ERROR_NOT_FOUND || !create {
		if err == windows.ERROR_NO_SUCH_LOGON_SESSION {
			return nil, ErrLogonUnavailable
		}
		return nil, ErrUnavailable
	}
	seed := make([]byte, 32)
	if _, e = rand.Read(seed); e != nil {
		return nil, ErrUnavailable
	}
	user, _ := windows.UTF16PtrFromString("RemoteApp")
	c := credential{Type: 1, Target: target, Size: 32, Blob: &seed[0], Persist: 2, User: user}
	ok, _, err = write.Call(uintptr(unsafe.Pointer(&c)), 0)
	if ok == 0 {
		for i := range seed {
			seed[i] = 0
		}
		if err == windows.ERROR_NO_SUCH_LOGON_SESSION {
			return nil, ErrLogonUnavailable
		}
		return nil, ErrUnavailable
	}
	return seed, nil
}
func Delete(scope string) error {
	target, _ := windows.UTF16PtrFromString("RemoteApp/ConnectionPoC/v1/" + scope)
	ok, _, _ := remove.Call(uintptr(unsafe.Pointer(target)), 1, 0)
	if ok == 0 {
		return ErrUnavailable
	}
	return nil
}
