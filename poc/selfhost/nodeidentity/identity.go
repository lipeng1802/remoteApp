// Package nodeidentity bootstraps a private profile for pinned tsnet 1.102.5.
// This is a version-sensitive state adapter, NOT a promised stable tsnet API.
package nodeidentity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"tailscale.com/ipn"
	"tailscale.com/ipn/store"
	"tailscale.com/types/key"
	"tailscale.com/types/persist"
)

var ErrIdentity = errors.New("private node identity unavailable")

// Prepare is offline: no network, browser, credential, or fake registered NodeID.
// The key never leaves the private StateStore. Existing corrupt state fails closed.
// Caller owns an exclusive helper process/dir; simultaneous processes forbidden.
func Prepare(dir, controlURL string) (string, error) {
	if controlURL != "https://mk.fengmap.com:8443" && controlURL != "http://127.0.0.1:18443" {
		return "", ErrIdentity
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", ErrIdentity
	}
	st, e := os.Lstat(dir)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return "", ErrIdentity
	}
	if os.Chmod(dir, 0700) != nil {
		return "", ErrIdentity
	}
	path := filepath.Join(dir, "tailscaled.state")
	_, e = os.Lstat(path)
	fresh := os.IsNotExist(e)
	if fresh {
		entries, e := os.ReadDir(dir)
		if e != nil || len(entries) != 0 {
			return "", ErrIdentity
		}
	}
	if !fresh && e != nil {
		return "", ErrIdentity
	}
	if !fresh {
		info, e := os.Lstat(path)
		if e != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 || info.Size() > 1<<20 {
			return "", ErrIdentity
		}
	}
	s, e := store.NewFileStore(func(string, ...any) {}, path)
	if e != nil {
		return "", ErrIdentity
	}
	if fresh {
		idBytes := make([]byte, 2)
		if _, e = rand.Read(idBytes); e != nil {
			return "", ErrIdentity
		}
		id := ipn.ProfileID(hex.EncodeToString(idBytes))
		pk := ipn.StateKey("profile-" + string(id))
		p := ipn.NewPrefs()
		p.ControlURL = controlURL
		p.WantRunning = true
		p.LoggedOut = false
		p.Persist = &persist.Persist{PrivateNodeKey: key.NewNode()}
		profile := &ipn.LoginProfile{ID: id, Key: pk, ControlURL: controlURL}
		profiles, _ := json.Marshal(map[ipn.ProfileID]*ipn.LoginProfile{id: profile})
		for _, w := range []struct {
			k ipn.StateKey
			b []byte
		}{{pk, p.ToBytes()}, {ipn.KnownProfilesStateKey, profiles}, {ipn.CurrentProfileStateKey, []byte(pk)}, {ipn.ServerModeStartKey, []byte(pk)}} {
			if s.WriteState(w.k, w.b) != nil {
				return "", ErrIdentity
			}
		}
	}
	pk, e := s.ReadState(ipn.CurrentProfileStateKey)
	if e != nil || len(pk) == 0 {
		return "", ErrIdentity
	}
	raw, e := s.ReadState(ipn.StateKey(pk))
	var p ipn.Prefs
	if e != nil || json.Unmarshal(raw, &p) != nil || p.ControlURL != controlURL || p.Persist == nil || p.Persist.PrivateNodeKey.IsZero() {
		return "", ErrIdentity
	}
	return p.Persist.PrivateNodeKey.Public().String(), nil
}
