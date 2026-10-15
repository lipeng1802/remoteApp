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
		if _, err := key(expected); err != nil {
			return nil, err
		}
		if e := pinnedSignerExecutable(exe, digest); e != nil {
			return nil, e
		}
		return privatepipe.NewSigner(exe, scope, expected, false)
	}
}

func pinnedSignerExecutable(exe, digest string) error {
	if !filepath.IsAbs(exe) || filepath.Clean(exe) != exe {
		return session.ErrDenied
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil || real != exe {
		return session.ErrDenied
	}
	if _, err = key(digest); err != nil {
		return err
	}
	f, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > 100<<20 {
		return session.ErrDenied
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil || hex.EncodeToString(h.Sum(nil)) != digest {
		return session.ErrDenied
	}
	return nil
}

// CreateNativeIdentity is installation-only wiring for the pinned sibling
// vault. Runtime clients use NativeSigner and can never create an identity.
func CreateNativeIdentity(exe, digest, scope string) (*privatepipe.Signer, error) {
	b, err := hex.DecodeString(scope)
	if err != nil || len(b) != 16 || hex.EncodeToString(b) != scope {
		return nil, session.ErrDenied
	}
	if err := pinnedSignerExecutable(exe, digest); err != nil {
		return nil, err
	}
	return privatepipe.NewSigner(exe, scope, "", true)
}

// DeleteNativeTestIdentity is installer TEST teardown only. The production
// autonomous entry has no call path to it.
func DeleteNativeTestIdentity(exe, digest, scope, expected string) error {
	if _, err := key(expected); err != nil {
		return err
	}
	if err := pinnedSignerExecutable(exe, digest); err != nil {
		return err
	}
	return privatepipe.DeleteTestIdentity(exe, scope, expected)
}
