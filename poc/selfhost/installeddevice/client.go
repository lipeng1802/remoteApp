// Package installeddevice owns device API requests under an installed-profile
// lease. It never accepts endpoint/root/CA/issuer or an exported device key.
package installeddevice

import (
	"context"
	"encoding/hex"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type Request struct {
	Op, VaultScope, ExpectedPublic string
	Device                         invite.Body
}

func canonical(value string, size int) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == size && hex.EncodeToString(b) == value
}

func ValidOpen(r Request) bool {
	return r.Op == "open" && canonical(r.VaultScope, 16) && canonical(r.ExpectedPublic, 32) && r.Device == (invite.Body{})
}

func validCommand(r Request) bool {
	if r.VaultScope != "" || r.ExpectedPublic != "" {
		return false
	}
	if r.Op == "stop" {
		return r.Device == (invite.Body{})
	}
	if r.Op != "device" || r.Device.Nonce != "" || r.Device.IssuedAt != 0 {
		return false
	}
	switch r.Device.Action {
	case "register", "invite", "claim", "incoming", "approve", "status", "revoke":
		return true
	}
	return false
}

// Runner is started only by clientowner.Open/Start, which supplies the durable
// current profile and monotonic lifetime. The output is a trusted private pipe,
// not a log or public peer interface; device results can contain invite/grant.
func Runner(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error) clientowner.Runner {
	return func(ctx context.Context, profile trustprofile.Profile) (result error) {
		if ctx.Err() != nil || open == nil || output == nil || commands == nil {
			return trustprofile.ErrTrust
		}
		signer, err := open()
		if err != nil || signer == nil {
			return trustprofile.ErrTrust
		}
		joined, ok := signer.(interface{ CloseJoined() error })
		if !ok {
			signer.Close()
			return trustprofile.ErrTrust
		}
		defer func() {
			if err := joined.CloseJoined(); err != nil {
				result = trustprofile.ErrTrust
			}
		}()
		anchor := release.Anchor()
		client, err := invite.NewProfileNetworkClient(profile, anchor.Root[:], anchor.Service, anchor.MinimumEpoch, signer)
		if err != nil {
			return trustprofile.ErrTrust
		}
		defer client.Close()
		if err := output(map[string]string{"status": "ready"}); err != nil {
			return err
		}
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.Canceled {
					return nil
				}
				return trustprofile.ErrTrust
			case request, exists := <-commands:
				if !exists {
					return nil
				}
				if !validCommand(request) {
					return trustprofile.ErrTrust
				}
				if request.Op == "stop" {
					return nil
				}
				call, cancel := context.WithTimeout(ctx, 15*time.Second)
				value, err := client.DeviceRequest(call, request.Device)
				cancel()
				reply := map[string]any{"status": "device_result", "passed": err == nil}
				if err == nil {
					reply["device"] = value
				}
				if err := output(reply); err != nil {
					return err
				}
			}
		}
	}
}
