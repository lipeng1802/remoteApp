package nodeidentity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileMetadataReadDoesNotInitializeOrRewrite(t *testing.T) {
	dir := t.TempDir()
	if _, e := ReadProfileID(dir); e == nil {
		t.Fatal("missing state accepted")
	}
	entries, e := os.ReadDir(dir)
	if e != nil || len(entries) != 0 {
		t.Fatal("metadata read initialized missing state")
	}
	if _, e := Prepare(dir, "http://127.0.0.1:18443"); e != nil {
		t.Fatal("offline TEST prepare")
	}
	path := filepath.Join(dir, "tailscaled.state")
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal("fixture read")
	}
	id, e := ReadProfileID(dir)
	if e != nil || len(id) != 4 {
		t.Fatal("metadata read")
	}
	after, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("metadata read rewrote state")
	}
	if os.WriteFile(path, []byte("corrupt public fixture"), 0600) != nil {
		t.Fatal("fixture write")
	}
	if _, e = ReadProfileID(dir); e == nil {
		t.Fatal("corrupt state accepted")
	}
	after, e = os.ReadFile(path)
	if e != nil || string(after) != "corrupt public fixture" {
		t.Fatal("corrupt state repaired")
	}
}
