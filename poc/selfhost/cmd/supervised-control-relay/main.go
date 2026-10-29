// Same-user local management relay. stdin/stdout must be private pipes: replies
// may contain invitations/grants. Never invoke with secret command-line args.
package main

import (
	"context"
	"encoding/json"
	"os"
	"remoteapp.local/selfhost-poc/desktopcontrol"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/privatepipe"
	"time"
)

func run() error {
	if len(os.Args) != 2 {
		return privatepipe.ErrProtocol
	}
	var request installeddevice.Request
	if privatepipe.NewReader(os.Stdin).Read(&request) != nil {
		return privatepipe.ErrProtocol
	}
	if !installeddevice.ValidSupervisedCommand(request) && request != (installeddevice.Request{Op: "product_count"}) {
		return privatepipe.ErrProtocol
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	conn, e := desktopcontrol.Dial(ctx, os.Args[1])
	if e != nil {
		return e
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(25 * time.Second))
	if e = json.NewEncoder(conn).Encode(request); e != nil {
		return e
	}
	var reply json.RawMessage
	if e = privatepipe.NewReader(conn).Read(&reply); e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(reply)
}
func main() {
	if run() != nil {
		os.Exit(1)
	}
}
