//go:build windows

package enrollmentstore

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsNativeDirectoryRejectsBroadDACL(t *testing.T) {
	var raw [16]byte
	rand.Read(raw[:])
	d, e := openDirectory(hex.EncodeToString(raw[:]), true, true)
	if e != nil {
		t.Fatal("native TEST directory create failed")
	}
	defer d.Close()
	w := d.(*winDirectory)
	path := filepath.Join(d.Path(), "tailscaled.state")
	if os.WriteFile(path, []byte("fixture-no-private-key"), 0600) != nil || d.Check(false) != nil {
		t.Fatal("private inherited DACL rejected")
	}
	set := func(sddl string) {
		sd, e := windows.SecurityDescriptorFromString(sddl)
		if e != nil {
			t.Fatal("TEST security descriptor invalid")
		}
		acl, _, e := sd.DACL()
		if e != nil || windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil) != nil {
			t.Fatal("TEST DACL update failed")
		}
	}
	private := "D:P(A;;FA;;;" + w.sid.String() + ")"
	set("D:P(A;;FA;;;" + w.sid.String() + ")(A;;FR;;;WD)")
	if d.Check(false) == nil {
		t.Fatal("Everyone read accepted")
	}
	set(private) // exact TEST-only ACL restored, not production permission repair
	if d.Check(false) != nil || d.ClearTestMaterials() != nil || d.Check(true) != nil {
		t.Fatal("TEST DACL restoration/cleanup failed")
	}
}
