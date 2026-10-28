// Package installeradapter coordinates fixed installer children. It never
// chooses releases, roots, credentials, or payload paths at runtime.
package installeradapter

import "errors"

var ErrInstall = errors.New("installer_adapter_rejected")

type Reply struct {
	Status string `json:"status"`
	Epoch  uint64 `json:"epoch,omitempty"`
	Phase  string `json:"phase,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type Invoke func(tool string, args ...string) (Reply, error)

// Run coordinates the restart boundaries already enforced by the signed
// transaction and managers. Every success requires a normal child exit.
func Run(operation string, epoch uint64, invoke Invoke) error {
	if (operation != "install" && operation != "upgrade") || epoch == 0 || epoch > 1<<53-1 || invoke == nil {
		return ErrInstall
	}
	prepared := false
	for owner := 0; owner < 2; owner++ {
		r, err := invoke("install-transaction")
		if err == nil && r.Status == "prepared" && r.Epoch == epoch && (r.Phase == "staged" || r.Phase == "promoted") {
			prepared = true
			break
		}
	}
	if !prepared {
		return ErrInstall
	}
	r, err := invoke("installed-release-manager", operation)
	if err != nil || r.Status != "active" || r.Epoch != epoch {
		return ErrInstall
	}
	r, err = invoke("install-transaction")
	if err != nil || r.Status != "prepared" || r.Epoch != epoch || r.Phase != "promoted" {
		return ErrInstall
	}
	active := false
	for owner := 0; owner < 4; owner++ {
		r, err = invoke("installed-bootstrap-manager")
		if err == nil && r.Status == "active" && r.Epoch == epoch {
			active = true
			break
		}
	}
	if !active {
		return ErrInstall
	}
	for _, tool := range []string{"installed-release-manager", "installed-bootstrap-manager"} {
		r, err = invoke(tool, "status")
		if err != nil || r.Status != "active" || r.Epoch != epoch {
			return ErrInstall
		}
	}
	return nil
}
