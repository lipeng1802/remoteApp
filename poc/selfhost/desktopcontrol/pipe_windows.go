package desktopcontrol

import (
	"context"
	"net"
	"regexp"

	winio "github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
	"remoteapp.local/selfhost-poc/privatepipe"
)

func validPipe(name string) bool {
	return regexp.MustCompile(`^\\\\\.\\pipe\\remoteapp-session-[0-9a-f]{32}$`).MatchString(name)
}
func Listen(name string) (net.Listener, error) {
	if !validPipe(name) {
		return nil, privatepipe.ErrProtocol
	}
	u, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return nil, e
	}
	// First-instance collision rejection / remote clients rejected by winio.
	return winio.ListenPipe(name, &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;" + u.User.Sid.String() + ")", InputBufferSize: 8192, OutputBufferSize: 8192})
}
func Dial(ctx context.Context, name string) (net.Conn, error) {
	if !validPipe(name) {
		return nil, privatepipe.ErrProtocol
	}
	return winio.DialPipeContext(ctx, name)
}
