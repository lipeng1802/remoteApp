//go:build (!windows && !darwin) || (darwin && !cgo)

package devicevault

import "errors"

var ErrUnavailable = errors.New("device_vault_unavailable")
var ErrLogonUnavailable = errors.New("interactive_logon_required")

func Load(string, bool) ([]byte, error) { return nil, ErrUnavailable }
func Delete(string) error               { return ErrUnavailable }
