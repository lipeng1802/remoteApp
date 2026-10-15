package installeddevice

import (
	"context"
	"sync"

	"remoteapp.local/selfhost-poc/invite"
)

// enrollmentSession is joined before the authorization watch, proof worker,
// node SDK and signer. That ordering prevents a product worker from borrowing
// network or identity resources after its terminal authorization has ended.
type enrollmentSession struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
	err    error
}

func (s *enrollmentSession) Done() <-chan struct{} { return s.done }
func (s *enrollmentSession) Err() error {
	select {
	case <-s.done:
		return s.err
	default:
		return nil
	}
}
func (s *enrollmentSession) Stop() {
	s.once.Do(s.cancel)
	<-s.done
}

func (e *enrollment) startSession(ctx context.Context, work SessionWork) error {
	if ctx == nil || work == nil || e.session != nil || e.watch == nil || e.watch.Faulted() || e.peers == nil {
		return invite.ErrDenied
	}
	select {
	case <-e.watch.Done():
		return invite.ErrDenied
	default:
	}
	life, cancel := context.WithCancel(ctx)
	s := &enrollmentSession{cancel: cancel, done: make(chan struct{})}
	e.session = s
	go func() {
		defer close(s.done)
		s.err = work(life, *e.peers)
	}()
	return nil
}
