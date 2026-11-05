//go:build darwin || linux

package headscale

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"remoteapp.local/selfhost-poc/productprofile"
)

// DesktopTestInstance pins CLI and policy operations to one private TEST root.
// The provider process and TLS material are prepared separately by its owner.
type DesktopTestInstance struct{ root string }

const desktopRootPrefix = "remoteapp-desktop-test-"

// PrepareDesktopTest writes a new fixed Headscale config and policy directory
// inside an empty, privately owned TEST stage. It never replaces an old stage.
func PrepareDesktopTest(root string) (*DesktopTestInstance, error) {
	if !desktopRoot(root) {
		return nil, ErrDriver
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		return nil, ErrDriver
	}
	instance := &DesktopTestInstance{root: root}
	if os.Mkdir(instance.PolicyDirectory(), 0700) != nil {
		return nil, ErrDriver
	}
	f, err := os.OpenFile(instance.configPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, ErrDriver
	}
	_, writeErr := f.Write(desktopConfig(root))
	syncErr := f.Sync()
	closeErr := f.Close()
	d, err := os.Open(root)
	if err != nil {
		return nil, ErrDriver
	}
	defer d.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil || d.Sync() != nil {
		return nil, ErrDriver
	}
	return instance, nil
}

// OpenDesktopTest verifies the exact prepared config before reopening CLI access.
func OpenDesktopTest(root string) (*DesktopTestInstance, error) {
	instance := &DesktopTestInstance{root: root}
	if !instance.valid() {
		return nil, ErrDriver
	}
	return instance, nil
}

func (d *DesktopTestInstance) PolicyDirectory() string { return filepath.Join(d.root, "policy") }
func (d *DesktopTestInstance) configPath() string      { return filepath.Join(d.root, "headscale.yaml") }

func (d *DesktopTestInstance) valid() bool {
	if d == nil || !desktopRoot(d.root) || !desktopDirectory(d.PolicyDirectory()) {
		return false
	}
	want := desktopConfig(d.root)
	f, err := os.Lstat(d.configPath())
	if err != nil || !desktopOwner(f) || f.Mode() != 0600 || f.Size() != int64(len(want)) {
		return false
	}
	actual, err := os.ReadFile(d.configPath())
	return err == nil && bytes.Equal(actual, want)
}

// Run uses the installed CLI binary with only this instance's configuration
// and socket. Inherited config/environment overrides are discarded.
func (d *DesktopTestInstance) Run(ctx context.Context, args ...string) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil || !d.valid() {
		return nil, ErrDriver
	}
	socket, err := os.Lstat(filepath.Join(d.root, "admin.sock"))
	if err != nil || !desktopOwner(socket) || socket.Mode() != os.ModeSocket|0600 {
		return nil, ErrDriver
	}
	args = append([]string{"-c", d.configPath(), "-o", "json"}, args...)
	cmd := exec.CommandContext(ctx, "/opt/remoteapp-poc/headscale", args...)
	cmd.Env = []string{"LANG=C"}
	var out limitedBuffer
	cmd.Stdout = &out
	if cmd.Run() != nil {
		return nil, ErrDriver
	}
	return out.Bytes(), nil
}

func desktopRoot(path string) bool {
	name := filepath.Base(path)
	id := strings.TrimPrefix(name, desktopRootPrefix)
	raw, err := hex.DecodeString(id)
	if id == name || err != nil || len(raw) != 8 || hex.EncodeToString(raw) != id ||
		!filepath.IsAbs(path) || filepath.Clean(path) != path || len(filepath.Join(path, "admin.sock")) > 103 ||
		strings.IndexFunc(path, func(r rune) bool {
			return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/_-.", r)
		}) >= 0 {
		return false
	}
	return desktopDirectory(filepath.Dir(path)) && desktopDirectory(path)
}

func desktopDirectory(path string) bool {
	real, err := filepath.EvalSymlinks(path)
	f, statErr := os.Lstat(path)
	return err == nil && real == path && statErr == nil && desktopOwner(f) && f.Mode() == os.ModeDir|0700
}

func desktopOwner(f os.FileInfo) bool {
	st, ok := f.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uint32(os.Geteuid())
}

func desktopConfig(root string) []byte {
	return []byte(fmt.Sprintf(`server_url: %[1]s
listen_addr: 0.0.0.0:19443
metrics_listen_addr: 127.0.0.1:29090
grpc_listen_addr: 127.0.0.1:25443
grpc_allow_insecure: false
tls_cert_path: %[2]s/control-tls.crt
tls_key_path: %[2]s/control-tls.key
noise:
  private_key_path: %[2]s/noise.key
prefixes:
  v4: 100.120.0.0/24
  v6: fd7a:115c:a1e0:abce::/64
  allocation: sequential
derp:
  server:
    enabled: true
    region_id: 902
    region_code: remoteapp-desktop-test
    region_name: RemoteApp Desktop TEST
    verify_clients: true
    stun_listen_addr: 0.0.0.0:3479
    private_key_path: %[2]s/derp.key
    automatically_add_embedded_derp_region: true
    ipv4: 182.92.117.114
  urls: []
  paths: []
  auto_update_enabled: false
disable_check_updates: true
database:
  type: sqlite
  sqlite:
    path: %[2]s/db.sqlite
    write_ahead_log: true
dns:
  magic_dns: false
  override_local_dns: false
policy:
  mode: database
unix_socket: %[2]s/admin.sock
unix_socket_permission: "0600"
log:
  level: warn
  format: json
logtail:
  enabled: false
taildrop:
  enabled: false
`, productprofile.DesktopControlURL, root))
}
