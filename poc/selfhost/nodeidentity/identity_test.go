package nodeidentity

import (
	"os"
	"path/filepath"
	"strings"
	"tailscale.com/envknob"
	"testing"
)

func TestOfflinePrivateIdentityStableAndIndependent(t *testing.T) {
	d := t.TempDir()
	k, e := Prepare(d, "http://127.0.0.1:18443")
	if e != nil || !strings.HasPrefix(k, "nodekey:") || len(k) != 72 {
		t.Fatal(e)
	}
	again, e := Prepare(d, "http://127.0.0.1:18443")
	if e != nil || again != k {
		t.Fatal("identity changed")
	}
	other, e := Prepare(t.TempDir(), "http://127.0.0.1:18443")
	if e != nil || other == k {
		t.Fatal("shared identity")
	}
	st, _ := os.Stat(filepath.Join(d, "tailscaled.state"))
	if st.Mode().Perm() != 0600 {
		t.Fatal("private file permissions")
	}
	if _, e = Prepare(d, "https://mk.fengmap.com:8443"); e == nil {
		t.Fatal("control plane change accepted")
	}
}

func TestOnlineEnvironmentIsolatedFromHostedLogin(t *testing.T) {
	t.Setenv("TS_AUTHKEY", "test-inherited-key")
	t.Setenv("TSNET_FORCE_LOGIN", "true")
	t.Setenv("TS_DEBUG_ALWAYS_USE_DERP", "true")
	ConfigureOnline()
	if os.Getenv("TS_AUTHKEY") != "" || os.Getenv("TSNET_FORCE_LOGIN") != "" || !envknob.NoLogsNoSupport() || os.Getenv("TS_DEBUG_ALWAYS_USE_DERP") != "false" || os.Getenv("TS_USE_CACHED_NETMAP") != "false" {
		t.Fatal("inherited or hosted environment not isolated")
	}
}
func TestCorruptOrSymlinkStateNotRegenerated(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "tailscaled.state")
	if os.WriteFile(path, []byte("{}"), 0600) != nil {
		t.Fatal("write")
	}
	if _, e := Prepare(d, "http://127.0.0.1:18443"); e == nil {
		t.Fatal("corrupt reset")
	}
	content, _ := os.ReadFile(path)
	if string(content) != "{}" {
		t.Fatal("overwritten")
	}
	other := t.TempDir()
	if os.Symlink(path, filepath.Join(other, "tailscaled.state")) != nil {
		t.Fatal("symlink")
	}
	if _, e := Prepare(other, "http://127.0.0.1:18443"); e == nil {
		t.Fatal("symlink accepted")
	}
}

func TestMissingStateInExistingPrivateDirectoryNotReset(t *testing.T) {
	d := t.TempDir()
	if _, e := Prepare(d, "http://127.0.0.1:18443"); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(d, "tailscaled.state")
	if os.Rename(path, path+".backup") != nil {
		t.Fatal("rename")
	}
	if _, e := Prepare(d, "http://127.0.0.1:18443"); e == nil {
		t.Fatal("missing existing identity reset")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("state regenerated")
	}
}
