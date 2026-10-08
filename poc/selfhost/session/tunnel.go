package session

import (
	"context"
	"io"
	"net"
	"time"
)

// Tunnel admits an encrypted product stream only after the same bounded mutual
// device proof and current grant check as the fixed-payload session. open is a
// private caller capability, never a destination chosen by the network peer.
func Tunnel(ctx context.Context, conn net.Conn, c Config, check Check, server bool, open func(context.Context) (net.Conn, error)) error {
	defer conn.Close()
	if !c.valid(conn, server) || !authorized(ctx, c, check) || open == nil {
		return ErrDenied
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer closeOnCancel(ctx, conn)()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if server {
		var h hello
		if read(conn, &h) != nil || !validNonce(h.Nonce) {
			return ErrDenied
		}
		n, e := nonce()
		if e != nil {
			return ErrDenied
		}
		t := transcript{1, c.Grant, n, h.Nonce, c.Binding.TargetIP.String(), c.Binding.ControllerIP.String()}
		sig, e := c.signature("product-tls-target", t)
		if e != nil || write(conn, welcome{t, sig}) != nil {
			return ErrDenied
		}
		var p proof
		if read(conn, &p) != nil || !verifies(c.Binding.Controller, "product-tls-controller", t, p.Signature) {
			return ErrDenied
		}
	} else {
		n, e := nonce()
		if e != nil || write(conn, hello{n}) != nil {
			return ErrDenied
		}
		var w welcome
		if read(conn, &w) != nil {
			return ErrDenied
		}
		t := w.Transcript
		if t.Version != 1 || t.Grant != c.Grant || t.ClientNonce != n || !validNonce(t.ServerNonce) || t.TargetIP != c.Binding.TargetIP.String() || t.ControllerIP != c.Binding.ControllerIP.String() || !verifies(c.Binding.Target, "product-tls-target", t, w.Signature) {
			return ErrDenied
		}
		sig, e := c.signature("product-tls-controller", t)
		if e != nil || write(conn, proof{sig}) != nil {
			return ErrDenied
		}
	}
	if !authorized(ctx, c, check) {
		return ErrDenied
	}
	local, e := open(ctx)
	if e != nil || local == nil {
		return ErrDenied
	}
	defer local.Close()
	defer closeOnCancel(ctx, local)()
	conn.SetDeadline(time.Time{})
	watchJoined := make(chan struct{})
	defer func() { cancel(); <-watchJoined }()
	go func() {
		defer close(watchJoined)
		t := time.NewTicker(checkInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if !authorized(ctx, c, check) {
					cancel()
					return
				}
			}
		}
	}()
	results := make(chan error, 2)
	copyStream := func(dst, src net.Conn) { _, e := io.CopyBuffer(dst, src, make([]byte, 32*1024)); results <- e }
	go copyStream(local, conn)
	go copyStream(conn, local)
	<-results
	cancel()
	conn.Close()
	local.Close()
	<-results // both bounded copy workers must leave before normal helper shutdown
	return ctx.Err()
}
