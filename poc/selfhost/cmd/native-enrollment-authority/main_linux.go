// Root-local isolated acceptance authority. Never replaces the formal Store.
// Long-lived verifier is REOPENED only after approved maintenance joins owners.
// TLS key file is server-private and is never returned through public metadata.
//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/verifier"
)

const base = "/var/lib/remoteapp-native-acceptance"

var denied = invite.ErrDenied

func checkedStage(stage string) bool {
	if os.Geteuid() != 0 || filepath.Dir(stage) != base || !strings.HasPrefix(filepath.Base(stage), "remoteapp-native-") {
		return false
	}
	id := strings.TrimPrefix(filepath.Base(stage), "remoteapp-native-")
	b, e := hex.DecodeString(id)
	if e != nil || len(b) != 8 || hex.EncodeToString(b) != id {
		return false
	}
	real, e := filepath.EvalSymlinks(stage)
	st, se := os.Lstat(stage)
	if e != nil || real != stage || se != nil || !st.IsDir() || st.Mode().Perm() != 0700 {
		return false
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	parent, pe := os.Lstat(base)
	if pe != nil || !parent.IsDir() || parent.Mode().Perm() != 0700 {
		return false
	}
	ps, po := parent.Sys().(*syscall.Stat_t)
	return ok && sys.Uid == 0 && po && ps.Uid == 0
}

func maintenancePaused(stage string) bool {
	b, e := os.ReadFile(filepath.Join(stage, "maintenance.json"))
	if e != nil || len(b) > 8192 {
		return false
	}
	var c struct {
		Phase     string `json:"phase"`
		Candidate string `json:"candidate"`
		Store     string `json:"store_sha"`
		Verifier  string `json:"verifier"`
	}
	if json.Unmarshal(b, &c) != nil || c.Phase != "paused" || c.Store != "5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca" {
		return false
	}
	exe, e := os.Executable()
	if e != nil || exe != filepath.Join(stage, "authority-candidate") {
		return false
	}
	raw, e := os.ReadFile(exe)
	if e != nil {
		return false
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]) == c.Candidate
}
func writePublic(stage, name string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return denied
	}
	f, e := os.OpenFile(filepath.Join(stage, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return denied
	}
	_, e = f.Write(b)
	syncErr := f.Sync()
	closeErr := f.Close()
	if e != nil || syncErr != nil || closeErr != nil {
		return denied
	}
	d, e := os.Open(stage)
	if e != nil {
		return denied
	}
	defer d.Close()
	return d.Sync()
}
func prepare(stage string) error {
	if !checkedStage(stage) {
		return denied
	}
	state := filepath.Join(stage, "authority")
	if _, e := os.Lstat(state); !os.IsNotExist(e) {
		return denied
	}
	s, e := invite.Open(state, time.Now)
	if e != nil {
		return denied
	}
	pub := hex.EncodeToString(s.PublicKey())
	if s.Close() != nil {
		return denied
	}
	return writePublic(stage, "public-bootstrap.json", struct {
		Version int
		Issuer  string
	}{1, pub})
}
func privateTLS(stage string) (tls.Certificate, error) {
	path := filepath.Join(stage, "authority-tls.json")
	st, e := os.Lstat(path)
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Size() > 32768 {
		return tls.Certificate{}, denied
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok || sys.Uid != 0 || sys.Nlink != 1 {
		return tls.Certificate{}, denied
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return tls.Certificate{}, denied
	}
	var fixture struct{ Certificate, Key string }
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if d.Decode(&fixture) != nil {
		return tls.Certificate{}, denied
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return tls.Certificate{}, denied
	}
	return tls.X509KeyPair([]byte(fixture.Certificate), []byte(fixture.Key))
}
func noFormalLocks() bool {
	for _, p := range []string{"/var/lib/remoteapp-authority/writer.lock", "/var/lib/remoteapp-verifier/owner.lock"} {
		if _, e := os.Lstat(p); !os.IsNotExist(e) {
			return false
		}
	}
	return true
}

// Explicit post-serve retirement only. Never touches the persistent verifier
// record, which is not part of this isolated Store's provision journal.
func retire(stage string) error {
	if !checkedStage(stage) || !noFormalLocks() {
		return denied
	}
	b, e := os.ReadFile(filepath.Join(stage, "candidate-joined.json"))
	if e != nil {
		return denied
	}
	var receipt struct{ Version, PID int }
	if json.Unmarshal(b, &receipt) != nil || receipt.Version != 1 || receipt.PID <= 1 {
		return denied
	}
	if e = syscall.Kill(receipt.PID, 0); e != syscall.ESRCH {
		return denied
	}
	if _, e = os.Lstat(filepath.Join(stage, "authority", "writer.lock")); !os.IsNotExist(e) {
		return denied
	}
	s, e := invite.Open(filepath.Join(stage, "authority"), time.Now)
	if e != nil {
		return denied
	}
	defer s.Close()
	d := &headscale.Driver{Run: headscale.LocalRunner()}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, r := range s.ProvisionRecords() {
		if r.Status == "cleaned" {
			continue
		}
		if s.ProvisionCleanup(r.ID, false) != nil || d.Cleanup(ctx, r) != nil || s.ProvisionCleanup(r.ID, true) != nil {
			return denied
		}
	}
	return nil
}
func serve(stage string) (result error) {
	if !checkedStage(stage) || !maintenancePaused(stage) || !noFormalLocks() {
		return denied
	}
	if _, e := os.Lstat(filepath.Join(stage, "public-ready.json")); !os.IsNotExist(e) {
		return denied
	}
	// Candidate cannot initialize a missing prepared Store.
	state := filepath.Join(stage, "authority")
	if st, e := os.Lstat(filepath.Join(state, "state.json")); e != nil || !st.Mode().IsRegular() {
		return denied
	}
	pair, e := privateTLS(stage)
	if e != nil {
		return denied
	}
	s, e := invite.Open(state, time.Now)
	if e != nil {
		return denied
	}
	defer func() {
		if s.Close() != nil {
			result = denied
		}
	}()
	run := headscale.LocalRunner()
	r, e := verifier.Open(context.Background(), "/var/lib/remoteapp-verifier", false, &headscale.Driver{Run: run}, verifier.NewOverlay)
	if e != nil {
		return denied
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if r.Stop(ctx) != nil {
			result = denied
		}
	}()
	policy := filepath.Join(stage, "policy")
	if os.Mkdir(policy, 0700) != nil {
		return denied
	}
	driver := &headscale.Driver{Run: run, PolicyDir: policy, PolicyMode: "database", VerifierIP: r.Info().IP}
	driver.Proof = func(ctx context.Context, rec invite.ProvisionRecord, o invite.ProvisionObservation) bool {
		pub, e := s.DevicePublicKey(rec.Device)
		return e == nil && r.VerifyWithReport(ctx, rec, o, pub, func(label string) { fmt.Println("PROOF_STAGE " + label) })
	}
	backend := &invite.ProvisionBackend{Store: s, Driver: driver, VerifierSource: func(ctx context.Context) (string, error) {
		i := r.Info()
		if ctx.Err() != nil || !i.Healthy || !r.Live() {
			return "", denied
		}
		return i.Key, nil
	}}
	super, e := broker.StartNetworkSupervisor(context.Background(), backend, r, 200*time.Millisecond)
	if e != nil {
		return denied
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if super.Stop(ctx) != nil || !super.DenyAllConfirmed() {
			result = denied
		}
	}()
	brokerCtx, brokerCancel := context.WithCancel(context.Background())
	brokerDone := make(chan error, 1)
	brokerReady := make(chan struct{}, 1)
	socket := filepath.Join(stage, "broker.sock")
	go func() {
		brokerDone <- broker.Serve(brokerCtx, broker.Config{Socket: socket, PeerUID: 0, GroupID: 0, Executor: super.Authority()}, func() { brokerReady <- struct{}{} })
	}()
	select {
	case <-brokerReady:
	case <-brokerDone:
		brokerCancel()
		return denied
	}
	defer func() {
		brokerCancel()
		if <-brokerDone != nil {
			result = denied
		}
	}()
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return denied
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	server := &http.Server{Handler: broker.SignedNetworkHandler(broker.Client{Socket: socket, ServerUID: 0}, hex.EncodeToString(s.PublicKey())), ReadHeaderTimeout: time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 20 * time.Second, ErrorLog: log.New(io.Discard, "", 0), BaseContext: func(net.Listener) context.Context { return ctx }}
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}}))
	}()
	if writePublic(stage, "public-ready.json", struct {
		Version, Port int
		Issuer        string
	}{1, listener.Addr().(*net.TCPAddr).Port, hex.EncodeToString(s.PublicKey())}) != nil {
		server.Close()
		<-done
		return denied
	}
	fmt.Println("READY isolated_native_authority; loopback TLS and reopened verifier only")
	var serveErr error
	finished := false
	select {
	case <-ctx.Done():
	case <-super.Done():
		result = denied
	case serveErr = <-done:
		finished = true
		result = denied
	}
	stop, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()
	if server.Shutdown(stop) != nil {
		server.Close()
		result = denied
	}
	if !finished {
		serveErr = <-done
	}
	if serveErr != http.ErrServerClosed {
		result = denied
	}
	return result
}
func main() {
	if len(os.Args) != 3 {
		fmt.Println("FAIL isolated_authority_arguments")
		os.Exit(1)
	}
	var e error
	switch os.Args[1] {
	case "prepare":
		e = prepare(os.Args[2])
	case "serve":
		e = serve(os.Args[2])
	case "retire":
		e = retire(os.Args[2])
	default:
		e = denied
	}
	if e != nil {
		fmt.Println("FAIL isolated_native_authority; retain scoped evidence")
		os.Exit(1)
	}
	if os.Args[1] == "prepare" {
		fmt.Println("PREPARED isolated_authority_public_issuer_only")
	} else if os.Args[1] == "serve" {
		if writePublic(os.Args[2], "candidate-joined.json", struct{ Version, PID int }{1, os.Getpid()}) != nil {
			fmt.Println("FAIL candidate_join_receipt")
			os.Exit(1)
		}
		fmt.Println("STOPPED isolated_native_authority; requests, policy and verifier joined")
	} else {
		fmt.Println("RETIRED isolated_store_tickets_only; existing_verifier_retained")
	}
}
