//go:build !windows

package installrelease

import (
	"os"
	"path"

	"golang.org/x/sys/unix"
)

func platformRemovalOpen(root, relative string, parent *os.File, directory bool) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	if directory {
		flags |= unix.O_DIRECTORY
	}
	var fd int
	var err error
	if relative == "." {
		fd, err = unix.Open(root, flags, 0)
	} else {
		if parent == nil {
			return nil, ErrRelease
		}
		fd, err = unix.Openat(int(parent.Fd()), path.Base(relative), flags, 0)
	}
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), relative), nil
}

// unlinkat never traverses a renamed/replaced ancestor. Unix still has a final
// basename check/unlink race against a writer with the same directory rights;
// production requires a protected exclusive-writer boundary, not just dirfds.
func platformRemovalDelete(parent, file *os.File, name string) error {
	if parent == nil {
		return ErrRelease
	}
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return ErrRelease
	}
	probe := os.NewFile(uintptr(fd), name)
	before, e := file.Stat()
	after, e2 := probe.Stat()
	closeErr := probe.Close()
	if e != nil || e2 != nil || closeErr != nil || !os.SameFile(before, after) {
		return ErrRelease
	}
	if unix.Unlinkat(int(parent.Fd()), name, 0) != nil {
		return ErrRelease
	}
	return nil
}
