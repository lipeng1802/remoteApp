package nodeproof

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"

	"remoteapp.local/selfhost-poc/startuptrace"

	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/net/netutil"
)

var ErrNoOverlayRoute = errors.New("overlay route withdrawn")
var ErrOverlayDial = errors.New("overlay dial rejected")

// DialOnlyOverlay uses the pinned LocalAPI upgrade but deliberately NEVER
// follows Dial-Self, unlike Client.DialTCP/tsnet.Dial. Thus a withdrawn ACL/peer
// route cannot silently use the machine's installed Tailscale or system socket.
func DialOnlyOverlay(ctx context.Context, lc *local.Client, host string, port uint16) (net.Conn, error) {
	connections := make(chan net.Conn, 1)
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{GotConn: func(v httptrace.GotConnInfo) {
		select {
		case connections <- v.Conn:
		default:
		}
	}})
	req, e := http.NewRequestWithContext(ctx, "POST", "http://"+apitype.LocalAPIHost+"/localapi/v0/dial", nil)
	if e != nil {
		startuptrace.Emit(ctx, startuptrace.OverlayRequestFailed)
		return nil, ErrOverlayDial
	}
	req.Header = http.Header{"Upgrade": {"ts-dial"}, "Connection": {"upgrade"}, "Dial-Host": {host}, "Dial-Port": {strconv.Itoa(int(port))}, "Dial-Network": {"tcp"}}
	res, e := lc.DoLocalRequest(req)
	if e != nil {
		startuptrace.Emit(ctx, startuptrace.OverlayRequestFailed)
		return nil, ErrOverlayDial
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		if res.StatusCode == http.StatusOK && res.Header.Get("Dial-Self") == "true" {
			res.Body.Close()
			startuptrace.Emit(ctx, startuptrace.OverlayRouteMissing)
			return nil, ErrNoOverlayRoute
		}
		stage := startuptrace.OverlayUpgradeRejected
		// TEST-only diagnostics: this never authorizes retry or changes the error.
		// Do not retain/log any raw response body or address.
		if startuptrace.Enabled(ctx) {
			body, readErr := io.ReadAll(io.LimitReader(res.Body, 513))
			if readErr == nil && len(body) <= 512 {
				stage = classifyUpgradeFailure(res.StatusCode, string(body))
			}
		}
		res.Body.Close()
		startuptrace.Emit(ctx, stage)
		return nil, ErrOverlayDial
	}
	var conn net.Conn
	select {
	case conn = <-connections:
	default:
	}
	rwc, ok := res.Body.(io.ReadWriteCloser)
	if conn == nil || !ok {
		startuptrace.Emit(ctx, startuptrace.OverlayResponseInvalid)
		res.Body.Close()
		return nil, ErrOverlayDial
	}
	startuptrace.Emit(ctx, startuptrace.OverlayUpgraded)
	return netutil.NewAltReadWriteCloserConn(rwc, conn), nil
}

// classifyUpgradeFailure recognizes only the audited pinned LocalAPI branches.
// Unknown/malformed/private text remains a single fixed rejection label. No
// label changes the fail-closed transport return value or permits fallback.
func classifyUpgradeFailure(status int, body string) startuptrace.Stage {
	switch status {
	case http.StatusBadRequest:
		if body == "bad ts-dial upgrade\n" || body == "make request over HTTP/1\n" {
			return startuptrace.OverlayBadUpgrade
		}
	case http.StatusForbidden:
		return startuptrace.OverlayPermissionDenied
	case http.StatusBadGateway:
		if strings.HasPrefix(body, "resolve failure: ") {
			return startuptrace.OverlayResolveFailed
		}
		if strings.HasPrefix(body, "dial failure: ") {
			if strings.Contains(body, "connection refused") || strings.Contains(body, "connection was refused") {
				return startuptrace.OverlayConnectRefused
			}
			if strings.Contains(body, "i/o timeout") || strings.Contains(body, "context deadline exceeded") {
				return startuptrace.OverlayConnectTimeout
			}
			return startuptrace.OverlayConnectFailed
		}
	}
	return startuptrace.OverlayUpgradeRejected
}
