// Cross-platform native compiled-installer acceptance. No SDK on target.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type reply struct {
	Status string `json:"status"`
	Epoch  uint64 `json:"epoch"`
	Reason string `json:"reason"`
}

func main() {
	if e := run(); e != nil {
		fmt.Println("VALIDATION FAILED:", e)
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: compiled anchor + native first install + upgrade/floor/recovery. NOT signed product installer or live-session acceptance.")
}
func run() (err error) {
	if len(os.Args) != 1 {
		return fmt.Errorf("no arguments permitted")
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	dir := filepath.Dir(exe)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	manifest, e := os.ReadFile(filepath.Join(dir, "SHA256.json"))
	if e != nil {
		return e
	}
	var hashes map[string]string
	if len(manifest) > 8192 || json.Unmarshal(manifest, &hashes) != nil {
		return fmt.Errorf("invalid test manifest")
	}
	for _, name := range []string{"trust-journal" + suffix, "trust-install-v1" + suffix, "trust-install-v2" + suffix, "trust-install-other-root" + suffix, "old-profile.json", "same-epoch-profile.json", "other-root-profile.json", "other-service-profile.json", "expired-profile.json", "installation-anchor.json", "tampered/trust-journal" + suffix, "tampered/trust-install-v1" + suffix} {
		b, e := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		h := sha256.Sum256(b)
		if e != nil || hex.EncodeToString(h[:]) != hashes[name] {
			return fmt.Errorf("fixture hash mismatch: %s", name)
		}
	}
	runTool := func(tool string, want bool, epoch uint64, args ...string) error {
		cmd := exec.Command(filepath.Join(dir, filepath.FromSlash(tool+suffix)), args...)
		// Poisoned environment cannot supply a replacement anchor or floor.
		cmd.Env = append(os.Environ(), "REMOTEAPP_RELEASE_ROOT="+strings.Repeat("0", 64), "REMOTEAPP_MINIMUM_EPOCH=0")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		e := cmd.Start()
		if e != nil {
			return fmt.Errorf("fixture executable unavailable")
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case e = <-done:
		case <-time.After(20 * time.Second):
			cmd.Process.Kill()
			<-done
			return fmt.Errorf("fixture timeout")
		}
		var r reply
		if json.Unmarshal(bytes.TrimSpace(out.Bytes()), &r) != nil {
			return fmt.Errorf("invalid maintenance reply")
		}
		if r.Reason == "interactive_logon_required" {
			return fmt.Errorf("interactive_logon_required: use normal desktop PowerShell")
		}
		if want {
			if e != nil || r.Status != "usable" || r.Epoch != epoch {
				return fmt.Errorf("expected usable committed epoch %d", epoch)
			}
		} else {
			if e == nil || r.Status != "failed" || r.Reason != "trust_rejected" {
				return fmt.Errorf("negative gate not rejected")
			}
		}
		return nil
	}
	// Always exact test namespace cleanup, including corrupted records. The
	// compiler fixed the channel; neither checker nor operator supplies a scope.
	defer func() {
		if e := runTool("trust-install-v1", true, 0, "cleanup-test"); e != nil {
			if err == nil {
				err = e
			}
			fmt.Println("FAIL exact native fixture credential cleanup")
		} else {
			fmt.Println("PASS exact native fixture credential cleanup")
		}
	}()
	positive := func(tool string, epoch uint64, args ...string) error { return runTool(tool, true, epoch, args...) }
	negative := func(tool string, args ...string) error { return runTool(tool, false, 0, args...) }
	if e = positive("trust-install-v1", 1, "initialize"); e != nil {
		return e
	}
	if e = negative("trust-install-v1", "initialize"); e != nil {
		return e
	}
	if e = positive("trust-install-v1", 1, "inspect"); e != nil {
		return e
	}
	fmt.Println("PASS first installation and committed-state overwrite refusal")
	if e = negative("trust-install-v2", "inspect"); e != nil {
		return e
	}
	if e = negative("trust-install-v2", "upgrade", filepath.Join(dir, "other-root-profile.json")); e != nil {
		return e
	}
	if e = positive("trust-install-v1", 1, "inspect"); e != nil {
		return e
	}
	fmt.Println("PASS raised compiled floor closes old configuration; failed upgrade preserves prior commit")
	if e = positive("trust-install-v2", 2, "upgrade"); e != nil {
		return e
	}
	if e = positive("trust-install-v2", 2, "inspect"); e != nil {
		return e
	}
	if e = positive("trust-install-v1", 2, "inspect"); e != nil {
		return e
	}
	if e = negative("trust-install-v1", "upgrade"); e != nil {
		return e
	}
	for _, name := range []string{"old-profile.json", "same-epoch-profile.json", "other-root-profile.json", "other-service-profile.json", "expired-profile.json"} {
		if e = negative("trust-install-v2", "upgrade", filepath.Join(dir, name)); e != nil {
			return e
		}
		if e = positive("trust-install-v2", 2, "inspect"); e != nil {
			return e
		}
	}
	fmt.Println("PASS native upgrade/reopen, old-binary downgrade and signed candidate rejection")
	if e = negative("trust-install-other-root", "inspect"); e != nil {
		return e
	}
	if e = negative("trust-install-other-root", "initialize"); e != nil {
		return e
	}
	if e = negative("tampered/trust-install-v1", "inspect"); e != nil {
		return e
	}
	if e = positive("trust-install-v2", 2, "inspect"); e != nil {
		return e
	}
	fmt.Println("PASS fixed channel refuses root replacement; sidecar/environment ignored; changed helper pin rejected")
	if e = positive("trust-install-v2", 0, "fault-test", "corrupt"); e != nil {
		return e
	}
	if e = negative("trust-install-v2", "inspect"); e != nil {
		return e
	}
	if e = negative("trust-install-v2", "upgrade"); e != nil {
		return e
	}
	if e = negative("trust-install-v1", "initialize"); e != nil {
		return e
	}
	fmt.Println("PASS corrupt native state stays closed and never auto-initializes")
	return nil
}
