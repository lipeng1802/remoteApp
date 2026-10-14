package enrollmentstore

import "os"

// Exact pinned SDK filenames only. Validate the entire set BEFORE deleting any
// material. Caller checks OS ownership/ACL, links and paths and has joined SDK.
func knownSDKMaterials(entries []os.DirEntry) bool {
	if len(entries) > 4 {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			return false
		}
		switch e.Name() {
		case "tailscaled.state", "tailscaled.log.conf", "tailscaled.log1.txt", "tailscaled.log2.txt":
		default:
			return false
		}
	}
	return true
}
