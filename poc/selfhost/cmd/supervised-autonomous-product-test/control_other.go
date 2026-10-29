//go:build !windows

package main

import (
	"context"
	"remoteapp.local/selfhost-poc/productmaintenance"
)

func startDesktopControl(context.Context, *productmaintenance.Controller) (string, func(), error) {
	return "", func() {}, nil
}
