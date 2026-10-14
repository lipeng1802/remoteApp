package enrollmentstore

import (
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/nodeidentity"
)

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

type sdkRemoval struct {
	path      string
	directory bool
}

// Successful pinned SDK login also creates an EMPTY cache hierarchy even with
// TS_USE_CACHED_NETMAP=false. Bind its profile to the actual current state, and
// reject ANY cache content/extra profile/unknown file before deleting anything.
func sdkCleanupPlan(dir string) ([]sdkRemoval, error) {
	entries, e := os.ReadDir(dir)
	if e != nil || len(entries) > 5 {
		return nil, ErrStorage
	}
	var files []os.DirEntry
	hasProfiles := false
	for _, entry := range entries {
		if entry.Name() == "profile-data" && entry.IsDir() {
			hasProfiles = true
		} else {
			files = append(files, entry)
		}
	}
	if !knownSDKMaterials(files) {
		return nil, ErrStorage
	}
	var plan []sdkRemoval
	for _, f := range files {
		plan = append(plan, sdkRemoval{path: f.Name()})
	}
	if !hasProfiles {
		return plan, nil
	}
	id, e := nodeidentity.ReadProfileID(dir)
	if e != nil {
		return nil, ErrStorage
	}
	profiles, e := os.ReadDir(filepath.Join(dir, "profile-data"))
	if e != nil || len(profiles) > 1 {
		return nil, ErrStorage
	}
	if len(profiles) == 1 {
		if profiles[0].Name() != id || !profiles[0].IsDir() {
			return nil, ErrStorage
		}
		profile := filepath.Join("profile-data", id)
		children, e := os.ReadDir(filepath.Join(dir, profile))
		if e != nil || len(children) > 1 {
			return nil, ErrStorage
		}
		if len(children) == 1 {
			if children[0].Name() != "netmap-cache" || !children[0].IsDir() {
				return nil, ErrStorage
			}
			cache := filepath.Join(profile, "netmap-cache")
			contents, e := os.ReadDir(filepath.Join(dir, cache))
			if e != nil || len(contents) != 0 {
				return nil, ErrStorage
			}
			plan = append(plan, sdkRemoval{path: cache, directory: true})
		}
		plan = append(plan, sdkRemoval{path: profile, directory: true})
	}
	plan = append(plan, sdkRemoval{path: "profile-data", directory: true})
	return plan, nil
}
