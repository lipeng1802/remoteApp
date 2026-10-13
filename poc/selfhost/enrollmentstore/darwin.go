//go:build darwin

package enrollmentstore

import (
	"os"
	"os/user"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type macDirectory struct {
	path             string
	app, scope, node int
}

func privateDir(fd int) bool {
	var st unix.Stat_t
	return unix.Fstat(fd, &st) == nil && st.Uid == uint32(os.Getuid()) && st.Mode&unix.S_IFMT == unix.S_IFDIR && st.Mode&0777 == 0700
}
func childDir(parent int, name string, create, exclusive bool) (int, error) {
	if create {
		e := unix.Mkdirat(parent, name, 0700)
		if e != nil && (exclusive || e != unix.EEXIST) {
			return -1, ErrStorage
		}
		if e == nil && unix.Fsync(parent) != nil {
			return -1, ErrStorage
		}
	}
	fd, e := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return -1, ErrStorage
	}
	if !privateDir(fd) {
		unix.Close(fd)
		return -1, ErrStorage
	}
	return fd, nil
}
func openDirectory(scope string, test, create bool) (directory, error) {
	u, e := user.Current()
	if e != nil || u.HomeDir == "" {
		return nil, ErrStorage
	}
	home, e := unix.Open(u.HomeDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, ErrStorage
	}
	defer unix.Close(home)
	library, e := unix.Openat(home, "Library", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, ErrStorage
	}
	defer unix.Close(library)
	base, e := unix.Openat(library, "Application Support", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, ErrStorage
	}
	defer unix.Close(base)
	name := "RemoteAppEnrollment-v1"
	if test {
		name = "RemoteAppEnrollmentTest-v1"
	}
	a, e := childDir(base, name, create, false)
	if e != nil {
		return nil, e
	}
	d, e := childDir(a, scope, create, true)
	if e != nil {
		unix.Close(a)
		return nil, e
	}
	n, e := childDir(d, "node", create, true)
	if e != nil {
		unix.Close(d)
		unix.Close(a)
		return nil, e
	}
	return &macDirectory{filepath.Join(u.HomeDir, "Library", "Application Support", name, scope, "node"), a, d, n}, nil
}
func sameDirectory(parent int, name string, fd int) bool {
	var a, b unix.Stat_t
	return unix.Fstat(fd, &a) == nil && unix.Fstatat(parent, name, &b, unix.AT_SYMLINK_NOFOLLOW) == nil && a.Dev == b.Dev && a.Ino == b.Ino && b.Mode&unix.S_IFMT == unix.S_IFDIR
}
func (d *macDirectory) Path() string { return d.path }
func (d *macDirectory) Check(empty bool) error {
	if !privateDir(d.app) || !privateDir(d.scope) || !privateDir(d.node) || !sameDirectory(d.scope, "node", d.node) || !sameDirectory(d.app, filepath.Base(filepath.Dir(d.path)), d.scope) {
		return ErrStorage
	}
	var st unix.Stat_t
	var current unix.Stat_t
	if unix.Fstat(d.node, &st) != nil || unix.Lstat(d.path, &current) != nil || st.Dev != current.Dev || st.Ino != current.Ino {
		return ErrStorage
	}
	count := 0
	e := filepath.WalkDir(d.path, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return ErrStorage
		}
		count++
		if count > 2048 {
			return ErrStorage
		}
		var st unix.Stat_t
		if unix.Lstat(path, &st) != nil || st.Uid != uint32(os.Getuid()) {
			return ErrStorage
		}
		if entry.IsDir() {
			if st.Mode&0777 != 0700 {
				return ErrStorage
			}
		} else {
			if empty || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&0777 != 0600 || st.Nlink != 1 {
				return ErrStorage
			}
		}
		if empty && path != d.path {
			return ErrStorage
		}
		return nil
	})
	if e != nil {
		return ErrStorage
	}
	return nil
}
func (d *macDirectory) Close() error {
	e := unix.Close(d.node)
	if v := unix.Close(d.scope); e == nil {
		e = v
	}
	if v := unix.Close(d.app); e == nil {
		e = v
	}
	return e
}

func (d *macDirectory) ClearTestMaterials() error {
	entries, e := os.ReadDir(d.path)
	if e != nil || len(entries) > 1 {
		return ErrStorage
	}
	if len(entries) == 1 {
		if entries[0].Name() != "tailscaled.state" || d.Check(false) != nil {
			return ErrStorage
		}
		if unix.Unlinkat(d.node, "tailscaled.state", 0) != nil || unix.Fsync(d.node) != nil {
			return ErrStorage
		}
	}
	return nil
}
