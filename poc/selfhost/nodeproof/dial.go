package nodeproof

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"

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
		res.Body.Close()
		if res.StatusCode == http.StatusOK && res.Header.Get("Dial-Self") == "true" {
			startuptrace.Emit(ctx, startuptrace.OverlayRouteMissing)
			return nil, ErrNoOverlayRoute
		}
		startuptrace.Emit(ctx, startuptrace.OverlayUpgradeRejected)
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
