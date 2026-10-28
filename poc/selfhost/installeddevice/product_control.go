package installeddevice

import (
	"context"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type ProductMaintenanceOwner interface {
	MaintainDurable(context.Context, clientowner.MaintenanceAction) error
}

type JoinedProduct interface {
	Done() <-chan struct{}
	Wait(context.Context) error
}

// ControlProduct routes the trusted inherited parent pipe, never a remote peer
// or a newly discovered local socket. Maintenance frames carry no selectors.
// The ready reply is valid only on this live connection while Maintain holds
// its native lease. It does not authorize filesystem deletion in another process.
// The caller must Stop/join the runner before releasing borrowed resources on
// every return path; a rejected control frame can precede runner completion.
func ControlProduct(ctx context.Context, owner ProductMaintenanceOwner, handle JoinedProduct, requests <-chan Request, commands chan<- Request, output func(any) error) error {
	return controlProduct(ctx, owner, handle, requests, commands, output, nil)
}

// ControlProductWithState enables a selector-free prepare on the inherited
// trusted parent pipe for TEST releases only. Existing begin/finish is unchanged.
func ControlProductWithState(ctx context.Context, r trustinstall.Release, owner *clientowner.Owner, handle JoinedProduct, requests <-chan Request, commands chan<- Request, output func(any) error) error {
	var prepare func(context.Context, func(context.Context, trustprofile.MaintenanceState) error) error
	if r.Specification().Test && owner != nil {
		prepare = func(life context.Context, action func(context.Context, trustprofile.MaintenanceState) error) error {
			return owner.MaintainDurableState(life, r, action)
		}
	}
	return controlProduct(ctx, owner, handle, requests, commands, output, prepare)
}

func controlProduct(ctx context.Context, owner ProductMaintenanceOwner, handle JoinedProduct, requests <-chan Request, commands chan<- Request, output func(any) error, prepare func(context.Context, func(context.Context, trustprofile.MaintenanceState) error) error) error {
	if ctx == nil || owner == nil || handle == nil || requests == nil || commands == nil || output == nil {
		return trustprofile.ErrTrust
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-handle.Done():
			return handle.Wait(context.Background())
		case request, ok := <-requests:
			if !ok {
				return trustprofile.ErrTrust
			}
			if request.Op == "maintenance_finish" {
				return trustprofile.ErrTrust
			}
			if request.Op == "maintenance_prepare" {
				if prepare == nil || request != (Request{Op: "maintenance_prepare"}) {
					return trustprofile.ErrTrust
				}
				life, cancel := context.WithTimeout(ctx, 40*time.Second)
				defer cancel()
				return prepare(life, func(joined context.Context, state trustprofile.MaintenanceState) error {
					if output(map[string]any{"status": "maintenance_ready", "epoch": state.Epoch, "joined": true, "transaction": state.Transaction}) != nil {
						return trustprofile.ErrTrust
					}
					select {
					case <-joined.Done():
						return joined.Err()
					case finish, ok := <-requests:
						if !ok || finish != (Request{Op: "maintenance_finish"}) {
							return trustprofile.ErrTrust
						}
						return nil
					}
				})
			}
			if request.Op == "maintenance_begin" {
				if request != (Request{Op: "maintenance_begin"}) {
					return trustprofile.ErrTrust
				}
				limit, cancel := context.WithTimeout(ctx, 40*time.Second)
				defer cancel()
				return owner.MaintainDurable(limit, func(life context.Context, epoch uint64) error {
					if err := output(map[string]any{"status": "maintenance_ready", "epoch": epoch, "joined": true}); err != nil {
						return err
					}
					select {
					case <-life.Done():
						return life.Err()
					case finish, ok := <-requests:
						if !ok || finish != (Request{Op: "maintenance_finish"}) {
							return trustprofile.ErrTrust
						}
						return nil
					}
				})
			}
			select {
			case commands <- request:
			case <-handle.Done():
				return handle.Wait(context.Background())
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
