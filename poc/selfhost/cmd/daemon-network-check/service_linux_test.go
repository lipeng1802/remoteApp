package main

import (
	"os"
	"strings"
	"testing"
)

func TestCandidateRenderingKeepsSandboxAndSeparatesFixture(t *testing.T) {
	path := os.Getenv("REMOTEAPP_CANDIDATE_TEST")
	if path == "" {
		path = "../../public-server/remoteapp-authority-network.service.candidate"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	base := "/run/remoteapp-daemon-check-fixture"
	args := []string{base + "/config/profile.json", strings.Repeat("a", 64), "daemon-isolated-check", "1", base + "/authority", base + "/socket/authority.sock", "99", "99", "--network-config", base + "/config/network.json"}
	unit, err := renderCandidate(string(b), base+"-daemon", base, args)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"PrivateTmp=true\n", "NoNewPrivileges=true\n", "Restart=no\n", "TimeoutStopSec=45\n", "Wants=remoteapp-poc.service\n", "--check-network-config " + args[9], "--network-config " + args[9]} {
		if !strings.Contains(unit, s) {
			t.Fatal("lost candidate constraint")
		}
	}
	for _, s := range []string{"@", "/var/lib/remoteapp-authority", "/etc/remoteapp-authority", "/opt/remoteapp-prerelease/authority-daemon", "Requires="} {
		if strings.Contains(unit, s) {
			t.Fatal("formal path/dependency remained")
		}
	}
	for _, bad := range []string{strings.Replace(string(b), "Restart=no", "Restart=always", 1), strings.Replace(string(b), "Wants=", "Requires=", 1), strings.Replace(string(b), "TimeoutStopSec=45", "TimeoutStopSec=10", 1)} {
		if _, err := renderCandidate(bad, base+"-daemon", base, args); err == nil {
			t.Fatal("unsafe unit accepted")
		}
	}
	args[1] += "\nExecStart=/bin/false"
	if _, err := renderCandidate(string(b), base+"-daemon", base, args); err == nil {
		t.Fatal("injected directive accepted")
	}
}

func TestFailedServiceRecoveryRejectsNonFixtureTargets(t *testing.T) {
	for _, base := range []string{"/var/lib/remoteapp-authority", "/run/remoteapp-daemon-check-x/..", "relative", "/run/other"} {
		if cleanupFailedService(base, "remoteapp-isolated-network-1234567890abcdef.service") == nil {
			t.Fatal("unsafe recovery target accepted")
		}
	}
	if cleanupFailedService("/run/remoteapp-daemon-check-nonexistent", "remoteapp-authority.service") == nil {
		t.Fatal("formal unit recovery accepted")
	}
}

func TestStoppedReceiptRequiresLatestSamePIDReadyThenStop(t *testing.T) {
	ready := `{"MESSAGE":"READY sole authority; supervised network enabled on private broker","_PID":"12345"}` + "\n"
	stop := `{"MESSAGE":"STOPPED authority","_PID":"12345"}` + "\n"
	for _, c := range []struct {
		raw  string
		want int
	}{
		{ready + stop, 12345}, {ready, 0}, {stop, 0}, {stop + ready, 0},
		{ready + strings.Replace(stop, "12345", "12346", 1), 0},
		{ready + stop + strings.Replace(ready, "12345", "12346", 1), 0},
		{ready + `{"MESSAGE":"systemctl inactive","_PID":"12345"}`, 0},
	} {
		if stoppedReceiptPID([]byte(c.raw)) != c.want {
			t.Fatal("invalid normal-stop receipt accepted")
		}
	}
}
