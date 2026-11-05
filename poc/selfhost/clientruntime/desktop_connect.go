package clientruntime

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/session"
)

// DesktopConnectIntent contains local GUI input, never a grant. It is not an
// argv/config/JSON object or log record. Claim still needs the OS signer's real
// authority request, matching approval and every original trust/role/time gate.
type DesktopConnectIntent struct {
	target, code string
	permission   invite.DesktopPermission
}

func (*DesktopConnectIntent) String() string               { return "DesktopConnectIntent(private)" }
func (*DesktopConnectIntent) GoString() string             { return "DesktopConnectIntent(private)" }
func (*DesktopConnectIntent) MarshalJSON() ([]byte, error) { return nil, session.ErrDenied }
func (i *DesktopConnectIntent) Claim() invite.Body {
	if i == nil {
		return invite.Body{}
	}
	return invite.Body{Action: "claim", TargetCode: i.target, Code: i.code, Permission: i.permission}
}

// ConnectIntent waits for the local controller's Connect button on the SAME
// prepared child. A nil intent with nil error is explicit local cancellation.
// Every attempt is single-use; waiting is before provision and TLS startup.
func (p *PreparedDesktop) ConnectIntent(ctx context.Context) (*DesktopConnectIntent, error) {
	if p == nil || ctx == nil || ctx.Err() != nil {
		return nil, session.ErrDenied
	}
	p.mu.Lock()
	if p.child == nil || p.server || p.broken || p.closing || p.connectStarted || p.decisionStarted || p.consumed || p.opCancel != nil {
		p.mu.Unlock()
		return nil, session.ErrDenied
	}
	life, cancel := context.WithTimeout(ctx, 120*time.Second)
	p.connectStarted = true
	p.opCancel, p.opDone = cancel, make(chan struct{})
	done := p.opDone
	p.mu.Unlock()
	defer func() { cancel(); p.mu.Lock(); p.opCancel = nil; close(done); p.mu.Unlock() }()
	raw, e := p.child.Exchange(life, map[string]string{"Op": "connect_intent"})
	var r struct {
		Status                       string `json:"status"`
		Permission, TargetCode, Code string
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if e != nil || life.Err() != nil || p.closing || decodeProductReady(raw, &r) != nil || r.Permission != p.policy.Permission() {
		p.broken = true
		return nil, session.ErrDenied
	}
	if r.Status == "desktop_connect_cancelled" && r.TargetCode == "" && r.Code == "" {
		p.broken = true
		return nil, nil
	}
	target, e := hex.DecodeString(r.TargetCode)
	if r.Status != "desktop_connect" || e != nil || len(target) != 6 || hex.EncodeToString(target) != r.TargetCode || len(r.Code) != 8 || strings.Trim(r.Code, "0123456789") != "" {
		p.broken = true
		return nil, session.ErrDenied
	}
	return &DesktopConnectIntent{target: r.TargetCode, code: r.Code, permission: invite.DesktopPermission(p.policy.Permission())}, nil
}
