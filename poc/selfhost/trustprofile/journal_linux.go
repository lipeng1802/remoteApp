//go:build linux

package trustprofile

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

// LinuxRootJournal protects against non-root local users, not root, disk
// snapshots or compromised privileged software. It is not a desktop fallback.
// Create is installer-only: it exclusively creates a NEW directory. Open never
// creates missing files or interprets an incomplete installation as new.
type LinuxRootJournal struct {
	mu                    sync.Mutex
	dir, lock             int
	fresh, failed, closed bool
}

var errJournal = errors.New("protected_journal_unavailable")
var journalPending = []byte("installation_pending")

func journalRecordValid(raw []byte) bool {
	r, e := decodeInstallation(raw)
	anchor, he := hex.DecodeString(r.Anchor)
	return e == nil && he == nil && len(anchor) == 32 && r.Version == 1 && r.Epoch > 0 && r.Epoch == r.Profile.Claims.Epoch && r.AcceptedAt >= r.Profile.Claims.IssuedAt && r.AcceptedAt < r.Profile.Claims.ExpiresAt
}

// secureRootDir walks anchored descriptors; every component must be root-owned,
// not group/world writable, and not a symlink. /tmp is intentionally rejected.
func secureRootDir(path string) (int, error) {
	if unix.Geteuid() != 0 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return -1, errJournal
	}
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return -1, errJournal
	}
	var rootStat unix.Stat_t
	if unix.Fstat(fd, &rootStat) != nil || rootStat.Uid != 0 || rootStat.Mode&0022 != 0 {
		unix.Close(fd)
		return -1, errJournal
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if err != nil {
			return -1, errJournal
		}
		fd = next
		var st unix.Stat_t
		if unix.Fstat(fd, &st) != nil || st.Uid != 0 || st.Mode&0022 != 0 {
			unix.Close(fd)
			return -1, errJournal
		}
	}
	return fd, nil
}

func journalFile(dir int, name string, flags int) (int, error) {
	fd, e := unix.Openat(dir, name, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
	if e != nil {
		return -1, errJournal
	}
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil || st.Uid != 0 || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&0777 != 0600 || st.Nlink != 1 {
		unix.Close(fd)
		return -1, errJournal
	}
	return fd, nil
}

func CreateLinuxRootJournal(path string) (*LinuxRootJournal, error) {
	if filepath.Base(path) == "." || filepath.Base(path) == "/" || filepath.Clean(path) != path {
		return nil, errJournal
	}
	parent, e := secureRootDir(filepath.Dir(path))
	if e != nil {
		return nil, e
	}
	defer unix.Close(parent)
	if unix.Mkdirat(parent, filepath.Base(path), 0700) != nil {
		return nil, errJournal
	}
	// Incomplete creation is deliberately retained, never silently reset.
	if unix.Fsync(parent) != nil {
		return nil, errJournal
	}
	return openLinuxRootJournal(path, true)
}

func OpenLinuxRootJournal(path string) (*LinuxRootJournal, error) {
	return openLinuxRootJournal(path, false)
}

func openLinuxRootJournal(path string, create bool) (*LinuxRootJournal, error) {
	dir, e := secureRootDir(path)
	if e != nil {
		return nil, e
	}
	j := &LinuxRootJournal{dir: dir, lock: -1, fresh: create}
	ok := false
	defer func() {
		if !ok {
			j.Close()
		}
	}()
	var st unix.Stat_t
	if unix.Fstat(dir, &st) != nil || st.Mode&0777 != 0700 {
		return nil, errJournal
	}
	flags := unix.O_RDWR
	if create {
		flags |= unix.O_CREAT | unix.O_EXCL
	}
	j.lock, e = journalFile(dir, "owner.lock", flags)
	if e != nil || unix.Flock(j.lock, unix.LOCK_EX|unix.LOCK_NB) != nil {
		return nil, errJournal
	}
	if create {
		if unix.Fsync(j.lock) != nil {
			return nil, errJournal
		}
		fd, err := journalFile(dir, "record", unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL)
		if err != nil {
			return nil, errJournal
		}
		n, err := unix.Write(fd, journalPending)
		synced := unix.Fsync(fd)
		closed := unix.Close(fd)
		if err != nil || n != len(journalPending) || synced != nil || closed != nil || unix.Fsync(dir) != nil {
			return nil, errJournal
		}
	} else {
		if _, e = j.load(); e != nil {
			return nil, errJournal
		}
	}
	ok = true
	return j, nil
}

func (j *LinuxRootJournal) load() ([]byte, error) {
	if j.closed || j.failed {
		return nil, errJournal
	}
	var dirStat, lockStat unix.Stat_t
	if unix.Fstat(j.dir, &dirStat) != nil || dirStat.Uid != 0 || dirStat.Mode&0777 != 0700 ||
		unix.Fstat(j.lock, &lockStat) != nil || lockStat.Uid != 0 || lockStat.Mode&0777 != 0600 || lockStat.Nlink != 1 {
		return nil, errJournal
	}
	fd, e := journalFile(j.dir, "record", unix.O_RDONLY)
	if e != nil {
		return nil, e
	}
	defer unix.Close(fd)
	var b bytes.Buffer
	buf := make([]byte, 1024)
	for {
		n, err := unix.Read(fd, buf)
		if err != nil {
			return nil, errJournal
		}
		if n == 0 {
			break
		}
		b.Write(buf[:n])
		if b.Len() > 8191 {
			return nil, errJournal
		}
	}
	if j.fresh && bytes.Equal(b.Bytes(), journalPending) {
		return nil, ErrNotInstalled
	}
	if !journalRecordValid(b.Bytes()) {
		return nil, errJournal
	}
	return b.Bytes(), nil
}

func (j *LinuxRootJournal) Load() ([]byte, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	b, e := j.load()
	if e != nil && e != ErrNotInstalled {
		j.failed = true
	}
	return b, e
}

func (j *LinuxRootJournal) Commit(expected, next []byte) (err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed || j.failed {
		return errJournal
	}
	defer func() {
		if err != nil {
			j.failed = true
		}
	}()
	prior, e := j.load()
	if e != nil && !(e == ErrNotInstalled && expected == nil) || !bytes.Equal(prior, expected) {
		return errJournal
	}
	if !journalRecordValid(next) {
		return errJournal
	}
	var nonce [16]byte
	if _, e = io.ReadFull(rand.Reader, nonce[:]); e != nil {
		return errJournal
	}
	name := "pending-" + hex.EncodeToString(nonce[:])
	fd, e := journalFile(j.dir, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL)
	if e != nil {
		return errJournal
	}
	defer unix.Unlinkat(j.dir, name, 0)
	for rest := next; len(rest) > 0; {
		n, e := unix.Write(fd, rest)
		if e != nil || n == 0 {
			unix.Close(fd)
			return errJournal
		}
		rest = rest[n:]
	}
	synced := unix.Fsync(fd)
	closed := unix.Close(fd)
	if synced != nil || closed != nil {
		return errJournal
	}
	if unix.Renameat(j.dir, name, j.dir, "record") != nil || unix.Fsync(j.dir) != nil {
		return errJournal
	}
	j.fresh = false
	return nil
}

// Close releases ownership only after caller has stopped all users/sessions.
// No deletion/reset API is provided. Calls after Close are rejected.
func (j *LinuxRootJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	var e error
	if j.lock >= 0 {
		e = unix.Close(j.lock)
	}
	if err := unix.Close(j.dir); e == nil {
		e = err
	}
	return e
}
