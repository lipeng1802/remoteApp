//go:build !windows

package installrelease

import (
	"os"
	"syscall"
)

func platformPolicy(test bool) string {
	if test {
		return "private-current-user-v1"
	}
	return "root-read-execute-v1"
}

func platformProtectDirectory(path string, test bool) error {
	if test {
		return os.Chmod(path, 0700)
	}
	if os.Geteuid() != 0 || os.Chown(path, 0, 0) != nil {
		return ErrRelease
	}
	return os.Chmod(path, 0755)
}

func platformIdentity(_ string, _ *os.File, info os.FileInfo, directory, test bool) (fileIdentity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink < 1 {
		return fileIdentity{}, ErrRelease
	}
	if directory {
		uid := uint32(os.Geteuid())
		if ((!test && stat.Uid != 0) || (test && stat.Uid != 0 && stat.Uid != uid)) || info.Mode().Perm()&0022 != 0 {
			return fileIdentity{}, ErrRelease
		}
	} else if info.Mode().Perm()&0022 != 0 {
		return fileIdentity{}, ErrRelease
	}
	return fileIdentity{volume: uint64(stat.Dev), index: uint64(stat.Ino), links: uint64(stat.Nlink)}, nil
}
