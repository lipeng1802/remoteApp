package installeddevice

import (
	"context"
	"net"
	"remoteapp.local/selfhost-poc/cleanupwitness"
	"sync"
)

type sessionListener struct {
	net.Listener
	once sync.Once
	err  error
	w    *cleanupwitness.Witness
}

func (l *sessionListener) Close() error {
	l.once.Do(func() { l.err = l.Listener.Close(); l.w.Complete(cleanupwitness.TransportListener, l.err) })
	return l.err
}
func trackedSessionListener(ctx context.Context, open func() (net.Listener, error)) (net.Listener, error) {
	w := cleanupwitness.From(ctx)
	w.Begin(cleanupwitness.TransportListener)
	l, e := open()
	if e != nil {
		return nil, e
	}
	return &sessionListener{Listener: l, w: w}, nil
}
