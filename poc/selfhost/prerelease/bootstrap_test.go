package prerelease

import (
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func destination(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return filepath.Join(p, "new")
}

func TestIndependentPrereleaseMaterial(t *testing.T) {
	d := destination(t)
	now := time.Now()
	if e := Create(d, "https://mk.fengmap.com:8444", "remoteapp-prerelease", now); e != nil {
		t.Fatal(e)
	}
	read := func(name string) []byte {
		t.Helper()
		b, e := os.ReadFile(filepath.Join(d, name))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var anchor Anchor
	if json.Unmarshal(read("client-public/installation-anchor.json"), &anchor) != nil {
		t.Fatal("anchor")
	}
	root, e := hex.DecodeString(anchor.ReleaseRoot)
	if e != nil {
		t.Fatal(e)
	}
	p, e := trustprofile.Decode(read("client-public/service-profile.json"))
	if e != nil {
		t.Fatal(e)
	}
	caDER, e := trustprofile.Verify(p, root, anchor.Service, anchor.MinimumEpoch, now)
	if e != nil {
		t.Fatal(e)
	}
	ca, e := x509.ParseCertificate(caDER)
	if e != nil {
		t.Fatal(e)
	}
	pair, e := tls.X509KeyPair(read("server-private/api-cert.pem"), read("server-private/api-key.pem"))
	if e != nil {
		t.Fatal(e)
	}
	leaf, e := x509.ParseCertificate(pair.Certificate[0])
	if e != nil {
		t.Fatal(e)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, e = leaf.Verify(x509.VerifyOptions{DNSName: "mk.fengmap.com", Roots: roots, CurrentTime: now}); e != nil {
		t.Fatal(e)
	}
	block, _ := pem.Decode(read("release-local-only/release-root.pem"))
	if block == nil {
		t.Fatal("release")
	}
	k, e := x509.ParsePKCS8PrivateKey(block.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	if hex.EncodeToString(k.(ed25519.PrivateKey).Public().(ed25519.PublicKey)) != anchor.ReleaseRoot {
		t.Fatal("root mismatch")
	}
	s, e := invite.Open(filepath.Join(d, "server-private/authority"), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if hex.EncodeToString(s.PublicKey()) != p.Claims.Issuer {
		t.Fatal("issuer mismatch")
	}
	if s.Close() != nil {
		t.Fatal("close")
	}
	entries, e := os.ReadDir(filepath.Join(d, "client-public"))
	if e != nil || len(entries) != 2 {
		t.Fatal("unexpected public payload")
	}
	if e = filepath.Walk(d, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Errorf("not owner-only: %s", path)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	before := read("client-public/service-profile.json")
	if Create(d, "https://mk.fengmap.com:8444", "remoteapp-prerelease", now) == nil {
		t.Fatal("overwritten")
	}
	if string(before) != string(read("client-public/service-profile.json")) {
		t.Fatal("mutated")
	}
}

func TestRejectInvalidDestinationAndEndpoint(t *testing.T) {
	for _, ep := range []string{"https://example.com", "https://example.com:443", "https://example.com:99999", "http://example.com:8444", "https://example.com:8444/path", "https://user@example.com:8444"} {
		d := destination(t)
		if Create(d, ep, "pre", time.Now()) == nil {
			t.Fatal(ep)
		}
		if _, e := os.Lstat(d); !os.IsNotExist(e) {
			t.Fatal("invalid request created state")
		}
	}
	p := destination(t)
	link := p + "-link"
	if e := os.Symlink(filepath.Dir(p), link); e != nil {
		t.Fatal(e)
	}
	if Create(filepath.Join(link, "child"), "https://example.com:8444", "pre", time.Now()) == nil {
		t.Fatal("symlink accepted")
	}
}
