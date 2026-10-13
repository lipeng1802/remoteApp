package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/verifier"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var candidatePath string

func serviceCommand(args ...string) ([]byte, error) {
	ctx, cancel := bounded()
	defer cancel()
	c := exec.CommandContext(ctx, "systemctl", args...)
	c.Stderr = nil
	return c.Output()
}

// Render the reviewed candidate, substituting only isolated trusted fixture
// paths/public pins. Never shell-quote caller strings into an ExecStart.
func renderCandidate(template, binary, base string, args []string) (string, error) {
	if len(args) != 10 || args[8] != "--network-config" {
		return "", denied
	}
	for _, p := range append([]string{binary, base}, args...) {
		if strings.ContainsAny(p, "\n\r\t \"'\\%") {
			return "", denied
		}
	}
	for _, required := range []string{"Restart=no\n", "TimeoutStopSec=45\n", "Wants=remoteapp-poc.service\n", "CapabilityBoundingSet=CAP_CHOWN CAP_DAC_OVERRIDE CAP_NET_RAW\n", "ExecStartPre=", "ExecStart="} {
		if !strings.Contains(template, required) {
			return "", denied
		}
	}
	if strings.Contains(template, "Requires=") {
		return "", denied
	}
	r := strings.NewReplacer(
		"/opt/remoteapp-prerelease/authority-daemon", binary,
		"/etc/remoteapp-authority/service-profile.json", args[0],
		"@RELEASE_ROOT@", args[1], "remoteapp-prerelease 1", args[2]+" "+args[3],
		"/var/lib/remoteapp-authority", args[4], "/run/remoteapp-authority/authority.sock", args[5],
		"@FRONTEND_UID@", args[6], "@FRONTEND_GID@", args[7],
		"/etc/remoteapp-authority/network.json", args[9],
		"/etc/remoteapp-authority", filepath.Dir(args[0]),
		"/run/remoteapp-authority", filepath.Dir(args[5]),
		"/var/lib/remoteapp-verifier", filepath.Join(base, "verifier"),
		"/var/lib/remoteapp-network-policy", filepath.Join(base, "policy"),
		"/var/lib/remoteapp-prerelease", filepath.Join(base, "material"),
	)
	unit := r.Replace(template)
	// PrivateTmp hides both /tmp and /var/tmp. Fixture lives under /run,
	// retaining the candidate sandbox rather than disabling PrivateTmp.
	if strings.Contains(unit, "@") || !strings.HasPrefix(binary, "/run/remoteapp-daemon-check-") || !strings.HasPrefix(base, "/run/remoteapp-daemon-check-") {
		return "", denied
	}
	return unit, nil
}

func launchService(binary string, args []string, base string) (*child, error) {
	b, err := os.ReadFile(candidatePath)
	if err != nil {
		return nil, denied
	}
	unit, err := renderCandidate(string(b), binary, base, args)
	if err != nil {
		return nil, err
	}
	token := make([]byte, 8)
	if _, err = rand.Read(token); err != nil {
		return nil, denied
	}
	p := &child{unit: "remoteapp-isolated-network-" + hex.EncodeToString(token) + ".service"}
	path := filepath.Join("/run/systemd/system", p.unit)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, denied
	}
	_, we := f.WriteString(unit)
	ce := f.Close()
	if we != nil || ce != nil {
		return p, denied
	}
	if _, err = serviceCommand("daemon-reload"); err != nil {
		return p, denied
	}
	if _, err = serviceCommand("start", p.unit); err != nil {
		return p, denied
	}
	return p, nil
}

func (p *child) state() (map[string]string, error) {
	b, err := serviceCommand("show", p.unit, "-p", "ActiveState", "-p", "SubState", "-p", "MainPID", "-p", "ExecMainPID", "-p", "ExecMainCode", "-p", "ExecMainStatus", "-p", "Restart")
	if err != nil {
		return nil, denied
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			return nil, denied
		}
		out[kv[0]] = kv[1]
	}
	if out["Restart"] != "no" {
		return nil, denied
	}
	return out, nil
}

func (p *child) logged(label string) bool {
	ctx, cancel := bounded()
	defer cancel()
	b, err := exec.CommandContext(ctx, "journalctl", "--no-pager", "-o", "cat", "-u", p.unit, "_PID="+strconv.Itoa(p.pid)).Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if line == label {
			return true
		}
	}
	return false
}

