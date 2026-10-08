// Package trustinstall supplies an offline configuration-maintenance owner.
// Its specification MUST be compiled into trusted installer code, not accepted
// from a server, sidecar file, environment or peer. Authentic distribution/code
// signing remains a separate release responsibility.
package trustinstall

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type Specification struct {
	Version      int                  `json:"version"`
	Channel      string               `json:"channel"`
	Root         string               `json:"release_root"`
	Service      string               `json:"service"`
	MinimumEpoch uint64               `json:"minimum_epoch"`
	HelperDigest string               `json:"helper_sha256"`
	Bundled      trustprofile.Profile `json:"bundled_profile"`
	Test         bool                 `json:"test_namespace"`
}

type Release struct {
	spec   Specification
	anchor trustprofile.InstallationAnchor
}

func canonical(s string, n int) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == n && hex.EncodeToString(b) == s
}

// Scope is stable across root/service/floor/helper changes: root replacement
// cannot silently select a fresh namespace and bypass the existing journal.
func (r Release) Scope() string {
	h := sha256.Sum256([]byte("remoteapp-installed-trust-channel-v1\n" + r.spec.Channel))
	return hex.EncodeToString(h[:16])
}
func (r Release) Specification() Specification            { return r.spec }
func (r Release) Anchor() trustprofile.InstallationAnchor { return r.anchor }

