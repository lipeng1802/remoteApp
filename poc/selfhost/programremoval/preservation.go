package programremoval

import (
	"crypto/sha256"
	"encoding/hex"
	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"time"
)

type preservationSnapshot struct {
	Epoch      uint64
	Profile    trustprofile.Profile
	Activation string
}

func capturePreservation(r trustinstall.Release, factory trustinstall.Factory, records autonomousbinding.JournalFactory, removed bool) (s preservationSnapshot, result error) {
	j, e := factory(false)
	if e != nil {
		return s, e
	}
	defer func() {
		if e := j.Close(); e != nil {
			result = trustprofile.ErrTrust
		}
	}()
	if removed {
		if e = trustprofile.VerifyRemovedProfile(r.Anchor(), j, r.Specification().Bundled, time.Now); e != nil {
			return s, e
		}
		state, e := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
		if e != nil {
			return s, e
		}
		s.Epoch = state.Epoch
		s.Profile = r.Specification().Bundled
	} else {
		installed, e := trustprofile.OpenInstallation(r.Anchor(), j, time.Now)
		if e != nil {
			return s, e
		}
		p, _, e := installed.Current()
		if e != nil || p != r.Specification().Bundled {
			return s, trustprofile.ErrTrust
		}
		s.Profile = p
		s.Epoch = p.Claims.Epoch
	}
	scope, e := installrelease.Scope(r.Scope())
	if e != nil {
		return s, e
	}
	a, e := records(scope, false, true)
	if e != nil {
		return s, e
	}
	raw, e := a.Load()
	closed := a.Close()
	if e != nil || closed != nil {
		return s, trustprofile.ErrTrust
	}
	h := sha256.Sum256(raw)
	s.Activation = hex.EncodeToString(h[:])
	return s, nil
}
