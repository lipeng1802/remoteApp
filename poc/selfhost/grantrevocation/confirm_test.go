package grantrevocation

import (
	"context"
	"remoteapp.local/selfhost-poc/invite"
	"testing"
	"time"
)

func TestEitherPinnedParticipantOnlyQueriesExactRevokedRequest(t *testing.T) {
	for _, controller := range []bool{false, true} {
		c, g, api, _ := fixture(t)
		pin(t, c, g)
		if controller {
			c.subject = g.Claims.Controller
		}
		api.reply = func(b invite.Body) (invite.Result, error) {
			return invite.Result{Status: "revoked", RequestID: "request"}, nil
		}
		if c.ConfirmRevoked(context.Background()) != nil || len(api.calls) != 1 || api.calls[0] != (invite.Body{Action: "status", RequestID: "request"}) {
			t.Fatal("confirmation mutated or selected another request")
		}
	}
}
func TestFaultConfirmationRejectsMixedUnknownExpiredAndCancelled(t *testing.T) {
	for _, mode := range []string{"pending", "wrong-request", "grant", "mixed", "expired", "rollback", "cancelled", "unobserved"} {
		t.Run(mode, func(t *testing.T) {
			c, g, api, now := fixture(t)
			if mode != "unobserved" {
				pin(t, c, g)
			}
			api.reply = func(b invite.Body) (invite.Result, error) {
				r := invite.Result{Status: "revoked", RequestID: "request"}
				switch mode {
				case "pending":
					r.Status = "pending"
				case "wrong-request":
					r.RequestID = "other"
				case "grant":
					r.Grant = &g
				case "mixed":
					r.Code = "private"
				}
				return r, nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "expired":
				*now = now.Add(time.Hour)
			case "rollback":
				*now = now.Add(-time.Hour)
			case "cancelled":
				cancel()
			}
			if c.ConfirmRevoked(ctx) == nil {
				t.Fatal("unsafe confirmation accepted")
			}
			for _, b := range api.calls {
				if b.Action != "status" {
					t.Fatal("confirmation mutated")
				}
			}
		})
	}
}
