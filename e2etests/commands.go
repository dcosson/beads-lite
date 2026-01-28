package e2etests

import (
	"fmt"
	"regexp"
	"strings"
)

// knownCommands is the registry of all bd commands that should be tested.
// Subcommands use space-separated format (e.g., "dep add").
var knownCommands = map[string]bool{
	"init":           true,
	"create":         true,
	"show":           true,
	"update":         true,
	"delete":         true,
	"list":           true,
	"close":          true,
	"reopen":         true,
	"ready":          true,
	"blocked":        true,
	"search":         true,
	"stats":          true,
	"dep add":        true,
	"dep remove":     true,
	"dep list":       true,
	"children":       true,
	"comment add":    true,
	"comment list":   true,
	"compact":        true,
	"doctor":         true,
}

// parentCommands are commands that have subcommands.
// We need to run --help on these to discover their subcommands.
var parentCommands = map[string]bool{
	"dep":     true,
	"comment": true,
}

// ignoredCommands are commands discovered via --help that we intentionally skip.
var ignoredCommands = map[string]bool{
	"help":       true,
	"completion": true,
}

// commandLinePattern matches "  <command>  <description>" in help output.
var commandLinePattern = regexp.MustCompile(`^\s{2}(\S+)\s{2,}`)

// DiscoverCommands runs bd --help and subcommand helps to find all available commands.
// Returns a list of commands not in the knownCommands registry.
func DiscoverCommands(r *Runner) (unknown []string, err error) {
	discovered := make(map[string]bool)

	// Parse top-level commands from bd --help
	result := r.Run("", "--help")
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("bd --help failed: %s", result.Stderr)
	}

	topLevel := parseCommandsFromHelp(result.Stdout)

	for _, cmd := range topLevel {
		if ignoredCommands[cmd] {
			continue
		}

		if parentCommands[cmd] {
			// Parse subcommands
			subResult := r.Run("", cmd, "--help")
			if subResult.ExitCode != 0 {
				return nil, fmt.Errorf("bd %s --help failed: %s", cmd, subResult.Stderr)
			}
			subs := parseCommandsFromHelp(subResult.Stdout)
			for _, sub := range subs {
				discovered[cmd+" "+sub] = true
			}
		} else {
			discovered[cmd] = true
		}
	}

	// Find commands not in registry
	for cmd := range discovered {
		if !knownCommands[cmd] {
			unknown = append(unknown, cmd)
		}
	}

	return unknown, nil
}

// parseCommandsFromHelp extracts command names from help output.
// It looks for the "Available Commands:" section and parses command names.
func parseCommandsFromHelp(helpOutput string) []string {
	var commands []string
	inCommandSection := false

	for _, line := range strings.Split(helpOutput, "\n") {
		if strings.HasPrefix(line, "Available Commands:") {
			inCommandSection = true
			continue
		}

		if inCommandSection {
			// Empty line or "Flags:" ends the section
			if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "Flags:") {
				break
			}

			matches := commandLinePattern.FindStringSubmatch(line)
			if len(matches) > 1 {
				commands = append(commands, matches[1])
			}
		}
	}

	return commands
}