func (p *child) serviceReady() error {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		s, err := p.state()
		if err != nil {
			return denied
		}
		p.pid, _ = strconv.Atoi(s["MainPID"])
		if s["ActiveState"] == "failed" {
			return denied
		}
		if s["ActiveState"] == "active" && p.pid > 1 && p.logged("READY sole authority; supervised network enabled on private broker") {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return denied
}

func (p *child) serviceWait(normal bool) error {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		s, err := p.state()
		if err != nil {
			return denied
		}
		if s["MainPID"] == "0" && (s["ActiveState"] == "inactive" || s["ActiveState"] == "failed") {
			pid, _ := strconv.Atoi(s["ExecMainPID"])
			if p.pid == 0 {
				if pid > 1 {
					p.pid = pid
				} else if normal {
					p.pid = p.lastStoppedPID()
				}
			}
			if p.pid <= 1 || (pid != 0 && pid != p.pid) || syscall.Kill(p.pid, 0) != syscall.ESRCH {
				return denied
			}
			p.joined = true // Actual PID exit, not proof of normal worker join.
			label := "STOPPED authority"
			if !normal {
				label = "FAIL authority unavailable"
			}
			codeOK := s["ExecMainCode"] == "1" || (normal && s["ExecMainCode"] == "0" && pid == 0 && s["ActiveState"] == "inactive")
			if !codeOK || (s["ExecMainStatus"] == "0") != normal || !p.logged(label) {
				return denied
			}
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return denied
}

// CentOS/systemd 219 clears ExecMainPID/Code after a normal stop. Recover only
// the last same-PID READY->STOPPED receipt, never infer join from inactive.
func (p *child) lastStoppedPID() int {
	ctx, cancel := bounded()
	defer cancel()
	b, err := exec.CommandContext(ctx, "journalctl", "--no-pager", "-o", "json", "-n", "200", "-u", p.unit).Output()
	if err != nil || len(b) > 1<<20 {
		return 0
	}
	return stoppedReceiptPID(b)
}

func stoppedReceiptPID(b []byte) int {
	if len(b) > 1<<20 {
		return 0
	}
	pid, stopped := 0, false
	for _, line := range strings.Split(string(b), "\n") {
		var row map[string]json.RawMessage
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		var message, id string
		if json.Unmarshal(row["MESSAGE"], &message) != nil || json.Unmarshal(row["_PID"], &id) != nil {
			continue
		}
		n, _ := strconv.Atoi(id)
		if message == "READY sole authority; supervised network enabled on private broker" {
			pid = n
			stopped = false
		}
		if message == "STOPPED authority" && n == pid {
			stopped = true
		}
	}
	if stopped && pid > 1 {
		return pid
	}
	return 0
}

func (p *child) removeService() error {
	if !p.joined || !strings.HasPrefix(p.unit, "remoteapp-isolated-network-") {
		return denied
	}
	// No enable, no persistent /etc unit, no other service target deletion.
	serviceCommand("reset-failed", p.unit)
	if os.Remove(filepath.Join("/run/systemd/system", p.unit)) != nil {
		return denied
	}
	_, err := serviceCommand("daemon-reload")
	return err
}

// Explicit recovery of one failed isolated fixture. Never force-stop a live
// worker, remove ownership locks or target production directories/units.
func cleanupFailedService(base, unit string) error {
	if os.Geteuid() != 0 || filepath.Dir(base) != "/run" || !strings.HasPrefix(filepath.Base(base), "remoteapp-daemon-check-") || filepath.Clean(base) != base {
		return denied
	}
	if !strings.HasPrefix(unit, "remoteapp-isolated-network-") || !strings.HasSuffix(unit, ".service") {
		return denied
	}
	token := strings.TrimSuffix(strings.TrimPrefix(unit, "remoteapp-isolated-network-"), ".service")
	b, err := hex.DecodeString(token)
	if err != nil || len(b) != 8 || hex.EncodeToString(b) != token {
		return denied
	}
	real, err := filepath.EvalSymlinks(base)
	if err != nil || real != base {
		return denied
	}
	config, err := serviceCommand("show", unit, "-p", "ExecStart")
	if err != nil || !strings.Contains(string(config), base+"/config/profile.json") || !strings.Contains(string(config), base+"/authority") {
		return denied
	}
	p := &child{unit: unit}
	s, err := p.state()
	if err != nil {
		return denied
	}
	if p.serviceWait(s["ExecMainStatus"] == "0") != nil {
		return denied
	}
	for _, lock := range []string{base + "/authority/writer.lock", base + "/verifier/owner.lock"} {
		if _, err := os.Lstat(lock); !os.IsNotExist(err) {
			return denied
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if verifier.Retire(ctx, base+"/verifier", &headscale.Driver{Run: headscale.LocalRunner()}) != nil {
		return denied
	}
	if p.removeService() != nil || os.RemoveAll(base) != nil {
		return denied
	}
	return check(true, "exact_failed_systemd_fixture_joined_and_retired")
}
