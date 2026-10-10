// Server-local isolated acceptance; never install as an online service.
package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/prerelease"
	"remoteapp.local/selfhost-poc/trustprofile"
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
	cmd                   *exec.Cmd
	lines                 chan string
	done                  chan error
	joined                bool
	readyLabel, stopLabel string
}

func launch(binary string, args []string) (*child, error) {
	return launchCommand(exec.Command(binary, args...))
}
func launchCommand(command *exec.Cmd) (*child, error) {
	p := &child{cmd: command, lines: make(chan string, 8), done: make(chan error, 1), readyLabel: "READY sole authority; supervised network enabled on private broker", stopLabel: "STOPPED authority"}
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
		p.done <- p.cmd.Wait()
	}()
	return p, nil
}
func (p *child) ready() error {
	timer := time.NewTimer(45 * time.Second)
	defer timer.Stop()
	select {
	case line := <-p.lines:
		if line != p.readyLabel {
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
		expected := p.stopLabel
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
func run(binary, frontend string, consent *consentPins) (result error) {
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
	var front *child
	frozen := false
	providerStopped := false
	vd := filepath.Join(base, "verifier")
	verifierPlanned := false
	defer func() {
		if frozen && current != nil && !current.joined {
			if current.cmd.Process.Signal(syscall.SIGCONT) != nil {
				result = denied
				fmt.Println("PRESERVED " + base)
				return
			}
			frozen = false
		}
		if providerStopped {
			if provider("start") != nil {
				result = denied
				fmt.Println("PRESERVED " + base)
				return
			}
		}
		if front != nil && !front.joined {
			if front.stop() != nil {
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
		if consent != nil && consent.Nodes {
			// Only after the sole writer has joined: diagnostics read no private
			// fields and never remove client journals with unretired provisions.
			b, err := os.ReadFile(filepath.Join(base, "authority/state.json"))
			if err != nil || !clientJournalsCleaned(b) {
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
		// Emitted only after actual child joins, durable ticket retirement and
		// scoped directory removal; SSH exit alone is not teardown evidence.
		fmt.Println("STOPPED isolated_coordinator_resources_joined")
	}()
	if os.Chmod(base, 0755) != nil {
		return denied
	}
	material := filepath.Join(base, "material")
	endpoint := "https://isolated.invalid:8444"
	var listen string
	if frontend != "" {
		l, e := net.Listen("tcp4", "127.0.0.1:0")
		if e != nil {
			return denied
		}
		listen = l.Addr().String()
		l.Close()
		endpoint = "https://" + listen
	}
	if prerelease.Create(material, endpoint, "daemon-isolated-check", time.Now()) != nil {
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
	var hc *http.Client
	var deviceRaw, networkRaw []byte
	https := func(route string, raw []byte, code int, label string) error {
		if hc == nil {
			return nil
		}
		r, e := hc.Post(endpoint+route, "application/json", strings.NewReader(string(raw)))
		if e != nil {
			return check(false, label)
		}
		defer r.Body.Close()
		body, e := io.ReadAll(io.LimitReader(r.Body, 8193))
		return check(e == nil && len(body) <= 8192 && r.StatusCode == code, label)
	}
	if frontend != "" {
		fd := filepath.Join(base, "frontend")
		for _, dir := range []string{fd, filepath.Join(fd, "server-private"), filepath.Join(fd, "client-public")} {
			if os.Mkdir(dir, 0700) != nil || os.Chown(dir, uid, gid) != nil {
				return denied
			}
		}
		for _, name := range []string{"server-private/api-cert.pem", "server-private/api-key.pem", "client-public/service-profile.json"} {
			b, e := os.ReadFile(filepath.Join(material, name))
			if e != nil {
				return denied
			}
			path := filepath.Join(fd, name)
			if os.WriteFile(path, b, 0600) != nil || os.Chown(path, uid, gid) != nil {
				return denied
			}
		}
		frontCommand := exec.Command(frontend, fd, a.ReleaseRoot, a.Service, "1", listen, socket, "0", "--private-network")
		// Signal and join the actual frontend PID, not a runuser wrapper whose
		// own TERM exit status can obscure the application's clean STOPPED.
		frontCommand.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}}
		front, e = launchCommand(frontCommand)
		if e != nil {
			return denied
		}
		front.readyLabel = "READY unprivileged frontend; private network enabled"
		front.stopLabel = "STOPPED frontend"
		if front.ready() != nil {
			return check(false, "nonroot_private_frontend_ready")
		}
		fmt.Println("PASS nonroot_private_frontend_ready")
		pb, e := os.ReadFile(profile)
		if e != nil {
			return denied
		}
		p, e := trustprofile.Decode(pb)
		if e != nil {
			return denied
		}
		root, e := hex.DecodeString(a.ReleaseRoot)
		if e != nil {
			return denied
		}
		der, e := trustprofile.Verify(p, root, a.Service, 1, time.Now())
		if e != nil {
			return denied
		}
		ca, e := x509.ParseCertificate(der)
		if e != nil {
			return denied
		}
		pool := x509.NewCertPool()
		pool.AddCert(ca)
		tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool}, Proxy: nil}
		defer tr.CloseIdleConnections()
		hc = &http.Client{Transport: tr, Timeout: 20 * time.Second}
		if consent != nil {
			// The coordinator receives public bootstrap only. It cannot register,
			// approve, enroll or sign on behalf of either OS-owned identity.
			var actions *faultActions
			if consent.Fault {
				actions = &faultActions{
					freeze: func() error {
						ctx, cancel := bounded()
						before, err := d.Run(ctx, "policy", "get")
						cancel()
						if err != nil || policyZero(d) {
							return denied
						}
						if current.cmd.Process.Signal(syscall.SIGSTOP) != nil {
							return denied
						}
						frozen = true
						if waitStopped(current.cmd.Process.Pid) != nil {
							return denied
						}
						r, err := hc.Post(endpoint+"/v1/network", "application/json", strings.NewReader(`{}`))
						if err != nil {
							return denied
						}
						io.Copy(io.Discard, io.LimitReader(r.Body, 8193))
						r.Body.Close()
						ctx, cancel = bounded()
						after, err := d.Run(ctx, "policy", "get")
						cancel()
						if err != nil || r.StatusCode != 503 || !samePolicy(before, after) {
							return denied
						}
						return nil
					},
					recover: func() error {
						if !frozen || current.cmd.Process.Signal(syscall.SIGCONT) != nil {
							return denied
						}
						frozen = false
						if current.stop() != nil {
							return denied
						}
						i, err := info(vd, d)
						if err != nil || i.Key != initial.Key || i.IP != initial.IP {
							return denied
						}
						current, err = launch(binary, args)
						if err != nil || current.ready() != nil {
							return denied
						}
						return nil
					},
				}
			}
			if consentSessionOps(os.Stdin, os.Stdout, *consent, signed.Claims.Issuer, der, listen, consent.timeout(), actions, initial.Key) != nil {
				return denied
			}
			if consent.Nodes {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err := awaitClientCleanup(ctx, func() ([]byte, error) { return os.ReadFile(filepath.Join(state, "state.json")) })
				cancel()
				if check(err == nil, "client_journals_durably_cleaned_before_daemon_stop") != nil {
					return denied
				}
			}
			if front.stop() != nil || current.stop() != nil {
				return denied
			}
			return check(policyZero(d), "split_os_consent_stopped_joined_zero_acl")
		}
		_, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return denied
		}
		nonce := make([]byte, 16)
		if _, e = rand.Read(nonce); e != nil {
			return denied
		}
		deviceRaw, e = json.Marshal(invite.Sign(key, invite.Body{Action: "register", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()}))
		if e != nil {
			return denied
		}
		if https("/v1/request", deviceRaw, 200, "signed_device_https_registration_via_nonroot_frontend") != nil {
			return denied
		}
		networkRaw, e = json.Marshal(invite.SignNetworkRequest(key, invite.NetworkRequestBody{Action: "state", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()}))
		if e != nil {
			return denied
		}
		if https("/v1/network", networkRaw, 403, "signed_network_without_grant_denied_via_real_supervisor") != nil {
			return denied
		}
		if https("/v1/request", deviceRaw, 403, "signed_device_replay_denied") != nil {
			return denied
		}
		for _, raw := range []string{`{"body":{"action":"state","Action":"bind"}}`, `{} {}`, strings.Repeat("x", 8193)} {
			if https("/v1/network", []byte(raw), 400, "private_network_strict_json_or_body_limit") != nil {
				return denied
			}
		}
		if https("/health", []byte("{}"), 404, "private_frontend_unknown_route_not_exposed") != nil {
			return denied
		}
		bad := tr.Clone()
		bad.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: x509.NewCertPool()}
		defer bad.CloseIdleConnections()
		r, e := (&http.Client{Transport: bad, Timeout: 5 * time.Second}).Post(endpoint+"/v1/network", "application/json", strings.NewReader(string(networkRaw)))
		if r != nil {
			r.Body.Close()
		}
		if check(e != nil, "wrong_api_ca_refuses_tls_connection") != nil {
			return denied
		}
		before, e = os.ReadFile(filepath.Join(state, "state.json"))
		if e != nil {
			return denied
		}
	}
	if current.stop() != nil {
		return denied
	}
	if check(policyZero(d), "normal_daemon_stop_zero_acl_confirmed") != nil {
		return denied
	}
	if https("/v1/network", networkRaw, 503, "authority_stopped_private_https_network_503") != nil {
		return denied
	}
	if https("/v1/request", deviceRaw, 503, "authority_stopped_private_https_device_503") != nil {
		return denied
	}
	i, e := info(vd, d)
	if check(e == nil && i.Key == initial.Key && i.IP == initial.IP, "daemon_stop_retains_same_verifier_key_and_ip") != nil {
		return denied
	}
	if start() != nil || probe() != nil {
		return denied
	}
	if https("/v1/request", deviceRaw, 403, "authority_restart_retains_https_device_replay") != nil {
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
	if https("/v1/network", networkRaw, 503, "real_provider_fault_private_https_network_503") != nil {
		return denied
	}
	if provider("start") != nil {
		return denied
	}
	providerStopped = false
	if _, e := os.Lstat(socket); check(os.IsNotExist(e), "provider_recovery_does_not_restart_daemon_or_broker") != nil {
		return denied
	}
	if https("/v1/network", networkRaw, 503, "provider_recovery_without_daemon_still_https_503") != nil {
		return denied
	}
	i, e = info(vd, d)
	if check(e == nil && i.Key == initial.Key && i.IP == initial.IP, "faulted_daemon_retains_same_verifier_identity") != nil {
		return denied
	}
	if start() != nil || probe() != nil {
		return denied
	}
	if https("/v1/network", networkRaw, 403, "explicit_daemon_restart_private_https_reconnects") != nil {
		return denied
	}
	if https("/v1/request", deviceRaw, 403, "fault_restart_retains_https_device_replay") != nil {
		return denied
	}
	if current.stop() != nil {
		return denied
	}
	if front != nil {
		if check(front.stop() == nil, "nonroot_private_frontend_zero_exit_and_join") != nil {
			return denied
		}
		fmt.Println("PASS nonroot_private_frontend_stopped_and_joined")
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
		e = run(os.Args[1], "", nil)
	} else if len(os.Args) == 4 && os.Args[2] == "--frontend" {
		e = run(os.Args[1], os.Args[3], nil)
	} else if len(os.Args) == 7 && os.Args[2] == "--frontend" && (os.Args[4] == "--os-consent" || os.Args[4] == "--os-nodes" || os.Args[4] == "--os-tls-fault" || os.Args[4] == "--os-compiled" || os.Args[4] == "--os-compiled-fault") {
		pins, err := parseConsentPins(os.Args[5], os.Args[6])
		if err != nil {
			e = denied
		} else {
			pins.Fault = os.Args[4] == "--os-tls-fault" || os.Args[4] == "--os-compiled-fault"
			pins.Compiled = os.Args[4] == "--os-compiled" || os.Args[4] == "--os-compiled-fault"
			pins.Nodes = pins.Fault || pins.Compiled || os.Args[4] == "--os-nodes"
			e = run(os.Args[1], os.Args[3], &pins)
		}
	} else {
		e = denied
	}
	if e != nil {
		fmt.Println("FAIL isolated_daemon_network_acceptance")
		os.Exit(1)
	}
}
