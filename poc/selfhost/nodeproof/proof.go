// Package nodeproof authenticates a fresh app challenge over a real tsnet peer.
package nodeproof

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"tailscale.com/tsnet"
)

const Port = "47477"

type challenge struct {
	Ticket, Grant, Device, Node, IP, Nonce string
	Issued                                 int64
}
type response struct{ Signature string }

func transcript(c challenge) []byte {
	b, _ := json.Marshal(c)
	return append([]byte("remoteapp-node-possession-v1\n"), b...)
}
func identity(pub ed25519.PublicKey) string { h := sha256.Sum256(pub); return hex.EncodeToString(h[:]) }
func frame(w io.Writer, v any) error {
	b, e := json.Marshal(v)
	if e != nil || len(b) > 2048 {
		return invite.ErrDenied
	}
	var n [2]byte
	binary.BigEndian.PutUint16(n[:], uint16(len(b)))
	if _, e = w.Write(n[:]); e != nil {
		return e
	}
	_, e = w.Write(b)
	return e
}
func read(r io.Reader, v any) error {
	var n [2]byte
	if _, e := io.ReadFull(r, n[:]); e != nil {
		return e
	}
	size := int(binary.BigEndian.Uint16(n[:]))
	if size == 0 || size > 2048 {
		return invite.ErrDenied
	}
	b := make([]byte, size)
	if _, e := io.ReadFull(r, b); e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}

// Verify requires the peer to exist in the authenticated tsnet map, preventing
// Dial's system-network fallback, and WhoIs to match the expected node key/IP.
func Verify(ctx context.Context, s *tsnet.Server, r invite.ProvisionRecord, o invite.ProvisionObservation, pub ed25519.PublicKey) bool {
	return VerifyWithReport(ctx, s, r, o, pub, nil)
}

// Report contains only fixed stage labels, never peer IDs, nonces or secrets.
func VerifyWithReport(ctx context.Context, s *tsnet.Server, r invite.ProvisionRecord, o invite.ProvisionObservation, pub ed25519.PublicKey, report func(string)) (verified bool) {
	stage := "identity"
	defer func() {
		if report != nil {
			if verified {
				report("passed")
			} else {
				report(stage)
			}
		}
	}()
	if len(pub) != 32 || identity(pub) != r.Device || o.NodeKey != r.NodeKey {
		return false
	}
	lc, e := s.LocalClient()
	if e != nil {
		return false
	}
	found := false
	stage = "network_map"
	for !found && ctx.Err() == nil {
		st, e := lc.Status(ctx)
		if e != nil {
			return false
		}
		for _, p := range st.Peer {
			if p.PublicKey.String() == o.NodeKey && p.InNetworkMap && p.Online {
				for _, ip := range p.TailscaleIPs {
					if ip.String() == o.IP {
						found = true
					}
				}
			}
		}
		if !found {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	if !found {
		return false
	}
	stage = "dial"
	conn, e := DialOnlyOverlay(ctx, lc, o.IP, 47477)
	if e != nil {
		return false
	}
	defer conn.Close()
	deadline := time.Now().Add(2 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	conn.SetDeadline(deadline)
	stage = "whois"
	// LocalAPI upgrade wraps the local control connection, so use the actual
	// fixed remote overlay address, resolved by the same authenticated netmap.
	who, e := lc.WhoIs(ctx, net.JoinHostPort(o.IP, Port))
	if e != nil || who.Node.Key.String() != o.NodeKey {
		return false
	}
	nonce := make([]byte, 32)
	if _, e = rand.Read(nonce); e != nil {
		return false
	}
	c := challenge{r.ID, r.Grant.Claims.ID, r.Device, o.NodeKey, o.IP, hex.EncodeToString(nonce), time.Now().Unix()}
	stage = "write_challenge"
	if frame(conn, c) != nil {
		return false
	}
	var reply response
	stage = "read_proof"
	if e := read(conn, &reply); e != nil {
		if e == io.EOF || e == io.ErrUnexpectedEOF {
			stage = "peer_rejected"
		}
		return false
	}
	sig, e := hex.DecodeString(reply.Signature)
	stage = "signature"
	return e == nil && len(sig) == 64 && ed25519.Verify(pub, transcript(c), sig) && ctx.Err() == nil
}

// Serve only accepts the administrator's provisioned verifier network key.
// It never answers an untrusted peer even if a wrong ACL accidentally allows it.
func Serve(ctx context.Context, s *tsnet.Server, key ed25519.PrivateKey, verifierNode string) error {
	return ServeWithReport(ctx, s, key, verifierNode, nil)
}
func ServeWithReport(ctx context.Context, s *tsnet.Server, key ed25519.PrivateKey, verifierNode string, report func(string)) error {
	if len(key) != 64 || verifierNode == "" {
		return invite.ErrDenied
	}
	ln, e := s.Listen("tcp", ":"+Port)
	if e != nil {
		return e
	}
	defer ln.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			ln.Close()
		case <-done:
		}
	}()
	for {
		conn, e := ln.Accept()
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		func() {
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(2 * time.Second))
			requestCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			lc, e := s.LocalClient()
			if e != nil {
				return
			}
			who, e := lc.WhoIs(requestCtx, conn.RemoteAddr().String())
			if e != nil || who.Node.Key.String() != verifierNode {
				return
			}
			var c challenge
			if read(conn, &c) != nil {
				return
			}
			if report != nil {
				report("challenge")
			}
			// Freshness is the verifier's random per-connection nonce + bounded
			// request deadline, not synchronized clocks on end-user machines.
			if len(c.Ticket) != 32 || len(c.Grant) != 32 || len(c.Nonce) != 64 || c.Device != identity(key.Public().(ed25519.PublicKey)) || c.Issued <= 0 {
				return
			}
			if _, e := hex.DecodeString(c.Nonce); e != nil {
				return
			}
			st, e := lc.Status(requestCtx)
			if e != nil || st.Self == nil || st.Self.PublicKey.String() != c.Node {
				return
			}
			ipMatch := false
			for _, ip := range st.TailscaleIPs {
				if ip.String() == c.IP {
					ipMatch = true
				}
			}
			if !ipMatch {
				return
			}
			if frame(conn, response{hex.EncodeToString(ed25519.Sign(key, transcript(c)))}) == nil && report != nil {
				report("signed")
			}
		}()
	}
}
