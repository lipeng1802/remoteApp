package main

import (
	"io"
	"net"
	"time"
)

// Fixed stages only; never expose the token, payload or raw network errors.
func exchangeProbe(conn net.Conn, token string) (bool, string) {
	if conn.SetDeadline(time.Now().Add(2*time.Second)) != nil {
		return false, "deadline_failed"
	}
	if n, err := io.WriteString(conn, token); err != nil || n != len(token) {
		return false, "write_failed"
	}
	b := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, b); err != nil {
		return false, "read_failed"
	}
	if string(b) != payload {
		return false, "payload_mismatch"
	}
	return true, "payload_verified"
}
