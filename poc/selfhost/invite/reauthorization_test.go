package invite

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFreshApprovalDoesNotReviveRevokedOrExpiredGrant(t *testing.T) {
	for _, cause := range []string{"revoked", "expired"} {
		t.Run(cause, func(t *testing.T) {
			f := newFixture(t)
			oldInvite := f.invitation(t)
			oldRequest := f.claim(t, oldInvite)
			old := *f.exec(t, f.owner, Body{Action: "approve", RequestID: oldRequest.RequestID, Allow: true}).Grant
			if cause == "revoked" {
				f.exec(t, f.owner, Body{Action: "revoke", GrantID: old.Claims.ID})
			} else {
				f.now = time.Unix(old.Claims.ExpiresAt, 0)
			}
			f.restart(t)
			checkOld := func() {
				t.Helper()
				status := f.exec(t, f.client, Body{Action: "status", RequestID: oldRequest.RequestID})
				if status.Status != cause || status.Grant != nil || f.s.Authorized(old, f.target.DeviceID, f.controller.DeviceID) {
					t.Fatal("old authorization revived")
				}
				backend := &ProvisionBackend{Store: f.s, Driver: &memoryDriver{}}
				proof := apiEnvelope(f, f.client, old, "grant_clock")
				proof.Body.Challenge = strings.Repeat("a", 64)
				proof = SignNetworkRequest(f.client, proof.Body)
				if _, err := backend.ExecuteNetworkRequest(context.Background(), proof); err != ErrDenied {
					t.Fatal("old grant obtained online clock proof")
				}
			}
			checkOld()
			if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "approve", RequestID: oldRequest.RequestID, Allow: true})); err != ErrDenied {
				t.Fatal("old request reapproved")
			}
			if _, err := f.s.Execute(f.envelope(f.client, Body{Action: "claim", TargetCode: f.target.DeviceCode, Code: oldInvite.Code})); err != ErrDenied {
				t.Fatal("old invitation reused")
			}
			fresh := f.claim(t, f.invitation(t))
			if fresh.RequestID == oldRequest.RequestID || fresh.Status != "pending" || fresh.Grant != nil {
				t.Fatal("fresh claim skipped consent")
			}
			if _, err := f.s.Execute(f.envelope(f.client, Body{Action: "approve", RequestID: fresh.RequestID, Allow: true})); err != ErrDenied {
				t.Fatal("controller self-approved")
			}
			f.restart(t)
			if f.exec(t, f.client, Body{Action: "status", RequestID: fresh.RequestID}).Grant != nil {
				t.Fatal("restart auto-approved pending request")
			}
			grant := *f.exec(t, f.owner, Body{Action: "approve", RequestID: fresh.RequestID, Allow: true}).Grant
			if grant.Claims.ID == old.Claims.ID || !f.s.Authorized(grant, f.target.DeviceID, f.controller.DeviceID) {
				t.Fatal("fresh approval not distinct and active")
			}
			f.restart(t)
			checkOld()
			if !f.s.Authorized(grant, f.target.DeviceID, f.controller.DeviceID) {
				t.Fatal("fresh approval lost on restart")
			}
			status := f.exec(t, f.client, Body{Action: "status", RequestID: fresh.RequestID})
			if status.Grant == nil || *status.Grant != grant {
				t.Fatal("fresh status lost exact grant")
			}
			backend := &ProvisionBackend{Store: f.s, Driver: &memoryDriver{}}
			proof := apiEnvelope(f, f.client, grant, "grant_clock")
			proof.Body.Challenge = strings.Repeat("b", 64)
			proof = SignNetworkRequest(f.client, proof.Body)
			response, err := backend.ExecuteNetworkRequest(context.Background(), proof)
			if err != nil || response.Proof == nil {
				t.Fatal("fresh grant online verification rejected")
			}
		})
	}
}
