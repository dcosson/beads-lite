// Package routing resolves issue IDs to the correct storage location
// by mapping ID prefixes to rig paths via a routes.jsonl file.
package routing

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"beads-lite/internal/config"
)

// routesFileName is the name of the routes file in a .beads directory.
const routesFileName = "routes.jsonl"

// Route maps a prefix to a rig path relative to town root.
type Route struct {
	Path string `json:"path"`
}

// routeEntry is the on-disk JSONL format: one entry per line.
type routeEntry struct {
	Prefix string `json:"prefix"`
	Path   string `json:"path"`
}

// Router resolves issue IDs to the correct storage location.
type Router struct {
	townRoot   string           // absolute path to town root
	localBeads string           // absolute path to local .beads dir
	routes     map[string]Route // prefix → route
}

// New creates a Router by discovering routes.jsonl from the given .beads dir.
// Walks up parent directories to find .beads/routes.jsonl at the town root.
// Returns nil Router (not an error) if no routes.jsonl exists or it has no routes.
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

// SameStore reports whether two issue IDs resolve to the same storage location.
// Returns true when the Router is nil (everything is local) or when both IDs
// resolve to the same data directory.
func (r *Router) SameStore(id1, id2 string) bool {
	if r == nil {
		return true
	}
	p1, _, r1, err1 := r.Resolve(id1)
	p2, _, r2, err2 := r.Resolve(id2)
	if err1 != nil || err2 != nil {
		return false
	}
	if !r1 && !r2 {
		return true // both local
	}
	if r1 != r2 {
		return false // one local, one remote
	}
	return p1.ConfigDir == p2.ConfigDir
}

// LoadRoutes reads a routes.jsonl file and returns the prefix→route map.
// Each line is a JSON object with "prefix" and "path" fields.
// Returns an empty map (not an error) if the file doesn't exist.
func LoadRoutes(path string) (map[string]Route, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]Route{}, nil
		}
		return nil, fmt.Errorf("reading routes file: %w", err)
	}
	defer f.Close()

	routes := make(map[string]Route)
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry routeEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parsing routes file line %d: %w", lineNum, err)
		}
		if entry.Prefix == "" {
			continue
		}
		routes[entry.Prefix] = Route{Path: entry.Path}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading routes file: %w", err)
	}

	return routes, nil
}

// ExtractPrefix returns everything up to and including the first hyphen.
// e.g. "bl-1jzo" -> "bl-", "hq-abc" -> "hq-".
// Returns "" if no hyphen found.
func ExtractPrefix(id string) string {
	idx := strings.IndexByte(id, '-')
	if idx < 0 {
		return ""
	}
	return id[:idx+1]
}

// findRoutesFile walks up from beadsDir looking for .beads/routes.jsonl.
// First checks the given beadsDir itself, then walks parent directories
// looking for .beads/routes.jsonl. Returns (townRoot, routesPath) or
// ("", "") if not found.
func findRoutesFile(beadsDir string) (string, string) {
	// Check the given .beads dir itself
	routesPath := filepath.Join(beadsDir, routesFileName)
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

		candidate := filepath.Join(dir, ".beads", routesFileName)
		if _, err := os.Stat(candidate); err == nil {
			return dir, candidate
		}
	}
}
