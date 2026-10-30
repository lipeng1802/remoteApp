package trustprofile

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

var ErrDesktopLogon = errors.New("interactive_logon_required")

type JournalState string

const (
	JournalAbsent    JournalState = "absent"
	JournalPending   JournalState = "pending"
	JournalCommitted JournalState = "committed"
	JournalUnusable  JournalState = "unusable"
)

// DesktopJournal owns a dedicated native helper via inherited anonymous pipes.
// Executable/scope come from trusted installation, never peers or downloaded
// profiles. macOS/Windows use native OS credentials, without a file fallback.
type DesktopJournal struct {
	mu                    sync.Mutex
	cmd                   *exec.Cmd
	in                    io.WriteCloser
	out                   io.ReadCloser
	reader                *privatepipe.Reader
	closed, created, test bool
	wait                  chan error
	installation          bool
	waitOnce              sync.Once
}
type journalReply struct {
	Status string `json:"status"`
	Data   string `json:"data,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func NewDesktopJournal(executable, scope string, create bool) (*DesktopJournal, error) {
	mode := "open"
	if create {
		mode = "create"
	}
	return startDesktopJournal(executable, scope, mode, false, true)
}

// Separate native namespace, only for random test scopes; no production cleanup.
func NewTestDesktopJournal(executable, scope string, create bool) (*DesktopJournal, error) {
	mode := "open"
	if create {
		mode = "create"
	}
	return startDesktopJournal(executable, scope, mode, true, true)
}

// NewRecordDesktopJournal owns the same pinned native helper protocol without
// imposing the trust-profile record schema. Its caller must strictly validate
// its own record before and after every commit.
func NewRecordDesktopJournal(executable, scope string, create, test bool) (*DesktopJournal, error) {
	mode := "open"
	if create {
		mode = "create"
	}
	return startDesktopJournal(executable, scope, mode, test, false)
}

func startDesktopJournal(executable, scope, mode string, test, installation bool) (*DesktopJournal, error) {
	b, e := hex.DecodeString(scope)
	if !filepath.IsAbs(executable) || e != nil || len(b) != 16 || hex.EncodeToString(b) != scope {
		return nil, ErrTrust
	}
	c := exec.Command(executable)
	in, e := c.StdinPipe()
	if e != nil {
		return nil, ErrTrust
	}
	out, e := c.StdoutPipe()
	if e != nil {
		in.Close()
		return nil, ErrTrust
	}
	c.Stderr = io.Discard
	if c.Start() != nil {
		in.Close()
		out.Close()
		return nil, ErrTrust
	}
	j := &DesktopJournal{cmd: c, in: in, out: out, reader: privatepipe.NewBoundedReader(out, 16384), created: mode == "create", test: test, wait: make(chan error, 1), installation: installation}
	r, e := j.rpc(map[string]any{"Scope": scope, "Mode": mode, "Test": test})
	status := "ready"
	if mode == "cleanup" {
		status = "deleted"
	}
	if e != nil || r.Status != status || r.Data != "" || r.Reason != "" {
		j.abort()
		if errors.Is(e, ErrNativeHelperTimeout) {
			return nil, e
		}
		return nil, journalFailure(r.Reason)
	}
	return j, nil
}

func (j *DesktopJournal) startWait() {
	j.waitOnce.Do(func() { go func() { j.wait <- j.cmd.Wait() }() })
}
func (j *DesktopJournal) abort() {
	if j.closed {
		return
	}
	j.closed = true
	j.in.Close()
	j.out.Close()
	j.cmd.Process.Kill()
	j.startWait()
	select {
	case <-j.wait:
	case <-time.After(3 * time.Second):
	}
}
func (j *DesktopJournal) rpc(v any) (journalReply, error) {
	var r journalReply
	if j.closed {
		return r, ErrTrust
	}
	done := make(chan error, 1)
	go func() {
		b, e := json.Marshal(v)
		if e == nil && len(b)+1 > 16384 {
			e = ErrTrust
		}
		if e == nil {
			_, e = j.in.Write(append(b, '\n'))
		}
		if e == nil {
			e = j.reader.Read(&r)
		}
		done <- e
	}()
	select {
	case e := <-done:
		return r, e
	case <-time.After(5 * time.Second):
		j.abort()
		<-done
		return journalReply{}, errors.Join(ErrTrust, ErrNativeHelperTimeout)
	}
}
func (j *DesktopJournal) finish() error {
	j.in.Close()
	j.startWait()
	select {
	case e := <-j.wait:
		j.closed = true
		j.out.Close()
		return e
	case <-time.After(3 * time.Second):
		j.abort()
		return ErrTrust
	}
}
func (j *DesktopJournal) Load() ([]byte, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	r, e := j.rpc(map[string]string{"Op": "load"})
	if e == nil && r.Status == "not_installed" && j.created && r.Data == "" && r.Reason == "" {
		return nil, ErrNotInstalled
	}
	b, be := base64.StdEncoding.DecodeString(r.Data)
	if e != nil || r.Status != "loaded" || r.Reason != "" || be != nil || base64.StdEncoding.EncodeToString(b) != r.Data || len(b) > 8191 || len(b) == 0 {
		j.abort()
		if errors.Is(e, ErrNativeHelperTimeout) {
			return nil, e
		}
		return nil, ErrTrust
	}
	return b, nil
}
func (j *DesktopJournal) Commit(expected, next []byte) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.installation {
		if _, e := decodeInstallation(next); e != nil {
			j.abort()
			return ErrTrust
		}
	} else if len(next) == 0 || len(next) > 8191 {
		j.abort()
		return ErrTrust
	}
	digest := ""
	if expected != nil {
		h := sha256.Sum256(expected)
		digest = hex.EncodeToString(h[:])
	}
	r, e := j.rpc(map[string]string{"Op": "commit", "Expected": digest, "Data": base64.StdEncoding.EncodeToString(next)})
	if e != nil || r.Status != "committed" || r.Data != "" || r.Reason != "" {
		j.abort()
		if errors.Is(e, ErrNativeHelperTimeout) {
			return e
		}
		return ErrTrust
	}
	j.created = false
	return nil
}
func (j *DesktopJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	r, e := j.rpc(map[string]string{"Op": "stop"})
	if e != nil || r.Status != "closed" || r.Data != "" || r.Reason != "" {
		j.abort()
		if errors.Is(e, ErrNativeHelperTimeout) {
			return e
		}
		return ErrTrust
	}
	return j.finish()
}
func DeleteTestDesktopJournal(executable, scope string) error {
	j, e := startDesktopJournal(executable, scope, "cleanup", true, true)
	if e != nil {
		return e
	}
	return j.finish()
}

// InspectDesktopJournalState starts the same fixed helper in a one-shot,
// read-only mode. It returns no protected record data.
func InspectDesktopJournalState(executable, scope string, test bool) (JournalState, error) {
	b, err := hex.DecodeString(scope)
	if !filepath.IsAbs(executable) || err != nil || len(b) != 16 || hex.EncodeToString(b) != scope {
		return "", ErrTrust
	}
	cmd := exec.Command(executable)
	in, err := cmd.StdinPipe()
	if err != nil {
		return "", ErrTrust
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return "", ErrTrust
	}
	cmd.Stderr = io.Discard
	if cmd.Start() != nil {
		in.Close()
		out.Close()
		return "", ErrTrust
	}
	type result struct {
		state JournalState
		err   error
	}
	done := make(chan result, 1)
	go func() {
		if err := json.NewEncoder(in).Encode(map[string]any{"Scope": scope, "Mode": "status", "Test": test}); err != nil {
			_ = in.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			done <- result{err: err}
			return
		}
		_ = in.Close()
		var reply journalReply
		readErr := privatepipe.NewBoundedReader(out, 16384).Read(&reply)
		waitErr := cmd.Wait()
		if readErr != nil || waitErr != nil {
			done <- result{err: ErrTrust}
			return
		}
		if reply.Status != "state" || reply.Reason != "" {
			done <- result{err: ErrTrust}
			return
		}
		state := JournalState(reply.Data)
		switch state {
		case JournalAbsent, JournalPending, JournalCommitted, JournalUnusable:
			done <- result{state: state}
		default:
			done <- result{err: ErrTrust}
		}
	}()
	select {
	case got := <-done:
		if got.err != nil {
			return "", ErrTrust
		}
		return got.state, nil
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return "", ErrTrust
	}
}
func (j *DesktopJournal) FaultTest(mode string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.test {
		return ErrTrust
	}
	r, e := j.rpc(map[string]string{"Op": "fault", "Fault": mode})
	if e != nil || r.Status != "faulted" || r.Data != "" || r.Reason != "" {
		j.abort()
		return ErrTrust
	}
	return nil
}

// Never include unknown native reply text in public errors.
var ErrNativeAuthentication = errors.New("native_authentication_required")

func journalFailure(reason string) error {
	switch reason {
	case "interactive_logon_required":
		return ErrDesktopLogon
	case "native_authentication_required":
		return errors.Join(ErrTrust, ErrNativeAuthentication)
	}
	return ErrTrust
}

// Timeouts are uncertain outcomes, not a permission to retry or release leases.
var ErrNativeHelperTimeout = errors.New("native_helper_timeout")

func NativeFailureReason(err error) string {
	if errors.Is(err, ErrNativeHelperTimeout) {
		return "native_helper_timeout"
	}
	if errors.Is(err, ErrNativeAuthentication) {
		return "native_authentication_required"
	}
	if errors.Is(err, ErrDesktopLogon) {
		return "interactive_logon_required"
	}
	return ""
}
