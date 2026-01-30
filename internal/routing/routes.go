package routing

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// LoadRoutes reads routes.json and returns the prefix→route map.
// Returns an empty map (not an error) if the file doesn't exist.
func LoadRoutes(path string) (map[string]Route, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]Route{}, nil
		}
		return nil, fmt.Errorf("reading routes file: %w", err)
	}

	var rf RoutesFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parsing routes file: %w", err)
	}

	if rf.PrefixRoutes == nil {
		return map[string]Route{}, nil
	}
	return rf.PrefixRoutes, nil
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
