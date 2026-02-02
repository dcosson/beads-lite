// Package routing resolves issue IDs to the correct storage location
// by mapping ID prefixes to rig paths via a routes.json file.
package routing

import (
	"os"
	"path/filepath"

	"beads-lite/internal/config"
)

// Route maps a prefix to a rig path relative to town root.
type Route struct {
	Path string `json:"path"`
}

// RoutesFile is the on-disk format of .beads/routes.json.
type RoutesFile struct {
	PrefixRoutes map[string]Route `json:"prefix_routes"`
}

// Router resolves issue IDs to the correct storage location.
type Router struct {
	townRoot   string           // absolute path to town root
	localBeads string           // absolute path to local .beads dir
	routes     map[string]Route // prefix → route
}

// New creates a Router by discovering routes.json from the given .beads dir.
// Walks up parent directories to find .beads/routes.json at the town root.
// Returns nil Router (not an error) if no routes.json exists or it has no
// prefix_routes key.
func New(beadsDir string) (*Router, error) {
	absBeads, err := filepath.Abs(beadsDir)
	if err != nil {
		return nil, err
	}

	townRoot, routesPath := findRoutesFile(absBeads)
	if routesPath == "" {
		return nil, nil
	}

	routes, err := LoadRoutes(routesPath)
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, nil
	}

	return &Router{
		townRoot:   townRoot,
		localBeads: absBeads,
		routes:     routes,
	}, nil
}

// Resolve returns the Paths for the rig that owns the given issue ID.
// Returns (zeroPaths, "", false, nil) if the router is nil or no route matches.
// The prefix return value is the matched ID prefix (e.g. "hq-").
// The isRemote return value is true when the resolved path differs from
// the local .beads directory.
func (r *Router) Resolve(issueID string) (config.Paths, string, bool, error) {
	if r == nil {
		return config.Paths{}, "", false, nil
	}

	prefix := ExtractPrefix(issueID)
	if prefix == "" {
		return config.Paths{}, "", false, nil
	}

	route, ok := r.routes[prefix]
	if !ok {
		return config.Paths{}, "", false, nil
	}

	targetBeads := filepath.Join(r.townRoot, route.Path, ".beads")
	targetBeads = filepath.Clean(targetBeads)

	paths, err := config.ResolveFromBase(targetBeads)
	if err != nil {
		return config.Paths{}, "", false, err
	}

	// Detect self-routing: if resolved path is the local .beads, it's not remote.
	resolvedAbs, _ := filepath.Abs(paths.ConfigDir)
	isRemote := resolvedAbs != r.localBeads

	return paths, prefix, isRemote, nil
}

// findRoutesFile walks up from beadsDir looking for .beads/routes.json.
// First checks the given beadsDir itself, then walks parent directories
// looking for .beads/routes.json. Returns (townRoot, routesPath) or
// ("", "") if not found.
func findRoutesFile(beadsDir string) (string, string) {
	// Check the given .beads dir itself
	routesPath := filepath.Join(beadsDir, "routes.json")
	if _, err := os.Stat(routesPath); err == nil {
		return filepath.Dir(beadsDir), routesPath
	}

	// Walk up parent directories
	dir := filepath.Dir(beadsDir) // parent of .beads
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "" // reached filesystem root
		}
		dir = parent

		candidate := filepath.Join(dir, ".beads", "routes.json")
		if _, err := os.Stat(candidate); err == nil {
			return dir, candidate
		}
	}
}
