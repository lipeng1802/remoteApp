package grantrevocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
)

func TestDesktopRevocationPolicyCannotBeInferredFromGrant(t *testing.T) {
	for _, expected := range []invite.DesktopPermission{"", invite.DesktopView, invite.DesktopControl} {
		for _, granted := range []invite.DesktopPermission{"", invite.DesktopView, invite.DesktopControl} {
			t.Run(string(expected)+"/"+string(granted), func(t *testing.T) {
				c, g, api, clock := scopedFixture(t, expected, granted)
				defer c.Close()
				*clock = clock.Add(-time.Second)
				err := c.Observe("request", g)
				if errors.Is(err, errGrantNotYetValid) != (expected == granted) {
					t.Fatal("future wrong-scope grant became admissible")
				}
				*clock = clock.Add(time.Second)
				if (c.Observe("request", g) == nil) != (expected == granted) {
					t.Fatal("scope selected from grant")
				}
				if expected != granted && (c.Pin(g) == nil || c.Revoke(context.Background()) == nil) {
					t.Fatal("wrong scope reached mutation")
				}
				if len(api.calls) != 0 {
					t.Fatal("scope rejection performed HTTP")
				}
			})
		}
	}
	for _, scope := range []invite.DesktopPermission{"", "fixed-payload", "clipboard", "desktop-view,desktop-control"} {
		if c, err := NewDesktop(trustinstall.Specification{Test: true}, nil, 0, scope); err == nil || c != nil {
			t.Fatal("invalid local policy constructed channel")
		}
	}
}

func TestDesktopUnknownRevocationRequiresExactScopedReceipt(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		for _, mode := range []string{"revoked", "approved", "approved-mixed", "approved-missing-scope", "approved-other-scope", "missing-scope", "other-scope", "mixed", "wrong-request", "foreign-grant", "missing-grant", "unknown"} {
			t.Run(string(scope)+"/"+mode, func(t *testing.T) {
				c, g, api, _ := scopedFixture(t, scope, scope)
				defer c.Close()
				pin(t, c, g)
				mutations := 0
				api.reply = func(b invite.Body) (invite.Result, error) {
					if b.Action == "revoke" {
						if b != (invite.Body{Action: "revoke", GrantID: g.Claims.ID}) {
							t.Fatal("mutation selectors")
						}
						mutations++
						if mutations == 1 {
							return invite.Result{}, errors.New("receipt lost")
						}
						return invite.Result{Status: "revoked"}, nil
					}
					if b != (invite.Body{Action: "status", RequestID: "request"}) {
						t.Fatal("resolution selectors")
					}
					r := invite.Result{Status: "revoked", RequestID: "request", Permission: scope}
					switch mode {
					case "approved":
						r.Status, r.Grant = "approved", &g
					case "approved-mixed":
						r.Status, r.Grant, r.InviteID = "approved", &g, "extra"
					case "approved-missing-scope":
						r.Status, r.Grant, r.Permission = "approved", &g, ""
					case "approved-other-scope":
						r.Status, r.Grant, r.Permission = "approved", &g, "clipboard"
					case "missing-scope":
						r.Permission = ""
					case "other-scope":
						r.Permission = invite.DesktopView
						if scope == invite.DesktopView {
							r.Permission = invite.DesktopControl
						}
					case "mixed":
						r.ControllerID = g.Claims.Controller
					case "wrong-request":
						r.RequestID = "other"
					case "foreign-grant":
						x := g
						x.Claims.ID = "foreign"
						r.Status, r.Grant = "approved", &x
					case "missing-grant":
						r.Status = "approved"
					case "unknown":
						return invite.Result{}, errors.New("unknown status")
					}
					return r, nil
				}
				if c.Revoke(context.Background()) == nil {
					t.Fatal("lost receipt confirmed")
				}
				err := c.Revoke(context.Background())
				if (err == nil) != (mode == "revoked" || mode == "approved") {
					t.Fatal("unsafe status proof")
				}
				want := 1
				if mode == "approved" {
					want = 2
				}
				if mutations != want || api.calls[1].Action != "status" {
					t.Fatal("blind mutation retry")
				}
			})
		}
	}
}

