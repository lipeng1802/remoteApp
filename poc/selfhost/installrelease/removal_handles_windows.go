//go:build windows

package installrelease

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func platformRemovalOpen(root, relative string, parent *os.File, directory bool) (*os.File, error) {
	if relative != "." && parent == nil {
		return nil, ErrRelease
	}
	p := filepath.Join(root, filepath.FromSlash(relative))
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return nil, ErrRelease
	}
	access := uint32(windows.GENERIC_READ)
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	if !directory {
		access |= windows.DELETE
		share = windows.FILE_SHARE_READ
	}
	h, err := windows.CreateFile(name, access, share, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), p)
	// No FILE_SHARE_DELETE: every directory ancestor remains fixed while the
	// batch runs. File handles also deny new/existing writers and renamers.
	return f, nil
}

func platformRemovalDelete(_ *os.File, file *os.File, _ string) error {
	// Win11 supports Ex flags including read-only signed payload files. No
	// path-based DeleteFile fallback or chmod of a reopened path is permitted.
	flags := uint32(windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS | windows.FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE)
	if windows.SetFileInformationByHandle(windows.Handle(file.Fd()), windows.FileDispositionInfoEx, (*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags))) != nil {
		return ErrRelease
	}
	return nil
}
