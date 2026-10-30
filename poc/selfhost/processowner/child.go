// Package processowner owns dedicated, trusted local JSON-pipe children. It
// never selects executables from network input or treats a timeout as a join.
package processowner

import (
	"bufio"
	"bytes"
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
	"remoteapp.local/selfhost-poc/startuptrace"
)

var ErrProcess = errors.New("owned_process_unavailable")

// Spec is supplied by trusted local coordination, not peer JSON. Hash pinning
// is integrity, not publisher authentication or same-user TOCTOU protection.
// Args must never contain secrets: send them only through Exchange's pipe.
type Spec struct {
	Executable, SHA256 string
	Args               []string
	// CaptureProductStage is trusted TEST wiring; default stderr remains discarded.
	CaptureProductStage bool
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
	finishMode                 atomic.Uint32
	terminalFrames             int
	failedTerminal             bool
	onJoined                   func(error)
	diagnosticDone             chan struct{}
	diagnosticErr              error
	stage                      atomic.Value
}

func Start(s Spec) (*Child, error) {
	return startOwnedProcess(s, nil)
}

// StartObserved is trusted local wiring. The callback runs only after the
// normal stop/EOF/actual-exit checks, before Done; its error is not a work error.
func StartObserved(s Spec, joined func(error)) (*Child, error) { return startOwnedProcess(s, joined) }
func startOwnedProcess(s Spec, joined func(error)) (*Child, error) {
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
	c.onJoined = joined
	c.diagnosticDone = make(chan struct{})
	var diagnosticR, diagnosticW *os.File
	if s.CaptureProductStage {
		diagnosticR, diagnosticW, e = os.Pipe()
		if e != nil {
			inR.Close()
			inW.Close()
			outR.Close()
			outW.Close()
			return nil, ErrProcess
		}
	} else {
		close(c.diagnosticDone)
	}
	c.cmd.Stdin, c.cmd.Stdout, c.cmd.Stderr = inR, outW, io.Discard
	if diagnosticW != nil {
		c.cmd.Stderr = diagnosticW
	}
	if c.cmd.Start() != nil {
		if diagnosticW != nil {
			diagnosticW.Close()
			diagnosticR.Close()
		}
		inR.Close()
		inW.Close()
		outR.Close()
		outW.Close()
		return nil, ErrProcess
	}
	inR.Close()
	outW.Close()
	if diagnosticW != nil {
		diagnosticW.Close()
		go c.drainStage(diagnosticR)
	}
	go func() { c.waitErr = c.cmd.Wait(); close(c.exited) }()
	go c.read()
	return c, nil
}

// drainStage bounds storage and discards all non-enum text, including raw stderr.
func (c *Child) drainStage(f *os.File) {
	defer close(c.diagnosticDone)
	defer f.Close()
	r := bufio.NewReader(f)
	line := make([]byte, 0, 128)
	overflow := false
	for {
		b, e := r.ReadByte()
		if e != nil {
			if e != io.EOF {
				c.diagnosticErr = ErrProcess
			}
			return
		}
		if b == '\n' {
			const prefix = "PRODUCT_STAGE "
			if !overflow && bytes.HasPrefix(line, []byte(prefix)) {
				s := string(line[len(prefix):])
				if startuptrace.Valid(s) {
					c.stage.Store(s)
				}
			}
			line = line[:0]
			overflow = false
		} else if len(line) < 128 && !overflow {
			line = append(line, b)
		} else {
			overflow = true
		}
	}
}

// ProductStage is cached telemetry and never sends a new product command.
func (c *Child) ProductStage() string {
	if s, ok := c.stage.Load().(string); ok {
		return s
	}
	return "unreported"
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

// JoinedFault needs the trusted product's dedicated nested-cleanup receipt,
// actual exit 1, stdout EOF and reader/stop join. Generic "failed" does NOT
// attest nested node/signer cleanup. The producer must implement that barrier;
// old products without it remain rejected. Stop's normal contract is unchanged.
func (c *Child) JoinedFault() bool {
	select {
	case <-c.done:
		return c.cmd.ProcessState != nil && c.cmd.ProcessState.ExitCode() == 1 && c.readErr == nil && c.diagnosticErr == nil && c.terminalFrames == 1 && c.failedTerminal
	default:
		return false
	}
}

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
		// A terminal failure can arrive while telemetry/business is in flight.
		// Preserve its exact receipt for the later joined-fault check instead of
		// losing it as an invalid count response. No more business can use it.
		var terminal struct {
			Status string `json:"status"`
		}
		d := json.NewDecoder(bytes.NewReader(v.value))
		d.DisallowUnknownFields()
		if d.Decode(&terminal) == nil && d.Decode(new(any)) == io.EOF && terminal.Status == "fault_joined" {
			c.terminalFrames++
			c.failedTerminal = true
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
	return c.finish(ctx, []byte("{\"Op\":\"stop\"}\n"), 1)
}

// FinishMaintenance is a terminal trusted-pipe command, not an Exchange of a
// reusable closed reply. It requires exactly one closed frame, EOF, reader join
// and actual zero exit. A timeout retains this same finish operation; no kill.
func (c *Child) FinishMaintenance(ctx context.Context) error {
	return c.finish(ctx, []byte("{\"Op\":\"maintenance_finish\"}\n"), 2)
}

func (c *Child) finish(ctx context.Context, command []byte, mode uint32) error {
	if ctx == nil {
		return ErrProcess
	}
	c.stopOnce.Do(func() { c.finishMode.Store(mode); close(c.stopping); go c.stop(command) })
	if mode == 2 && c.finishMode.Load() != 2 {
		return ErrProcess
	}
	select {
	case <-c.done:
		return c.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (c *Child) stop(command []byte) {
	c.mu.Lock()
	if !c.fault.Load() {
		if _, e := c.in.Write(command); e != nil {
			c.fault.Store(true)
		}
	}
	c.in.Close()
	closed := false
	for r := range c.replies {
		c.terminalFrames++
		var v struct {
			Status string `json:"status"`
		}
		d := json.NewDecoder(bytes.NewReader(r.value))
		d.DisallowUnknownFields()
		valid := d.Decode(&v) == nil && d.Decode(new(any)) == io.EOF
		if valid && v.Status == "fault_joined" {
			c.failedTerminal = true
		}
		if !valid || v.Status != "closed" || closed {
			c.fault.Store(true)
		} else {
			closed = true
		}
	}
	<-c.readDone
	<-c.exited
	<-c.diagnosticDone
	if c.fault.Load() || !closed || c.waitErr != nil || c.readErr != nil || c.diagnosticErr != nil {
		c.closeErr = ErrProcess
	}
	c.mu.Unlock()
	if c.onJoined != nil {
		c.onJoined(c.closeErr)
	}
	close(c.done)
}
