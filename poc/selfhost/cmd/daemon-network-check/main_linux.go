// Server-local isolated acceptance; never install as an online service.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/prerelease"
	"remoteapp.local/selfhost-poc/verifier"
)

var denied = errors.New("isolated_acceptance_failed")

func check(ok bool, label string) error {
	if !ok {
		fmt.Println("FAIL " + label)
		return denied
	}
	fmt.Println("PASS " + label)
	return nil
}
func bounded() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 40*time.Second)
}

type child struct {
	cmd    *exec.Cmd
	lines  chan string
	done   chan error
	joined bool
}

func launch(binary string, args []string) (*child, error) {
	p := &child{cmd: exec.Command(binary, args...), lines: make(chan string, 8), done: make(chan error, 1)}
	out, e := p.cmd.StdoutPipe()
	if e != nil {
		return nil, denied
	}
	p.cmd.Stderr = io.Discard
	if p.cmd.Start() != nil {
		return nil, denied
	}
	go func() {
		s := bufio.NewScanner(out)
		s.Buffer(make([]byte, 1024), 1024)
		for s.Scan() {
			p.lines <- s.Text()
		}
		close(p.lines)
	}()
	go func() { p.done <- p.cmd.Wait() }()
	return p, nil
}
func (p *child) ready() error {
	timer := time.NewTimer(45 * time.Second)
	defer timer.Stop()
	select {
	case line := <-p.lines:
		if line != "READY sole authority; supervised network enabled on private broker" {
			return denied
		}
		return nil
	case <-timer.C:
		return denied
	}
}
func (p *child) wait(normal bool) error {
	timer := time.NewTimer(40 * time.Second)
	defer timer.Stop()
	select {
	case e := <-p.done:
		p.joined = true
		if (e == nil) != normal {
			return denied
		}
		expected := "STOPPED authority"
		if !normal {
			expected = "FAIL authority unavailable"
		}
		select {
		case line := <-p.lines:
			if line != expected {
				return denied
			}
		case <-timer.C:
			return denied
		}
		return nil
	case <-timer.C:
		return denied
	}
}
func (p *child) stop() error {
	if p == nil || p.joined {
		return nil
	}
	if p.cmd.Process.Signal(syscall.SIGTERM) != nil {
		return denied
	}
	return p.wait(true)
}
func provider(action string) error {
	ctx, cancel := bounded()
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", action, "remoteapp-poc")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if cmd.Run() != nil {
		return denied
	}
	return nil
}
func policyZero(d *headscale.Driver) bool {
	ctx, cancel := bounded()
	defer cancel()
	b, e := d.Run(ctx, "policy", "get")
	var p map[string]json.RawMessage
	var a []json.RawMessage
	return e == nil && json.Unmarshal(b, &p) == nil && len(p) == 1 && json.Unmarshal(p["acls"], &a) == nil && a != nil && len(a) == 0
}
func info(dir string, d *headscale.Driver) (verifier.Info, error) {
	ctx, cancel := bounded()
	defer cancel()
	r, e := verifier.Open(ctx, dir, false, d, verifier.NewOverlay)
	if e != nil {
		return verifier.Info{}, denied
	}
	i := r.Info()
	return i, r.Stop(ctx)
}
func peer(socket string, paths []string) error {
	if os.Geteuid() == 0 || len(paths) != 4 {
		return denied
	}
	ctx, cancel := bounded()
	defer cancel()
	c := broker.Client{Socket: socket, ServerUID: 0}
	r, e := c.Call(ctx, broker.Request{Operation: "identity"})
	if e != nil || r.Status != 200 || len(r.Issuer) != 64 || r.Issuer != paths[0] {
		return denied
	}
	// No registered device/grant: a live backend must reject, not mint.
	r, e = c.Call(ctx, broker.Request{Operation: "network", Network: &invite.NetworkRequest{}})
	if e != nil || r.Status != 403 || r.Network != nil {
		return denied
	}
	for _, p := range paths[1:] {
		if _, e := os.ReadFile(p); !os.IsPermission(e) {
			return denied
		}
	}
	fmt.Println("PASS authorized_nonroot_peer_identity_and_network_denial_private_reads_denied")
	return nil
}
func run(binary string) (result error) {
	if os.Geteuid() != 0 || !filepath.IsAbs(binary) {
		return denied
	}
	u, e := user.Lookup("nobody")
	if e != nil {
		return denied
	}
	uid, e := strconv.Atoi(u.Uid)
	if e != nil || uid == 0 {
		return denied
	}
	gid, e := strconv.Atoi(u.Gid)
	if e != nil {
		return denied
	}
	// A non-root peer must be able to traverse the experiment's public parent.
	// Never loosen the existing PoC service's private state directory.
	base, e := os.MkdirTemp("/tmp", "remoteapp-daemon-check-")
	if e != nil {
		return denied
	}
	d := &headscale.Driver{Run: headscale.LocalRunner()}
	var current *child
	providerStopped := false
	vd := filepath.Join(base, "verifier")
	verifierPlanned := false
	defer func() {
		if providerStopped {
			if provider("start") != nil {
				result = denied
				fmt.Println("PRESERVED " + base)
				return
			}
		}
		if current != nil && !current.joined {
			if current.stop() != nil {
				result = denied
				fmt.Println("PRESERVED " + base)
				return
			}
		}
		if verifierPlanned {
			ctx, cancel := bounded()
			e := verifier.Retire(ctx, vd, d)
			cancel()
			if e != nil {
				result = denied
				fmt.Println("PRESERVED " + base)
				return
			}
			fmt.Println("PASS exact_verifier_ticket_retired")
		}
		if os.RemoveAll(base) != nil {
			result = denied
			return
		}
		fmt.Println("PASS isolated_experiment_directory_removed")
	}()
	if os.Chmod(base, 0755) != nil {
		return denied
	}
	material := filepath.Join(base, "material")
	if prerelease.Create(material, "https://isolated.invalid:8444", "daemon-isolated-check", time.Now()) != nil {
		return denied
	}
	var a prerelease.Anchor
	b, e := os.ReadFile(filepath.Join(material, "client-public/installation-anchor.json"))
	if e != nil || json.Unmarshal(b, &a) != nil {
		return denied
	}
	state := filepath.Join(base, "authority")
	if os.Rename(filepath.Join(material, "server-private/authority"), state) != nil {
		return denied
	}
	config := filepath.Join(base, "config")
	pd := filepath.Join(base, "policy")
	sd := filepath.Join(base, "socket")
	for _, dir := range []string{config, pd} {
		if os.Mkdir(dir, 0700) != nil {
			return denied
		}
	}
	if os.Mkdir(sd, 0710) != nil || os.Chown(sd, 0, gid) != nil {
		return denied
	}
	profile := filepath.Join(config, "profile.json")
	b, e = os.ReadFile(filepath.Join(material, "client-public/service-profile.json"))
	if e != nil || os.WriteFile(profile, b, 0600) != nil {
		return denied
	}
	var signed struct {
		Claims struct {
			Issuer string `json:"issuer"`
		} `json:"claims"`
	}
	if json.Unmarshal(b, &signed) != nil || len(signed.Claims.Issuer) != 64 {
		return denied
	}
	n := broker.DaemonNetworkConfig{Version: 1, VerifierDirectory: vd, PolicyDirectory: pd}
	b, e = json.Marshal(n)
	np := filepath.Join(config, "network.json")
	if e != nil || os.WriteFile(np, b, 0600) != nil {
		return denied
	}
	verifierPlanned = true
	ctx, cancel := bounded()
	v, e := verifier.Open(ctx, vd, true, d, verifier.NewOverlay)
	if e != nil {
		cancel()
		return denied
	}
	initial := v.Info()
	e = v.Stop(ctx)
	cancel()
	if e != nil {
		return denied
	}
	if check(initial.Healthy, "real_verifier_prepared_then_stopped_identity_retained") != nil {
		return denied
	}
	before, e := os.ReadFile(filepath.Join(state, "state.json"))
	if e != nil {
		return denied
	}
	socket := filepath.Join(sd, "authority.sock")
	args := []string{profile, a.ReleaseRoot, a.Service, "1", state, socket, strconv.Itoa(uid), strconv.Itoa(gid), "--network-config", np}
	start := func() error {
		var e error
		current, e = launch(binary, args)
		if e != nil {
			return e
		}
		if current.ready() != nil {
			return denied
		}
		return check(policyZero(d), "real_daemon_ready_projects_explicit_zero_acl")
	}
	probe := func() error {
		self, e := os.Executable()
		if e != nil {
			return denied
		}
		ctx, cancel := bounded()
		defer cancel()
		cmd := exec.CommandContext(ctx, "runuser", "-u", "nobody", "--", self, "peer", socket, signed.Claims.Issuer, filepath.Join(state, "state.json"), profile, filepath.Join(vd, "node/tailscaled.state"))
		out, e := cmd.Output()
		if e != nil || string(out) != "PASS authorized_nonroot_peer_identity_and_network_denial_private_reads_denied\n" {
			fmt.Println("FAIL authorized_nonroot_peer_probe")
			return denied
		}
		fmt.Print(string(out))
		_, e = (broker.Client{Socket: socket, ServerUID: 0}).Call(ctx, broker.Request{Operation: "identity"})
		return check(e != nil, "wrong_root_peer_uid_rejected_by_kernel_broker")
	}
	if start() != nil || probe() != nil {
		return denied
	}
	if current.stop() != nil {
		return denied
	}
	if check(policyZero(d), "normal_daemon_stop_zero_acl_confirmed") != nil {
		return denied
	}
	i, e := info(vd, d)
	if check(e == nil && i.Key == initial.Key && i.IP == initial.IP, "daemon_stop_retains_same_verifier_key_and_ip") != nil {
		return denied
	}
	if start() != nil || probe() != nil {
		return denied
	}
	if provider("stop") != nil {
		return denied
	}
	providerStopped = true
	if current.wait(false) != nil {
		return denied
	}
	if check(true, "real_provider_outage_faults_and_joins_daemon_nonzero") != nil {
		return denied
	}
	if provider("start") != nil {
		return denied
	}
	providerStopped = false
	if _, e := os.Lstat(socket); check(os.IsNotExist(e), "provider_recovery_does_not_restart_daemon_or_broker") != nil {
		return denied
	}
	i, e = info(vd, d)
	if check(e == nil && i.Key == initial.Key && i.IP == initial.IP, "faulted_daemon_retains_same_verifier_identity") != nil {
		return denied
	}
	if start() != nil || probe() != nil {
		return denied
	}
	if current.stop() != nil {
		return denied
	}
	after, e := os.ReadFile(filepath.Join(state, "state.json"))
	if check(e == nil && string(before) == string(after), "explicit_restart_no_new_issuer_or_device_state") != nil {
		return denied
	}
	return check(policyZero(d), "final_daemon_normal_stop_zero_acl")
}
func main() {
	var e error
	if len(os.Args) >= 3 && os.Args[1] == "peer" {
		e = peer(os.Args[2], os.Args[3:])
	} else if len(os.Args) == 2 {
		e = run(os.Args[1])
	} else {
		e = denied
	}
	if e != nil {
		fmt.Println("FAIL isolated_daemon_network_acceptance")
		os.Exit(1)
	}
}
