package nodeidentity

import (
	"os"
	"tailscale.com/envknob"
)

// ConfigureOnline affects only this independent helper process. Do not inherit
// system-tailnet login/OAuth configuration or upload logs to hosted Tailscale.
func ConfigureOnline() {
	for _, name := range []string{"TS_AUTHKEY", "TS_AUTH_KEY", "TS_CONTROL_URL", "TS_CLIENT_SECRET", "TSNET_FORCE_LOGIN"} {
		os.Unsetenv(name)
	}
	envknob.SetNoLogsNoSupport()
	envknob.Setenv("TS_USE_CACHED_NETMAP", "false")
	// ALWAYS_USE_DERP reproduced leaking dummy sockets in the pinned SDK.
	envknob.Setenv("TS_DEBUG_ALWAYS_USE_DERP", "false")
	envknob.Setenv("TS_DEBUG_NEVER_DIRECT_UDP", "true")
}
