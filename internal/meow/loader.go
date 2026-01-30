package meow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// FormulaSearchPath is an ordered list of directories to search for formula
// files. Earlier entries take priority over later ones.
type FormulaSearchPath []string

// DefaultSearchPath returns a 3-tier search path (highest priority first):
//  1. .beads/formulas/ (project-level, relative to configDir)
//  2. ~/.beads/formulas/ (user-level)
//  3. $GT_ROOT/.beads/formulas/ (orchestrator-level)
func DefaultSearchPath(configDir string) FormulaSearchPath {
	var path FormulaSearchPath

	// Project-level (highest priority).
	// configDir is the .beads directory itself, so formulas/ is directly inside it.
	path = append(path, filepath.Join(configDir, "formulas"))

	// User-level
	if home, err := os.UserHomeDir(); err == nil {
		path = append(path, filepath.Join(home, ".beads", "formulas"))
	}

	// Orchestrator-level
	if gtRoot := os.Getenv("GT_ROOT"); gtRoot != "" {
		path = append(path, filepath.Join(gtRoot, ".beads", "formulas"))
	}

	return path
}

// LoadFormula searches for a formula by name across the search path.
// It looks for <name>.formula.json and <name>.formula.toml in each directory.
// The first match wins (priority order). File extension determines the parser.
func LoadFormula(name string, path FormulaSearchPath) (*Formula, error) {
	extensions := []string{".formula.json", ".formula.toml"}

	for _, dir := range path {
		for _, ext := range extensions {
			filePath := filepath.Join(dir, name+ext)
			data, err := os.ReadFile(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("reading formula %s: %w", filePath, err)
			}

			f := &Formula{}
			if strings.HasSuffix(filePath, ".formula.json") {
				if err := json.Unmarshal(data, f); err != nil {
					return nil, fmt.Errorf("parsing JSON formula %s: %w", filePath, err)
				}
			} else {
				if err := toml.Unmarshal(data, f); err != nil {
					return nil, fmt.Errorf("parsing TOML formula %s: %w", filePath, err)
				}
			}

			return f, nil
		}
	}

	return nil, fmt.Errorf("formula %q not found in search path: %s", name, strings.Join(path, ", "))
}
