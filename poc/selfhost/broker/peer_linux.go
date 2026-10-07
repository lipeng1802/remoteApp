//go:build linux

package broker

import (
	"golang.org/x/sys/unix"
	"net"
	"os"
	"syscall"
)

func peerUID(c *net.UnixConn) (uint32, error) {
	raw, e := c.SyscallConn()
	if e != nil {
		return 0, ErrBroker
	}
	var uid uint32
	var failed error
	if raw.Control(func(fd uintptr) {
		u, e := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if e != nil {
			failed = e
		} else {
			uid = u.Uid
		}
	}) != nil || failed != nil {
		return 0, ErrBroker
	}
	return uid, nil
}
func safeDirectory(path string, gid uint32) bool {
	i, e := os.Lstat(path)
	if e != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0022 != 0 {
		return false
	}
	s, ok := i.Sys().(*syscall.Stat_t)
	return ok && s.Uid == uint32(os.Geteuid()) && s.Gid == gid
}
