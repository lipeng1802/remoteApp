package trustprofile

import "time"

// VerifyRemovedProfile is a comparison, never a profile getter or admission.
// The caller already holds this compiled public profile. Only the exact native
// files_removed record and a currently valid signature/epoch may match. The
// maintenance fence remains permanent; OpenInstallation/Current still reject.
func VerifyRemovedProfile(a InstallationAnchor, j TrustJournal, expected Profile, clock func() time.Time) error {
	r, _, e := readInstallation(a, j, clock)
	if e != nil || r.Maintenance == nil || r.Maintenance.State != "files_removed" || r.Maintenance.PlanSHA256 == "" || r.Profile != expected || r.Epoch != expected.Claims.Epoch {
		return ErrTrust
	}
	_, e = Verify(expected, a.Root[:], a.Service, r.Epoch, clock())
	return e
}
