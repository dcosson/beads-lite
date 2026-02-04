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

// FormulaEntry describes a formula found during a search path scan.
type FormulaEntry struct {
	Name        string      `json:"name"`
	Type        FormulaType `json:"type"`
	Phase       string      `json:"phase,omitempty"`
	Description string      `json:"description"`
	Vars        int         `json:"vars"`
	SourcePath  string      `json:"source_path"`
	Format      string      `json:"format"` // "json" or "toml"
}

// ListFormulas scans all directories in the search path and returns discovered
// formulas. Earlier directories take priority: if the same formula name appears
// in multiple directories, only the highest-priority entry is returned.
func ListFormulas(path FormulaSearchPath) ([]FormulaEntry, error) {
	seen := make(map[string]bool)
	var entries []FormulaEntry

	for _, dir := range path {
		dirEntries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading formula directory %s: %w", dir, err)
		}

		for _, de := range dirEntries {
			if de.IsDir() {
				continue
			}
			name := de.Name()
			var formulaName, format string
			if strings.HasSuffix(name, ".formula.json") {
				formulaName = strings.TrimSuffix(name, ".formula.json")
				format = "json"
			} else if strings.HasSuffix(name, ".formula.toml") {
				formulaName = strings.TrimSuffix(name, ".formula.toml")
				format = "toml"
			} else {
				continue
			}

			if seen[formulaName] {
				continue
			}
			seen[formulaName] = true

			filePath := filepath.Join(dir, name)
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("reading formula %s: %w", filePath, err)
			}

			f := &Formula{}
			if format == "json" {
				if err := json.Unmarshal(data, f); err != nil {
					return nil, fmt.Errorf("parsing JSON formula %s: %w", filePath, err)
				}
			} else {
				if err := toml.Unmarshal(data, f); err != nil {
					return nil, fmt.Errorf("parsing TOML formula %s: %w", filePath, err)
				}
			}

			entries = append(entries, FormulaEntry{
				Name:        formulaName,
				Type:        f.Type,
				Phase:       f.Phase,
				Description: f.Description,
				Vars:        len(f.Vars),
				SourcePath:  filePath,
				Format:      format,
			})
		}
	}

	return entries, nil
}

// FindFormulaFile returns the file path and format of a formula by name.
// It searches the same way as LoadFormula but returns the path instead of parsing.
func FindFormulaFile(name string, path FormulaSearchPath) (filePath, format string, err error) {
	extensions := []struct {
		suffix string
		format string
	}{
		{".formula.json", "json"},
		{".formula.toml", "toml"},
	}

	for _, dir := range path {
		for _, ext := range extensions {
			fp := filepath.Join(dir, name+ext.suffix)
			if _, statErr := os.Stat(fp); statErr == nil {
				return fp, ext.format, nil
			}
		}
	}

	return "", "", fmt.Errorf("formula %q not found in search path: %s", name, strings.Join(path, ", "))
}
