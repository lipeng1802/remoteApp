// Package supervisedcontrol routes SAME-USER local business IPC. Listener
// authentication belongs to the OS protected pipe, not to request fields.
package supervisedcontrol

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/privatepipe"
)

type Backend interface {
	Forward(context.Context, installeddevice.Request) (json.RawMessage, error)
	Count(context.Context) (clientruntime.ProductCount, error)
}

// Serve never owns or releases the product. Cancellation closes listener and
// active connections, then joins all handlers before the caller releases it.
// One bounded request per connection; private replies must never be logged.
func Serve(ctx context.Context, ln net.Listener, b Backend) {
	life, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	monitor := make(chan struct{})
	go func() { defer close(monitor); <-life.Done(); ln.Close() }()
	defer func() { cancel(); ln.Close(); wg.Wait(); <-monitor }()
	slots := make(chan struct{}, 4)
	for {
		conn, e := ln.Accept()
		if e != nil {
			return
		}
		select {
		case slots <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			defer conn.Close()
			closed := make(chan struct{})
			stop := context.AfterFunc(life, func() { conn.Close(); close(closed) })
			defer func() {
				if !stop() {
					<-closed
				}
			}()
			conn.SetDeadline(time.Now().Add(25 * time.Second))
			var request installeddevice.Request
			if privatepipe.NewReader(conn).Read(&request) != nil {
				return
			}
			output := json.NewEncoder(conn)
			command, done := context.WithTimeout(life, 20*time.Second)
			defer done()
			if request == (installeddevice.Request{Op: "product_count"}) {
				count, e := b.Count(command)
				if e != nil {
					output.Encode(map[string]string{"status": "product_unavailable"})
				} else {
					output.Encode(count)
				}
				return
			}
			if !installeddevice.ValidSupervisedCommand(request) {
				output.Encode(map[string]string{"status": "request_rejected"})
				return
			}
			raw, e := b.Forward(command, request)
			if e != nil {
				output.Encode(map[string]string{"status": "request_rejected"})
				return
			}
			output.Encode(raw)
		}()
	}
}
