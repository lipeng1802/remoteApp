//go:build linux || darwin

package broker

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func preparedNetworkFixture(t *testing.T) (DaemonConfig, DaemonNetworkConfig) {
	t.Helper()
	c, n := networkDaemonFixture(t)
	node := filepath.Join(n.VerifierDirectory, "node")
	if err := os.Mkdir(node, 0700); err != nil {
		t.Fatal(err)
	}
	journal := `{"Version":1,"Control":"https://mk.fengmap.com:8443","Phase":"issued","Record":{"ID":"` + strings.Repeat("1", 32) + `","NodeKey":"nodekey:` + strings.Repeat("2", 64) + `","Expires":1,"UserID":"1","KeyID":"2"}}`
	for file, b := range map[string]string{filepath.Join(n.VerifierDirectory, "journal.json"): journal, filepath.Join(node, "tailscaled.state"): "opaque-test-state"} {
		if err := os.WriteFile(file, []byte(b), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return c, n
}

func snapshotPreparation(t *testing.T, c DaemonConfig, n DaemonNetworkConfig) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	for _, root := range []string{c.StateDirectory, n.VerifierDirectory, n.PolicyDirectory, filepath.Dir(c.Profile)} {
		err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				b, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				out[p] = sha256.Sum256(b)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestNetworkPreparationReadOnlyAndRepeatable(t *testing.T) {
	c, n := preparedNetworkFixture(t)
	before := snapshotPreparation(t, c, n)
	for range 2 {
		if err := CheckNetworkPreparation(c); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(before, snapshotPreparation(t, c, n)) {
		t.Fatal("preflight changed files or created locks")
	}
	for _, lock := range []string{filepath.Join(c.StateDirectory, "writer.lock"), filepath.Join(n.VerifierDirectory, "owner.lock")} {
		if _, err := os.Lstat(lock); !os.IsNotExist(err) {
			t.Fatal("preflight acquired ownership")
		}
	}
}

func TestNetworkPreparationRejectsUnsafeSnapshotsWithoutRepair(t *testing.T) {
	for _, scenario := range []string{"store_lock", "verifier_lock", "missing_journal", "invalid_journal", "public_state", "symlink_state", "retired", "wrong_anchor", "wrong_service", "future_floor", "missing_node", "relative_socket"} {
		t.Run(scenario, func(t *testing.T) {
			c, n := preparedNetworkFixture(t)
			journal := filepath.Join(n.VerifierDirectory, "journal.json")
			state := filepath.Join(n.VerifierDirectory, "node", "tailscaled.state")
			switch scenario {
			case "store_lock":
				os.WriteFile(filepath.Join(c.StateDirectory, "writer.lock"), []byte("owned"), 0600)
			case "verifier_lock":
				os.WriteFile(filepath.Join(n.VerifierDirectory, "owner.lock"), []byte("owned"), 0600)
			case "missing_journal":
				os.Remove(journal)
			case "invalid_journal":
				os.WriteFile(journal, []byte(`{"Version":1,"version":1}`), 0600)
			case "public_state":
				os.Chmod(state, 0644)
			case "symlink_state":
				os.Remove(state)
				os.Symlink(journal, state)
			case "retired":
				b, _ := os.ReadFile(journal)
				os.WriteFile(journal, []byte(strings.Replace(string(b), "issued", "retired", 1)), 0600)
			case "wrong_anchor":
				c.ReleaseRoot = strings.Repeat("0", 64)
			case "wrong_service":
				c.Service = "other-service"
			case "future_floor":
				c.MinimumEpoch = 2
			case "missing_node":
				os.Remove(state)
			case "relative_socket":
				c.Socket = "relative.sock"
			}
			before := snapshotPreparation(t, c, n)
			if CheckNetworkPreparation(c) == nil {
				t.Fatal("unsafe snapshot accepted")
			}
			if !reflect.DeepEqual(before, snapshotPreparation(t, c, n)) {
				t.Fatal("failure repaired or modified snapshot")
			}
		})
	}
}
