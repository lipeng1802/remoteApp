package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"remoteapp.local/selfhost-poc/clientruntime"
	"strings"
	"testing"
)

type domainProbeSigner struct {
	key   ed25519.PrivateKey
	stale bool
	calls int
}

func (s *domainProbeSigner) PublicKey() ed25519.PublicKey { return s.key.Public().(ed25519.PublicKey) }
func (s *domainProbeSigner) Sign(raw []byte) ([]byte, error) {
	s.calls++
	if s.stale && strings.HasPrefix(string(raw), "remoteapp-provision-intent-v1\n") {
		return nil, errors.New("stale helper")
	}
	return ed25519.Sign(s.key, raw), nil
}
func TestProbeRejectsStaleSignerBeforeRemoteEnrollment(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	s := &domainProbeSigner{key: key}
	if probeSigningDomains(s) != nil || s.calls != 8 {
		t.Fatal("complete signing contract not probed")
	}
	s = &domainProbeSigner{key: key, stale: true}
	if probeSigningDomains(s) == nil || s.calls != 3 {
		t.Fatal("stale native helper accepted")
	}
}

func TestDesktopStartRejectsRoleAndIdentityBeforeResources(t *testing.T) {
	b := &backend{expected: "desktop-identity"}
	for _, c := range []clientruntime.Config{{}, {Server: true, Target: "other"}, {Server: false, Target: "desktop-identity"}} {
		if b.Start(context.Background(), c, func(any) error { t.Fatal("unexpected readiness"); return nil }) == nil {
			t.Fatal("unbound role accepted")
		}
	}
}
func TestDesktopEntryRejectsUncompiledRelease(t *testing.T) {
	if run() == nil {
		t.Fatal("uncompiled desktop entry accepted")
	}
}
