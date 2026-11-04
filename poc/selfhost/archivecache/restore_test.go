package archivecache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) (string, string, string, Plan) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base, output := filepath.Join(root, "cache"), filepath.Join(root, "new")
	wire := filepath.Join(output, "_transport")
	for _, d := range []string{base, output, wire} {
		if os.Mkdir(d, 0700) != nil {
			t.Fatal("mkdir")
		}
	}
	makeSpec := func(name string, b []byte) File {
		h := sha256.Sum256(b)
		return File{name, int64(len(b)), hex.EncodeToString(h[:])}
	}
	os.WriteFile(filepath.Join(base, "old.exe"), []byte("cached"), 0600)
	os.WriteFile(filepath.Join(wire, "literal.bin"), []byte("fresh"), 0600)
	p := Plan{Version: 1, Block: 32768, Literal: makeSpec("literal.bin", []byte("fresh")), Sources: []File{makeSpec("old.exe", []byte("cached"))}, Targets: []Target{{makeSpec("payload/new.exe", []byte("cachedfresh")), []Step{{1, 0, 6}, {0, 0, 5}}}}}
	return base, output, wire, p
}
func TestRestoreExactFreshBytesAndRejectUnknownInputs(t *testing.T) {
	for _, fault := range []string{"", "cache", "literal", "target_hash", "traversal", "overrun", "existing", "link", "step_length", "unknown"} {
		t.Run(fault, func(t *testing.T) {
			base, output, wire, p := fixture(t)
			switch fault {
			case "cache":
				os.WriteFile(filepath.Join(base, "old.exe"), []byte("change"), 0600)
			case "literal":
				os.WriteFile(filepath.Join(wire, "literal.bin"), []byte("shortened"), 0600)
			case "target_hash":
				p.Targets[0].SHA256 = p.Sources[0].SHA256
			case "traversal":
				p.Targets[0].Name = "../escape.exe"
			case "overrun":
				p.Targets[0].Steps[0][1] = 999
			case "existing":
				os.Mkdir(filepath.Join(output, "payload"), 0700)
				os.WriteFile(filepath.Join(output, "payload/new.exe"), []byte("KEEP"), 0600)
			case "link":
				os.Symlink(base, filepath.Join(output, "payload"))
			}
			raw, _ := json.Marshal(p)
			if fault == "step_length" {
				raw = []byte(`{"version":1,"block":32768,"literal":{},"sources":[],"targets":[{"steps":[[1,0,6,9]]}]}`)
			}
			if fault == "unknown" {
				raw = append([]byte(`{"extra":true,`), raw[1:]...)
			}
			os.WriteFile(filepath.Join(wire, "plan.json"), raw, 0600)
			err := Restore(base, output, wire)
			if fault != "" {
				if err != ErrArchive {
					t.Fatal("invalid reconstruction accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(filepath.Join(output, "payload/new.exe"))
			if string(got) != "cachedfresh" {
				t.Fatal("bytes differ")
			}
			if Restore(base, output, wire) != ErrArchive {
				t.Fatal("unknown output replay accepted")
			}
		})
	}
}
