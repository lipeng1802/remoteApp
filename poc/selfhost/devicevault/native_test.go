//go:build windows || (darwin && cgo)

package devicevault

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func TestNativeCreateIsExclusiveAndReopenStable(t *testing.T) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal(err)
	}
	scope := hex.EncodeToString(raw[:])
	seed, err := Load(scope, true)
	if err != nil || len(seed) != 32 {
		t.Fatal("create", err)
	}
	defer func() {
		for i := range seed {
			seed[i] = 0
		}
	}()
	if duplicate, duplicateErr := Load(scope, true); duplicateErr == nil || duplicate != nil {
		t.Fatal("existing identity accepted as newly created")
	}
	reopened, err := Load(scope, false)
	if err != nil || !bytes.Equal(seed, reopened) {
		t.Fatal("stable reopen", err)
	}
	for i := range reopened {
		reopened[i] = 0
	}
	if err = Delete(scope); err != nil {
		t.Fatal("delete exact test identity", err)
	}
	if missing, missingErr := Load(scope, false); missingErr == nil || missing != nil {
		t.Fatal("deleted identity reopened")
	}
}
