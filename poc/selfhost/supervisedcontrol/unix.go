//go:build !windows

package supervisedcontrol

import (
	"errors"
	"net"
	"os"
	"path/filepath"
)

// PrivateUnix owns a fresh same-user TEST business socket, independent of the
// inherited lifecycle stream. No existing path is removed or adopted. This is
// OS user isolation, not publisher authentication or production service setup.
type PrivateUnix struct {
	*net.UnixListener
	dir, address      string
	directory, socket os.FileInfo
}

func ListenPrivateUnix() (*PrivateUnix, error) {
	parent, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(parent, "remoteapp-session-")
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	address := filepath.Join(dir, "control.sock")
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: address, Net: "unix"})
	if err != nil {
		os.Remove(dir)
		return nil, err
	}
	ln.SetUnlinkOnClose(false)
	if err = os.Chmod(address, 0600); err != nil {
		ln.Close()
		return nil, err
	}
	socket, err := os.Lstat(address)
	if err != nil {
		ln.Close()
		return nil, err
	}
	return &PrivateUnix{UnixListener: ln, dir: dir, address: address, directory: info, socket: socket}, nil
}

func (l *PrivateUnix) Address() string { return l.address }

// Cleanup follows listener/handler join. Close deliberately does not unlink a
// replacement socket, and no RemoveAll/recursive or unowned-path cleanup occurs.
func (l *PrivateUnix) Cleanup() error {
	dir, err := os.Lstat(l.dir)
	if err != nil || !os.SameFile(dir, l.directory) {
		return errors.New("control_directory_changed")
	}
	socket, err := os.Lstat(l.address)
	if err != nil || !os.SameFile(socket, l.socket) {
		return errors.New("control_socket_changed")
	}
	if err = os.Remove(l.address); err != nil {
		return err
	}
	return os.Remove(l.dir)
}
