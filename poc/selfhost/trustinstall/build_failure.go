package trustinstall

import (
	"context"
	"errors"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var ErrBuildTimeout = errors.New("trusted_build_timeout")
var ErrBuildInterrupted = errors.New("trusted_build_interrupted")
var ErrBuildProcess = errors.New("trusted_build_process_failed")

func buildProcessFailure(ctx context.Context) error {
	if ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.Join(trustprofile.ErrTrust, ErrBuildTimeout)
	}
	if ctx != nil && ctx.Err() != nil {
		return errors.Join(trustprofile.ErrTrust, ErrBuildInterrupted)
	}
	return errors.Join(trustprofile.ErrTrust, ErrBuildProcess)
}
func BuildFailureReason(err error) string {
	if errors.Is(err, ErrBuildTimeout) {
		return "trusted_build_timeout"
	}
	if errors.Is(err, ErrBuildInterrupted) {
		return "trusted_build_interrupted"
	}
	if errors.Is(err, ErrBuildProcess) {
		return "trusted_build_process_failed"
	}
	return "trusted_build_rejected"
}
