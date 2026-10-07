package vaultbridge

import (
	"context"
	"net"
	"regexp"

	winio "github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
	"remoteapp.local/selfhost-poc/privatepipe"
)

func validPipe(name string) bool {
	return regexp.MustCompile(`^\\\\\.\\pipe\\remoteapp-vault-[0-9a-f]{32}$`).MatchString(name)
}

func Listen(name string) (net.Listener, error) {
	if !validPipe(name) {
		return nil, privatepipe.ErrProtocol
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	// Protected DACL: only this user. go-winio uses PIPE_REJECT_REMOTE_CLIENTS
	// and FILE_CREATE for the first instance (rejects name collisions).
	// No TCP/firewall/listener exposure.
	return winio.ListenPipe(name, &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;" + user.User.Sid.String() + ")", InputBufferSize: 8192, OutputBufferSize: 8192})
}
func Dial(ctx context.Context, name string) (net.Conn, error) {
	if !validPipe(name) {
		return nil, privatepipe.ErrProtocol
	}
	return winio.DialPipeContext(ctx, name)
}
