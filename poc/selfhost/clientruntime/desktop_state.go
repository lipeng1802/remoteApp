package clientruntime

import (
	"context"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/session"
)

// ShowInvite displays the authority's one-time codes only in the target GUI.
func (p *PreparedDesktop) ShowInvite(ctx context.Context, device, code string) error {
	if p == nil || ctx == nil || ctx.Err() != nil || !validDesktopCode(device, code) {
		return session.ErrDenied
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.server || p.inviteShown || p.decisionStarted || !p.idle() {
		return session.ErrDenied
	}
	p.inviteShown = true
	raw, err := p.shortExchange(ctx, map[string]string{"Op": "show_invite", "DeviceCode": device, "Code": code})
	var ack struct {
		Status     string `json:"status"`
		Permission string
	}
	if err != nil || decodeProductReady(raw, &ack) != nil || ack.Status != "invite_displayed" || ack.Permission != p.policy.Permission() {
		p.broken = true
		return session.ErrDenied
	}
	return nil
}

func validDesktopCode(device, code string) bool {
	return len(device) == 12 && strings.Trim(device, "0123456789abcdef") == "" &&
		len(code) == 8 && strings.Trim(code, "0123456789") == ""
}

// DesktopState contains only aggregate, local GUI telemetry.
type DesktopState struct {
	Status        string `json:"status"`
	Frames        int64  `json:"frames"`
	Inputs        int64  `json:"inputs"`
	Releases      int64  `json:"releases"`
	Held          int64  `json:"held"`
	StopRequested bool   `json:"stop_requested"`
	Ended         bool   `json:"ended"`
	Failed        bool   `json:"failed"`
}

// State serializes a short read with activation. A human decision or a TLS
// startup exchange in flight yields unavailable without touching the pipe.
func (p *PreparedDesktop) State(ctx context.Context) (DesktopState, error) {
	if p == nil || ctx == nil || ctx.Err() != nil {
		return DesktopState{}, session.ErrDenied
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.idle() {
		return DesktopState{}, session.ErrDenied
	}
	raw, err := p.shortExchange(ctx, map[string]string{"Op": "desktop_state"})
	var r struct {
		Status        string `json:"status"`
		Frames        *int64 `json:"frames"`
		Inputs        *int64 `json:"inputs"`
		Releases      *int64 `json:"releases"`
		Held          *int64 `json:"held"`
		StopRequested *bool  `json:"stop_requested"`
		Ended         *bool  `json:"ended"`
		Failed        *bool  `json:"failed"`
	}
	if err != nil || decodeProductReady(raw, &r) != nil || r.Status != "desktop_state" ||
		r.Frames == nil || *r.Frames < 0 || r.Inputs == nil || *r.Inputs < 0 ||
		r.StopRequested == nil || r.Ended == nil || r.Failed == nil ||
		p.server != (r.Releases != nil) || p.server != (r.Held != nil) ||
		(p.server && (*r.Releases < 0 || *r.Held < 0)) ||
		(!p.consumed && (*r.Frames != 0 || *r.Inputs != 0 || *r.Ended || p.server && (*r.Held != 0 || *r.Releases != 0))) {
		p.broken = true
		return DesktopState{}, session.ErrDenied
	}
	state := DesktopState{Status: r.Status, Frames: *r.Frames, Inputs: *r.Inputs,
		StopRequested: *r.StopRequested, Ended: *r.Ended, Failed: *r.Failed}
	if p.server {
		state.Releases, state.Held = *r.Releases, *r.Held
	}
	return state, nil
}

func (p *PreparedDesktop) idle() bool {
	return p.child != nil && !p.closing && !p.broken && p.opCancel == nil
}

func (p *PreparedDesktop) shortExchange(ctx context.Context, frame map[string]string) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, session.ErrDenied
	}
	life, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	raw, err := p.child.Exchange(life, frame)
	if life.Err() != nil {
		return nil, life.Err()
	}
	return raw, err
}
