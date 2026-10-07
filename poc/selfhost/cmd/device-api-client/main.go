// Private device client fixture: no keys in argv or logs. HTTP requests are
// signed here, not by a privileged parent. Bootstrap trust is still private.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"time"
)

func main() {
	if run() != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "failed"})
		os.Exit(1)
	}
}
func run() error {
	read := privatepipe.NewReader(os.Stdin).Read
	var cfg struct {
		Endpoint, Certificate, AppKey               string
		Issuer, Target, Controller                  string
		Grant                                       invite.Grant
		VaultExecutable, VaultScope, ExpectedPublic string
		CreateIdentity                              bool
	}
	if read(&cfg) != nil {
		return invite.ErrDenied
	}
	cert, e := base64.StdEncoding.DecodeString(cfg.Certificate)
	if e != nil {
		return invite.ErrDenied
	}
	var client *invite.NetworkClient
	if cfg.VaultExecutable != "" {
		if cfg.AppKey != "" {
			return invite.ErrDenied
		}
		signer, err := privatepipe.NewSigner(cfg.VaultExecutable, cfg.VaultScope, cfg.ExpectedPublic, cfg.CreateIdentity)
		if err != nil {
			return err
		}
		defer signer.Close()
		client, e = invite.NewNetworkClientWithSigner(cfg.Endpoint, cert, signer)
	} else {
		key, err := hex.DecodeString(cfg.AppKey)
		if err != nil {
			return invite.ErrDenied
		}
		client, e = invite.NewNetworkClient(cfg.Endpoint, cert, ed25519.PrivateKey(key))
	}
	if e != nil {
		return e
	}
	defer client.Close()
	clock := time.Now
	var watch *invite.StateWatch
	defer func() {
		if watch != nil {
			watch.Stop()
		}
	}()
	out := json.NewEncoder(os.Stdout)
	out.Encode(map[string]any{"status": "ready"})
	for {
		var request struct {
			Op, Ticket, Challenge         string
			Intent                        invite.ProvisionEnvelope
			Device                        invite.Body
			Grant                         invite.Grant
			Certificate, CertificateProof string
			Binding                       invite.NetworkBinding
		}
		if e := read(&request); e == io.EOF {
			return nil
		} else if e != nil {
			return e
		}
		if request.Op == "stop" {
			if watch != nil {
				watch.Stop()
				watch = nil
			}
			out.Encode(map[string]any{"status": "closed"})
			return nil
		}
		if request.Op == "watch_start" {
			passed := false
			if watch == nil {
				issuer, err := hex.DecodeString(cfg.Issuer)
				if err == nil {
					watch, err = client.StartStateWatch(context.Background(), issuer, cfg.Grant, request.Binding, clock)
					passed = err == nil
					if passed {
						end := time.Now().Add(2 * time.Second)
						for {
							if _, e := watch.Snapshot(); e == nil {
								break
							}
							if time.Now().After(end) {
								watch.Stop()
								watch = nil
								passed = false
								break
							}
							time.Sleep(10 * time.Millisecond)
						}
					}
				}
			}
			out.Encode(map[string]any{"status": "watch_start", "passed": passed})
			continue
		}
		if request.Op == "watch_state" {
			value := map[string]any{"status": "watch_state", "passed": false}
			if watch != nil {
				n, err := watch.Snapshot()
				if err == nil {
					value["passed"] = true
					value["network_state"] = n
				}
			}
			out.Encode(value)
			continue
		}
		if request.Op == "watch_stop" {
			passed := watch != nil
			if watch != nil {
				watch.Stop()
				watch = nil
			}
			out.Encode(map[string]any{"status": "watch_stop", "passed": passed})
			continue
		}
		if request.Op == "use_grant" {
			issuer, err := hex.DecodeString(cfg.Issuer)
			var nonce [32]byte
			_, randomErr := rand.Read(nonce[:])
			challenge := hex.EncodeToString(nonce[:])
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			proof, proofErr := client.Request(ctx, invite.NetworkRequestBody{Action: "grant_clock", Grant: request.Grant, Challenge: challenge})
			cancel()
			passed := false
			if err == nil && randomErr == nil && proofErr == nil && proof.Proof != nil && cfg.Grant == (invite.Grant{}) {
				calibrated, clockErr := invite.AuthorityClock(issuer, *proof.Proof, request.Grant.Claims.ID, challenge, start)
				passed = clockErr == nil && invite.VerifyGrant(issuer, request.Grant, cfg.Target, cfg.Controller, calibrated())
				if passed {
					clock = calibrated
				}
			}
			if passed {
				cfg.Grant = request.Grant
			}
			out.Encode(map[string]any{"status": "use_grant", "passed": passed})
			continue
		}
		if request.Op == "device" {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			result, e := client.DeviceRequest(ctx, request.Device)
			cancel()
			value := map[string]any{"status": "device", "passed": e == nil}
			if e == nil {
				value["device"] = result
			}
			if out.Encode(value) != nil {
				return io.ErrClosedPipe
			}
			continue
		}
		action := request.Op
		if action == "network_state" {
			action = "state"
		}
		if action == "clock_proof" {
			action = "clock"
		}
		if action == "tls_identity" {
			if request.Challenge != "" {
				return invite.ErrDenied
			}
			var nonce [32]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				return err
			}
			request.Challenge = hex.EncodeToString(nonce[:])
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		reply, e := client.Request(ctx, invite.NetworkRequestBody{Action: action, Grant: cfg.Grant, Ticket: request.Ticket, Challenge: request.Challenge, Intent: request.Intent, Certificate: request.Certificate, CertificateProof: request.CertificateProof})
		cancel()
		if e == nil && reply.TLS != nil {
			issuer, err := hex.DecodeString(cfg.Issuer)
			if err != nil || !invite.VerifyTLSIdentity(issuer, *reply.TLS, cfg.Grant, request.Binding, request.Challenge, clock()) {
				e = invite.ErrDenied
			}
		}
		value := map[string]any{"status": request.Op, "passed": e == nil}
		if e == nil {
			if reply.TLS != nil {
				value["tls_identity"] = reply.TLS
			}
			if reply.Ticket != "" {
				value["ticket"] = reply.Ticket
				value["credential"] = reply.Credential
			}
			if reply.State != nil {
				value["network_state"] = reply.State
			}
			if reply.Proof != nil {
				value["proof"] = reply.Proof
			}
		}
		if out.Encode(value) != nil {
			return io.ErrClosedPipe
		}
	}
}
