// Package startuptrace exposes only fixed TEST startup stages, never raw errors.
package startuptrace

import (
	"context"
	"fmt"
	"os"
)

type Stage string

const (
	TargetTLS        Stage = "target_tls"
	TargetListen     Stage = "target_listen"
	TargetSecret     Stage = "target_secret"
	TargetServe      Stage = "target_serve"
	ControllerListen Stage = "controller_listen"
	ControllerDial   Stage = "controller_dial"
	ControllerSecret Stage = "controller_secret"
	ControllerTLS    Stage = "controller_tls"
	ControllerAccept Stage = "controller_accept"
	ControllerBridge Stage = "controller_bridge"
)

type Report struct {
	Status string `json:"status"`
	Stage  string `json:"stage"`
}

func Valid(s string) bool {
	switch Stage(s) {
	case TargetTLS, TargetListen, TargetSecret, TargetServe, ControllerListen, ControllerDial, ControllerSecret, ControllerTLS, ControllerAccept, ControllerBridge:
		return true
	}
	return false
}

type key struct{}

// Context is trusted compiled wiring; no peer or environment can enable it.
func Context(ctx context.Context, test bool) context.Context {
	return context.WithValue(ctx, key{}, test)
}
func Emit(ctx context.Context, s Stage) {
	if enabled, _ := ctx.Value(key{}).(bool); enabled && Valid(string(s)) {
		fmt.Fprintln(os.Stderr, "PRODUCT_STAGE "+string(s))
	}
}
