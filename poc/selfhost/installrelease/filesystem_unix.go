//go:build !windows

package installrelease

import (
	"os"
	"syscall"
)

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