func validate(s Specification) (Release, error) {
	if s.Version != 1 || len(s.Channel) < 1 || len(s.Channel) > 64 || s.Channel[0] == '-' || strings.Trim(s.Channel, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" ||
		!canonical(s.Root, 32) || s.Root == strings.Repeat("0", 64) || !canonical(s.HelperDigest, 32) || s.HelperDigest == strings.Repeat("0", 64) ||
		s.Service == "" || len(s.Service) > 64 || strings.ContainsAny(s.Service, "\r\n\x00") || s.MinimumEpoch == 0 || s.MinimumEpoch > 1<<53-1 {
		return Release{}, trustprofile.ErrTrust
	}
	var a trustprofile.InstallationAnchor
	a.Service = s.Service
	a.MinimumEpoch = s.MinimumEpoch
	b, _ := hex.DecodeString(s.Root)
	copy(a.Root[:], b)
	// Historical validation only decodes compiled release material. First
	// installation still validates at current time before creating native state.
	if _, e := trustprofile.Verify(s.Bundled, a.Root[:], a.Service, a.MinimumEpoch, time.Unix(s.Bundled.Claims.IssuedAt, 0)); e != nil {
		return Release{}, e
	}
	return Release{s, a}, nil
}

func EncodeForTrustedBuild(s Specification, now time.Time) (string, error) {
	r, e := validate(s)
	if e != nil {
		return "", e
	}
	if _, e = trustprofile.Verify(s.Bundled, r.anchor.Root[:], s.Service, s.MinimumEpoch, now); e != nil {
		return "", e
	}
	b, e := json.Marshal(s)
	if e != nil || len(b) > 16383 {
		return "", trustprofile.ErrTrust
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func DecodeCompiled(encoded string) (Release, error) {
	if len(encoded) == 0 || len(encoded) > 24000 {
		return Release{}, trustprofile.ErrTrust
	}
	b, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || base64.StdEncoding.EncodeToString(b) != encoded || len(b) > 16383 {
		return Release{}, trustprofile.ErrTrust
	}
	r := privatepipe.NewBoundedReader(bytes.NewReader(append(b, '\n')), 16384)
	var s Specification
	var extra struct{}
	if r.Read(&s) != nil || r.Read(&extra) != io.EOF {
		return Release{}, trustprofile.ErrTrust
	}
	return validate(s)
}

type Journal interface {
	trustprofile.TrustJournal
	Close() error
}
type Factory func(create bool) (Journal, error)

// NativeFactory pins the sibling helper before every start. Hash pinning is
// transfer/content integrity, not publisher authentication or protection from
// a malicious same-user binary replacement race; signed protected installation
// remains required for release. No helper path comes from a downloaded profile.
func (r Release) NativeFactory(helper string) (Factory, error) {
	if !filepath.IsAbs(helper) || filepath.Clean(helper) != helper {
		return nil, trustprofile.ErrTrust
	}
	check := func() error {
		real, e := filepath.EvalSymlinks(helper)
		if e != nil || real != helper {
			return trustprofile.ErrTrust
		}
		f, e := os.Open(helper)
		if e != nil {
			return trustprofile.ErrTrust
		}
		defer f.Close()
		st, e := f.Stat()
		if e != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > 100<<20 {
			return trustprofile.ErrTrust
		}
		h := sha256.New()
		if _, e = io.Copy(h, f); e != nil || hex.EncodeToString(h.Sum(nil)) != r.spec.HelperDigest {
			return trustprofile.ErrTrust
		}
		return nil
	}
	if check() != nil {
		return nil, trustprofile.ErrTrust
	}
	return func(create bool) (Journal, error) {
		if check() != nil {
			return nil, trustprofile.ErrTrust
		}
		if r.spec.Test {
			return trustprofile.NewTestDesktopJournal(helper, r.Scope(), create)
		}
		return trustprofile.NewDesktopJournal(helper, r.Scope(), create)
	}, nil
}

type OfflineOwner struct {
	release Release
	factory Factory
	clock   func() time.Time
}

func NewOfflineOwner(r Release, f Factory, clock func() time.Time) (*OfflineOwner, error) {
	if _, e := validate(r.spec); e != nil || f == nil || clock == nil {
		return nil, trustprofile.ErrTrust
	}
	return &OfflineOwner{r, f, clock}, nil
}

func (o *OfflineOwner) Initialize() (trustprofile.Profile, error) {
	p := o.release.spec.Bundled
	a := o.release.anchor
	if _, e := trustprofile.Verify(p, a.Root[:], a.Service, a.MinimumEpoch, o.clock()); e != nil {
		return trustprofile.Profile{}, e
	}
	j, e := o.factory(true)
	if e != nil {
		return trustprofile.Profile{}, e
	}
	s, e := trustprofile.InitializeInstallation(a, p, j, o.clock)
	if e != nil {
		j.Close()
		return trustprofile.Profile{}, e
	}
	current, _, e := s.Current()
	if closed := j.Close(); e == nil {
		e = closed
	}
	return current, e
}

func (o *OfflineOwner) Inspect() (trustprofile.Profile, error) {
	j, e := o.factory(false)
	if e != nil {
		return trustprofile.Profile{}, e
	}
	s, e := trustprofile.OpenInstallation(o.release.anchor, j, o.clock)
	if e != nil {
		j.Close()
		return trustprofile.Profile{}, e
	}
	p, _, e := s.Current()
	if closed := j.Close(); e == nil {
		e = closed
	}
	return p, e
}

type noOfflineSessions struct{}

func (noOfflineSessions) Quiesce(context.Context) error { return nil }

// Upgrade owns the exclusive native journal in a process that never opens
// watch/TLS/tunnel sessions. It is NOT a substitute for a live client's actual
// cancel/join adapter: that integration is the following slice.
func (o *OfflineOwner) Upgrade(ctx context.Context, p trustprofile.Profile) (trustprofile.Profile, error) {
	if ctx == nil || ctx.Err() != nil {
		return trustprofile.Profile{}, trustprofile.ErrTrust
	}
	j, e := o.factory(false)
	if e != nil {
		return trustprofile.Profile{}, e
	}
	s, e := trustprofile.OpenInstallation(o.release.anchor, j, o.clock)
	if e != nil {
		j.Close()
		return trustprofile.Profile{}, e
	}
	if e = s.Accept(ctx, p, noOfflineSessions{}); e != nil {
		j.Close()
		return trustprofile.Profile{}, e
	}
	current, _, e := s.Current()
	if closed := j.Close(); e == nil {
		e = closed
	}
	return current, e
}
