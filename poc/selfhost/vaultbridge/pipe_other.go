//go:build !windows

package vaultbridge

import (
	"context"
	"net"
	"remoteapp.local/selfhost-poc/privatepipe"
)

func Listen(string) (net.Listener, error)            { return nil, privatepipe.ErrProtocol }
func Dial(context.Context, string) (net.Conn, error) { return nil, privatepipe.ErrProtocol }
