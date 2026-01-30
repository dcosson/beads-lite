package e2etests

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// knownCommands is the registry of all bd commands that should be tested.
// Subcommands use space-separated format (e.g., "dep add").
var knownCommands = map[string]bool{
	"init":         true,
	"create":       true,
	"show":         true,
	"update":       true,
	"delete":       true,
	"list":         true,
	"close":        true,
	"reopen":       true,
	"ready":        true,
	"blocked":      true,
	"search":       true,
	"stats":        true,
	"dep add":      true,
	"dep remove":   true,
	"dep list":     true,
	"children":     true,
	"comments add": true,
	"compact":          true,
	"doctor":           true,
	"config get":       true,
	"config set":       true,
	"config list":      true,
	"config unset":     true,
	"config validate":  true,
	"mol pour":         true,
	"mol wisp":         true,
	"mol current":      true,
	"mol progress":     true,
	"mol stale":        true,
	"mol burn":         true,
	"mol squash":       true,
	"mol gc":           true,
	"cook":             true,
}

// nonParentCommands are commands that do NOT have subcommands.
// Most commands have subcommands, so we list the exceptions here.
var nonParentCommands = map[string]bool{
	"mail": true,
}

// ignoredCommands are commands discovered via --help that we intentionally skip.
var ignoredCommands = map[string]bool{
	"help":       true,
	"completion": true,
}

// commandLinePattern matches "  <command>  <description>" in help output.
var commandLinePattern = regexp.MustCompile(`^\s{2}(\S+)\s{2,}`)

// validCommandPattern matches valid command names (alphanumeric, hyphens, underscores).
// Filters out emoji/symbol "commands" that appear in some help output.
var validCommandPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// DiscoverCommands runs bd --help and subcommand helps to find all available commands.
// Returns all discovered commands, ignored commands, and commands not in the knownCommands registry.
func DiscoverCommands(r *Runner) (all []string, ignored []string, unknown []string, err error) {
	discovered := make(map[string]bool)

	// Parse top-level commands from bd --help
	result := r.Run("", "--help")
	if result.ExitCode != 0 {
		return nil, nil, nil, fmt.Errorf("bd --help failed: %s", result.Stderr)
	}

	topLevel := parseCommandsFromHelp(result.Stdout)

	for _, cmd := range topLevel {
		if ignoredCommands[cmd] {
			ignored = append(ignored, cmd)
			continue
		}

		if nonParentCommands[cmd] {
			discovered[cmd] = true
		} else {
			// Try to parse subcommands
			subResult := r.Run("", cmd, "--help")
			if subResult.ExitCode != 0 {
				return nil, nil, nil, fmt.Errorf("bd %s --help failed: %s", cmd, subResult.Stderr)
			}
			subs := parseCommandsFromHelp(subResult.Stdout)
			if len(subs) > 0 {
				for _, sub := range subs {
					discovered[cmd+" "+sub] = true
				}
			} else {
				// No subcommands found, treat as a leaf command
				discovered[cmd] = true
			}
		}
	}

	// Build sorted list of all discovered commands
	for cmd := range discovered {
		all = append(all, cmd)
	}
	sort.Strings(all)
	sort.Strings(ignored)

	// Find commands not in registry
	for cmd := range discovered {
		if !knownCommands[cmd] {
			unknown = append(unknown, cmd)
		}
	}
	sort.Strings(unknown)

	return all, ignored, unknown, nil
}

// parseCommandsFromHelp extracts command names from help output.
// Handles two formats:
// 1. Cobra default: "Available Commands:" section
// 2. Original beads: Category headers like "Working With Issues:" followed by indented commands
func parseCommandsFromHelp(helpOutput string) []string {
	var commands []string
	inCommandSection := false

	for _, line := range strings.Split(helpOutput, "\n") {
		// Cobra format: "Available Commands:"
		if strings.HasPrefix(line, "Available Commands:") {
			inCommandSection = true
			continue
		}

		// Original beads format: category headers end with ":" and are not indented
		// e.g., "Working With Issues:", "Views & Reports:", etc.
		if strings.HasSuffix(strings.TrimSpace(line), ":") && !strings.HasPrefix(line, " ") {
			// Check if this looks like a category header (not "Usage:" or "Flags:")
			trimmed := strings.TrimSpace(line)
			if trimmed != "Usage:" && trimmed != "Flags:" {
				inCommandSection = true
				continue
			}
		}

		if inCommandSection {
			// "Flags:" section ends command parsing
			if strings.HasPrefix(line, "Flags:") {
				break
			}

			// Empty line in Cobra format ends section, but not in original beads
			// So we only break on empty line if we see "Available Commands:" format

			// Parse command from indented line
			matches := commandLinePattern.FindStringSubmatch(line)
			if len(matches) > 1 && validCommandPattern.MatchString(matches[1]) {
				commands = append(commands, matches[1])
			}
		}
	}

	return commands
}
