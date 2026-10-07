//go:build linux || darwin

package broker

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

func send(c *net.UnixConn, v any) error {
	b, e := json.Marshal(v)
	if e != nil || len(b) > 8191 {
		return ErrBroker
	}
	b = append(b, '\n')
	_, e = c.Write(b)
	if e != nil {
		return ErrBroker
	}
	return nil
}
func receive(c *net.UnixConn, v any) error {
	r := privatepipe.NewReader(c)
	var extra struct{}
	if r.Read(v) != nil || r.Read(&extra) != io.EOF {
		return ErrBroker
	}
	return nil
}
func deadline(ctx context.Context) time.Time {
	return operationDeadline(ctx, 5*time.Second)
}
func operationDeadline(ctx context.Context, budget time.Duration) time.Time {
	d := time.Now().Add(budget)
	if cd, ok := ctx.Deadline(); ok && cd.Before(d) {
		d = cd
	}
	return d
}

// Serve authenticates kernel peer UID BEFORE parsing. It accepts one strict
// request per connection, never commands/paths/secrets provided as authority.
// Stop closes listeners/connections, cancels worker contexts and joins workers.
// Blocking storage itself is not claimed to be force-cancellable.
func Serve(ctx context.Context, cfg Config, ready func()) error {
	if ctx.Err() != nil || cfg.Executor == nil || !filepath.IsAbs(cfg.Socket) || filepath.Clean(cfg.Socket) != cfg.Socket {
		return ErrBroker
	}
	dir := filepath.Dir(cfg.Socket)
	real, e := filepath.EvalSymlinks(dir)
	if e != nil || real != dir || !safeDirectory(dir, cfg.GroupID) {
		return ErrBroker
	}
	if _, e = os.Lstat(cfg.Socket); !os.IsNotExist(e) {
		return ErrBroker
	}
	l, e := net.ListenUnix("unix", &net.UnixAddr{Name: cfg.Socket, Net: "unix"})
	if e != nil {
		return ErrBroker
	}
	defer l.Close()
	if os.Chown(cfg.Socket, os.Geteuid(), int(cfg.GroupID)) != nil || os.Chmod(cfg.Socket, 0660) != nil {
		return ErrBroker
	}
	run, cancel := context.WithCancel(ctx)
	defer cancel()
	var mu sync.Mutex
	connections := map[*net.UnixConn]bool{}
	var workers sync.WaitGroup
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		<-run.Done()
		l.Close()
		mu.Lock()
		defer mu.Unlock()
		for conn := range connections {
			conn.Close()
		}
	}()
	defer func() { cancel(); <-watchDone; workers.Wait() }()
	if ready != nil {
		ready()
	}
	workSlots := make(chan struct{}, 4)
	for {
		conn, e := l.AcceptUnix()
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return ErrBroker
		}
		uid, e := peerUID(conn)
		if e != nil || uid != cfg.PeerUID {
			conn.Close()
			continue
		}
		mu.Lock()
		if run.Err() != nil || len(connections) >= 8 {
			mu.Unlock()
			conn.Close()
			continue
		}
		connections[conn] = true
		mu.Unlock()
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { conn.Close(); mu.Lock(); delete(connections, conn); mu.Unlock() }()
			conn.SetDeadline(deadline(run))
			var r Request
			if receive(conn, &r) != nil {
				return
			}
			select {
			case workSlots <- struct{}{}:
				defer func() { <-workSlots }()
			default:
				send(conn, Reply{Status: 503})
				return
			}
			budget := 4 * time.Second
			if r.Operation == "network" {
				budget = 14 * time.Second
			}
			conn.SetDeadline(operationDeadline(run, budget+time.Second))
			work, stop := context.WithTimeout(run, budget)
			defer stop()
			if work.Err() != nil {
				return
			}
			send(conn, cfg.Executor.Execute(work, r))
		}()
	}
}

func (c Client) Call(ctx context.Context, r Request) (Reply, error) {
	var result Reply
	if ctx.Err() != nil || !filepath.IsAbs(c.Socket) || filepath.Clean(c.Socket) != c.Socket {
		return result, ErrBroker
	}
	base, e := (&net.Dialer{}).DialContext(ctx, "unix", c.Socket)
	if e != nil {
		return result, ErrBroker
	}
	conn, ok := base.(*net.UnixConn)
	if !ok {
		base.Close()
		return result, ErrBroker
	}
	defer conn.Close()
	uid, e := peerUID(conn)
	if e != nil || uid != c.ServerUID {
		return result, ErrBroker
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	conn.SetDeadline(deadline(ctx))
	if r.Operation == "network" {
		conn.SetDeadline(operationDeadline(ctx, 15*time.Second))
	}
	if send(conn, r) != nil || conn.CloseWrite() != nil || receive(conn, &result) != nil {
		return Reply{}, ErrBroker
	}
	if result.Status != 200 && result.Status != 403 && result.Status != 503 {
		return Reply{}, ErrBroker
	}
	if result.Status != 200 && (result.Device != nil || result.Network != nil || result.Issuer != "") {
		return Reply{}, ErrBroker
	}
	if result.Status == 200 {
		pub, e := hex.DecodeString(result.Issuer)
		if e != nil || len(pub) != 32 || hex.EncodeToString(pub) != result.Issuer {
			return Reply{}, ErrBroker
		}
	}
	if result.Status == 200 && ((r.Operation == "device" && (result.Device == nil || result.Network != nil)) || (r.Operation == "network" && (result.Network == nil || result.Device != nil)) || (r.Operation == "identity" && (result.Device != nil || result.Network != nil)) || (r.Operation != "device" && r.Operation != "network" && r.Operation != "identity")) {
		return Reply{}, ErrBroker
	}
	return result, nil
}
