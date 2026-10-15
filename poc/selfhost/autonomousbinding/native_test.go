//go:build darwin || windows

package autonomousbinding

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func randomHex(t *testing.T, size int) string {
	t.Helper()
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw)
}

func TestNativeReservationIsTerminalAndCommittedBindingReopens(t *testing.T) {
	release := randomHex(t, 16)
	reservation, err := Begin(release, true)
	if err != nil {
		t.Fatal("reserve", err)
	}
	if err = reservation.Close(); err != nil {
		t.Fatal("close pending", err)
	}
	if retry, retryErr := Begin(release, true); retryErr == nil || retry != nil {
		t.Fatal("pending install was silently retried")
	}
	if err = CleanupTest(release); err != nil {
		t.Fatal("cleanup isolated pending test", err)
	}

	release = randomHex(t, 16)
	vault, public := randomHex(t, 16), randomHex(t, 32)
	reservation, err = Begin(release, true)
	if err != nil {
		t.Fatal("reserve committed", err)
	}
	binding, err := reservation.Commit(vault, public)
	if err != nil || binding.Check() != nil || binding.Record().PublicKey != public {
		t.Fatal("commit binding", err)
	}
	if err = binding.Close(); err != nil {
		t.Fatal("close binding", err)
	}
	reopened, err := Open(release, true)
	if err != nil || reopened.Check() != nil || reopened.Record().VaultScope != vault {
		t.Fatal("reopen binding", err)
	}
	if err = reopened.Close(); err != nil {
		t.Fatal("close reopen", err)
	}
	if err = CleanupTest(release); err != nil {
		t.Fatal("cleanup isolated binding test", err)
	}
}
