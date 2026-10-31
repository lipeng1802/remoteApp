//go:build windows

package installrelease

import (
	"golang.org/x/sys/windows"
	"os"
)

func platformOpenDirectoryForSync(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	// FlushFileBuffers requires GENERIC_WRITE. os.Open uses only GENERIC_READ.
	// BACKUP_SEMANTICS opens a directory without requesting backup privileges;
	// OPEN_REPARSE_POINT plus the handle check rejects redirects.
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(h, &info); err != nil ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(h)
		return nil, ErrRelease
	}
	return os.NewFile(uintptr(h), path), nil
}
