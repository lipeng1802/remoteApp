package clientruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/session"
)

// NativeSigner reopens a previously created OS identity using a compiled
// sibling executable digest. It never creates/deletes/exports a device key.
// Hash pinning is integrity, not publisher authenticity or same-user isolation.
func NativeSigner(exe, digest, scope, expected string) SignerFactory {
	return func() (Signer, error) {
		if !filepath.IsAbs(exe) || filepath.Clean(exe) != exe {
			return nil, session.ErrDenied
		}
		real, e := filepath.EvalSymlinks(exe)
		if e != nil || real != exe {
			return nil, session.ErrDenied
		}
		if _, e = key(digest); e != nil {
			return nil, e
		}
		if _, e = key(expected); e != nil {
			return nil, e
		}
		f, e := os.Open(exe)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		st, e := f.Stat()
		if e != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > 100<<20 {
			return nil, session.ErrDenied
		}
		h := sha256.New()
		if _, e = io.Copy(h, f); e != nil || hex.EncodeToString(h.Sum(nil)) != digest {
			return nil, session.ErrDenied
		}
		return privatepipe.NewSigner(exe, scope, expected, false)
	}
}
