package installeddevice

// NativeSDKRejectionValidationFactory is TEST-only trusted composition for an
// actual SDK Up against a fixed loopback failure fixture. It never opens a
// production namespace and does not accept a control URL or state path from IPC.
// This is cancellation/join evidence, NOT successful Headscale enrollment.
func NativeSDKRejectionValidationFactory(scope string) EnrollmentFactory {
	return protectedEnrollmentFactory(scope, true, func(dir string) EnrollmentFactory {
		return tsNetEnrollmentFactory(dir, "http://127.0.0.1:18443")
	})
}
