//go:build remoteapp_desktop_control

package main

import "remoteapp.local/selfhost-poc/invite"

// A compile-time constant, never a link variable or private/network selector.
// Selecting both desktop tags intentionally fails compilation (duplicate const).
const compiledDesktopPermission invite.DesktopPermission = "desktop-control"