func TestDesktopPreprovisionCleanupAndPinnedConfirmationAreScopeBound(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		for _, terminal := range []string{"denied", "revoked"} {
			for _, mode := range []string{"exact", "missing", "wrong", "grant", "extra", "expired", "rollback"} {
				t.Run(string(scope)+"/"+terminal+"/"+mode, func(t *testing.T) {
					c, g, api, clock := scopedFixture(t, scope, scope)
					defer c.Close()
					ctx := context.Background()
					if c.ObserveRequest(ctx, "request") != nil {
						t.Fatal("bind request")
					}
					api.reply = func(b invite.Body) (invite.Result, error) {
						if b != (invite.Body{Action: "status", RequestID: "request"}) {
							t.Fatal("cleanup mutated")
						}
						r := invite.Result{Status: terminal, RequestID: "request", Permission: scope}
						switch mode {
						case "missing":
							r.Permission = ""
						case "wrong":
							r.Permission = "clipboard"
						case "grant":
							r.Grant = &g
						case "extra":
							r.Code = "extra"
						}
						return r, nil
					}
					if mode == "expired" {
						*clock = clock.Add(time.Hour)
					}
					if mode == "rollback" {
						*clock = clock.Add(-time.Hour)
					}
					if (c.ConfirmBeforeProvision(ctx) == nil) != (mode == "exact") {
						t.Fatal("unsafe preprovision close")
					}
					if (c.ObserveRequest(ctx, "next") == nil) != (mode == "exact") {
						t.Fatal("unsafe request replacement")
					}
					if mode == "exact" && (c.request != "next" || c.grant != (invite.Grant{})) {
						t.Fatal("previous grant retained")
					}
				})
			}
		}
		for _, controller := range []bool{false, true} {
			c, g, api, _ := scopedFixture(t, scope, scope)
			pin(t, c, g)
			if controller {
				c.subject = g.Claims.Controller
			}
			api.reply = func(b invite.Body) (invite.Result, error) {
				if b.Action != "status" {
					t.Fatal("confirmation mutated")
				}
				return invite.Result{Status: "revoked", RequestID: "request", Permission: scope}, nil
			}
			if c.ConfirmRevoked(context.Background()) != nil {
				t.Fatal("either participant must confirm")
			}
			c.Close()
		}
	}
}

func TestRevocationMixedMutationReceiptIsUnknownRatherThanJoinAuthority(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{"", invite.DesktopView, invite.DesktopControl} {
		c, g, api, _ := scopedFixture(t, scope, scope)
		pin(t, c, g)
		api.reply = func(b invite.Body) (invite.Result, error) {
			return invite.Result{Status: "revoked", RequestID: "request"}, nil
		}
		if c.Revoke(context.Background()) == nil || c.confirmed {
			t.Fatal("mixed mutation receipt admitted")
		}
		api.reply = func(b invite.Body) (invite.Result, error) {
			if b.Action != "status" {
				t.Fatal("mutation retried")
			}
			return invite.Result{Status: "revoked", RequestID: "request", Permission: scope}, nil
		}
		if c.Revoke(context.Background()) != nil {
			t.Fatal("exact status failed to resolve")
		}
		c.Close()
	}
}

func TestFixedChannelCannotAdoptScopedTerminalReceipt(t *testing.T) {
	c, g, api, _ := fixture(t)
	defer c.Close()
	ctx := context.Background()
	if c.ObserveRequest(ctx, "request") != nil {
		t.Fatal("request bind")
	}
	api.reply = func(invite.Body) (invite.Result, error) {
		return invite.Result{Status: "revoked", RequestID: "request", Permission: invite.DesktopView}, nil
	}
	if c.ConfirmBeforeProvision(ctx) == nil || c.ObserveRequest(ctx, "other") == nil {
		t.Fatal("default adopted scoped terminal")
	}
	pin(t, c, g)
	if c.ConfirmRevoked(ctx) == nil {
		t.Fatal("default confirmed scoped grant")
	}
	c.attempted = true // resolve the same request, never send a new mutation
	if c.Revoke(ctx) == nil || c.confirmed {
		t.Fatal("default resolved with scoped receipt")
	}
	for _, b := range api.calls {
		if b.Action != "status" {
			t.Fatal("scoped receipt caused mutation")
		}
	}
}
