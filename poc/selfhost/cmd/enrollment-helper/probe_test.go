package main

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"
)

type probeConn struct {
	*bytes.Reader
	deadlineErr, writeErr error
	shortWrite            bool
}

func (c *probeConn) Write(b []byte) (int, error) {
	if c.shortWrite {
		return len(b) - 1, nil
	}
	return len(b), c.writeErr
}
func (*probeConn) Close() error                     { return nil }
func (*probeConn) LocalAddr() net.Addr              { return nil }
func (*probeConn) RemoteAddr() net.Addr             { return nil }
func (c *probeConn) SetDeadline(time.Time) error    { return c.deadlineErr }
func (*probeConn) SetReadDeadline(time.Time) error  { return nil }
func (*probeConn) SetWriteDeadline(time.Time) error { return nil }

func TestExchangeProbeStagesRequireExactPayload(t *testing.T) {
	broken := errors.New("private detail must not become a stage")
	for _, tc := range []struct {
		name, data, stage string
		deadline, write   error
		short, passed     bool
	}{
		{name: "verified", data: payload, stage: "payload_verified", passed: true},
		{name: "deadline", data: payload, stage: "deadline_failed", deadline: broken},
		{name: "write", data: payload, stage: "write_failed", write: broken},
		{name: "short_write", data: payload, stage: "write_failed", short: true},
		{name: "read", data: "", stage: "read_failed"},
		{name: "mismatch", data: string(bytes.Repeat([]byte{'x'}, len(payload))), stage: "payload_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := &probeConn{Reader: bytes.NewReader([]byte(tc.data)), deadlineErr: tc.deadline, writeErr: tc.write, shortWrite: tc.short}
			ok, stage := exchangeProbe(conn, "private test token")
			if ok != tc.passed || stage != tc.stage {
				t.Fatalf("unexpected result: %v %s", ok, stage)
			}
		})
	}
}
