// Package processowner owns dedicated, trusted local JSON-pipe children. It
// never selects executables from network input or treats a timeout as a join.
package processowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"

	"remoteapp.local/selfhost-poc/privatepipe"
)

var ErrProcess = errors.New("owned_process_unavailable")

// Spec is supplied by trusted local coordination, not peer JSON. Hash pinning
// is integrity, not publisher authentication or same-user TOCTOU protection.
// Args must never contain secrets: send them only through Exchange's pipe.
type Spec struct {
	Executable, SHA256 string
	Args               []string
}
type reply struct {
	value json.RawMessage
	err   error
}
type Child struct {
	cmd                        *exec.Cmd
	in, out                    *os.File
	mu                         sync.Mutex // one request / stop writer; keeps late replies from reuse
	fault                      atomic.Bool
	stopping                   chan struct{}
	replies                    chan reply
	readDone, exited, done     chan struct{}
	waitErr, readErr, closeErr error
	stopOnce                   sync.Once
}

func Start(s Spec) (*Child, error) {
	if !filepath.IsAbs(s.Executable) || filepath.Clean(s.Executable) != s.Executable {
		return nil, ErrProcess
	}
	real, e := filepath.EvalSymlinks(s.Executable)
	if e != nil || real != s.Executable {
		return nil, ErrProcess
	}
	st, e := os.Lstat(s.Executable)
	if e != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > 100<<20 {
		return nil, ErrProcess
	}
	pin, e := hex.DecodeString(s.SHA256)
	if e != nil || len(pin) != 32 || hex.EncodeToString(pin) != s.SHA256 {
		return nil, ErrProcess
	}
	b, e := os.ReadFile(s.Executable)
	if e != nil {
		return nil, ErrProcess
	}
	digest := sha256.Sum256(b)
	if hex.EncodeToString(digest[:]) != s.SHA256 {
		return nil, ErrProcess
	}
	inR, inW, e := os.Pipe()
	if e != nil {
		return nil, ErrProcess
	}
	outR, outW, e := os.Pipe()
	if e != nil {
		inR.Close()
		inW.Close()
		return nil, ErrProcess
	}
	c := &Child{cmd: exec.Command(s.Executable, s.Args...), in: inW, out: outR, replies: make(chan reply, 16), readDone: make(chan struct{}), exited: make(chan struct{}), done: make(chan struct{}), stopping: make(chan struct{})}
	c.cmd.Stdin, c.cmd.Stdout, c.cmd.Stderr = inR, outW, io.Discard
	if c.cmd.Start() != nil {
		inR.Close()
		inW.Close()
		outR.Close()
		outW.Close()
		return nil, ErrProcess
	}
	inR.Close()
	outW.Close()
	go func() { c.waitErr = c.cmd.Wait(); close(c.exited) }()
	go c.read()
	return c, nil
}
func (c *Child) read() {
	defer close(c.readDone)
	defer close(c.replies)
	defer c.out.Close()
	r := privatepipe.NewReader(c.out)
	for {
		var v json.RawMessage
		e := r.Read(&v)
		if e != nil {
			if e != io.EOF {
				c.readErr = ErrProcess
			}
			return
		}
		select {
		case c.replies <- reply{value: v}:
		default:
			c.readErr = ErrProcess
			return
		}
	}
}

// Exited signals only direct child exit, NOT pipe workers/descendants joined.
func (c *Child) Exited() <-chan struct{} { return c.exited }
func (c *Child) Done() <-chan struct{}   { return c.done }

// Exchange's response remains private to the trusted caller. Never log it:
// product readiness may contain a temporary test HMAC. A timed-out exchange
// latches fault and closes input; its late response cannot authorize reuse.
func (c *Child) Exchange(ctx context.Context, command any) (json.RawMessage, error) {
	if ctx == nil {
		return nil, ErrProcess
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	result := make(chan reply, 1)
	go func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		select {
		case <-c.stopping:
			result <- reply{err: ErrProcess}
			return
		default:
		}
		if ctx.Err() != nil || c.fault.Load() {
			result <- reply{err: ErrProcess}
			return
		}
		if command != nil {
			b, e := json.Marshal(command)
			if e != nil || len(b)+1 > 8192 {
				c.fault.Store(true)
				result <- reply{err: ErrProcess}
				return
			}
			if _, e = c.in.Write(append(b, '\n')); e != nil {
				c.fault.Store(true)
				result <- reply{err: ErrProcess}
				return
			}
		}
		v, ok := <-c.replies
		if !ok {
			c.fault.Store(true)
			result <- reply{err: ErrProcess}
			return
		}
		result <- v
	}()
	select {
	case r := <-result:
		return r.value, r.err
	case <-ctx.Done():
		// Close interrupts a blocked writer; queued exchanges observe EOF/fault.
		c.fault.Store(true)
		c.in.Close()
		return nil, ctx.Err()
	}
}

// Stop initiates graceful stop once; retry only waits for that same join.
// It never kills a child or releases ownership on a deadline. Even a "closed"
// reply is insufficient: zero exit + stdout EOF + reader join are required.
func (c *Child) Stop(ctx context.Context) error {
	if ctx == nil {
		return ErrProcess
	}
	c.stopOnce.Do(func() { close(c.stopping); go c.stop() })
	select {
	case <-c.done:
		return c.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (c *Child) stop() {
	c.mu.Lock()
	if !c.fault.Load() {
		if _, e := c.in.Write([]byte("{\"Op\":\"stop\"}\n")); e != nil {
			c.fault.Store(true)
		}
	}
	c.in.Close()
	closed := false
	for r := range c.replies {
		var v struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(r.value, &v) != nil || v.Status != "closed" || closed {
			c.fault.Store(true)
		} else {
			closed = true
		}
	}
	<-c.readDone
	<-c.exited
	if c.fault.Load() || !closed || c.waitErr != nil || c.readErr != nil {
		c.closeErr = ErrProcess
	}
	c.mu.Unlock()
	close(c.done)
}
