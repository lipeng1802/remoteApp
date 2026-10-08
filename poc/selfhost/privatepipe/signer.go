package privatepipe

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

var deviceLogonError = errors.New("interactive_logon_required")

type Signer struct {
	mu                 sync.Mutex
	cmd                *exec.Cmd
	in                 io.WriteCloser
	out                io.ReadCloser
	reader             *Reader
	pub                ed25519.PublicKey
	closed             bool
	created            bool
	wait               chan struct{}
	waitErr            error
	aborted, closedAck bool
	waitOnce           sync.Once
}
type VaultReply struct {
	Status    string `json:"status"`
	PublicKey string `json:"public_key,omitempty"`
	Signature string `json:"signature,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// Executable must come from the trusted installation/bootstrap, never peer
// input. No shell, arguments, inherited network listener or stderr forwarding.
func NewSigner(executable, scope, expected string, create bool) (*Signer, error) {
	if !filepath.IsAbs(executable) {
		return nil, ErrProtocol
	}
	c := exec.Command(executable)
	in, e := c.StdinPipe()
	if e != nil {
		return nil, ErrProtocol
	}
	out, e := c.StdoutPipe()
	if e != nil {
		in.Close()
		return nil, ErrProtocol
	}
	c.Stderr = io.Discard
	if c.Start() != nil {
		in.Close()
		out.Close()
		return nil, ErrProtocol
	}
	s := &Signer{cmd: c, in: in, out: out, created: create, reader: NewReader(out), wait: make(chan struct{})}
	r, e := s.rpc(map[string]any{"Scope": scope, "Create": create, "Expected": expected})
	pub, pe := hex.DecodeString(r.PublicKey)
	if e != nil || r.Status != "ready" || pe != nil || len(pub) != 32 || hex.EncodeToString(pub) != r.PublicKey || (expected != "" && expected != r.PublicKey) {
		s.abort()
		<-s.wait // a failed startup must not leave an unowned OS signer process
		if r.Reason == "interactive_logon_required" {
			return nil, deviceLogonError
		}
		return nil, ErrProtocol
	}
	s.pub = pub
	return s, nil
}
func (s *Signer) abort() {
	if !s.closed {
		s.closed = true
		s.aborted = true
		s.in.Close()
		s.out.Close()
		s.cmd.Process.Kill()
		s.startWait()
		select {
		case <-s.wait:
		case <-time.After(3 * time.Second):
		}
	}
}
func (s *Signer) startWait() {
	s.waitOnce.Do(func() { go func() { s.waitErr = s.cmd.Wait(); close(s.wait) }() })
}
func (s *Signer) rpc(v any) (VaultReply, error) {
	var r VaultReply
	done := make(chan error, 1)
	go func() {
		e := json.NewEncoder(s.in).Encode(v)
		if e == nil {
			e = s.reader.Read(&r)
		}
		done <- e
	}()
	select {
	case e := <-done:
		return r, e
	case <-time.After(3 * time.Second):
		s.abort()
		<-done
		return VaultReply{}, ErrProtocol
	}
}
func (s *Signer) PublicKey() ed25519.PublicKey { return append(ed25519.PublicKey(nil), s.pub...) }
func (s *Signer) Sign(raw []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || len(raw) > 4096 {
		return nil, ErrProtocol
	}
	r, e := s.rpc(map[string]string{"Op": "sign", "Message": base64.StdEncoding.EncodeToString(raw)})
	sig, se := hex.DecodeString(r.Signature)
	if e != nil || se != nil || r.Status != "signed" || len(sig) != 64 || !ed25519.Verify(s.pub, raw, sig) {
		s.abort()
		return nil, ErrProtocol
	}
	return sig, nil
}
func (s *Signer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	r, e := s.rpc(map[string]string{"Op": "stop"})
	if e != nil || r.Status != "closed" {
		s.abort()
		return
	}
	s.closedAck = true
	s.in.Close()
	s.startWait()
	select {
	case <-s.wait:
		s.closed = true
		s.out.Close()
	case <-time.After(3 * time.Second):
		s.abort()
	}
}

// CloseJoined is the installation-owner barrier. Legacy Close's bounded
// emergency handling is not a proof of process exit. Join the SAME Wait even
// after a previous timeout/abort; failed or forced exit is never success.
func (s *Signer) CloseJoined() error {
	s.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startWait()
	<-s.wait
	s.out.Close()
	if s.aborted || !s.closedAck || s.waitErr != nil {
		return ErrProtocol
	}
	return nil
}

// Test teardown only; helper also denies delete on read-only reopen. This
// namespace is independent of installed production credentials.
func (s *Signer) DeleteCreatedIdentity() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !s.created {
		return ErrProtocol
	}
	r, e := s.rpc(map[string]string{"Op": "delete"})
	if e != nil || r.Status != "deleted" {
		s.abort()
		return ErrProtocol
	}
	s.in.Close()
	s.startWait()
	select {
	case <-s.wait:
		s.closed = true
		s.out.Close()
		if s.waitErr != nil {
			return ErrProtocol
		}
	case <-time.After(3 * time.Second):
		s.abort()
		return ErrProtocol
	}
	return nil
}
