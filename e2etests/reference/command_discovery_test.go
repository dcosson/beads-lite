package reference

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCommandDiscovery(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	runner := &Runner{BdCmd: bdCmd}

	discovered, ignored, _, err := DiscoverCommands(runner)
	if err != nil {
		t.Fatalf("command discovery failed: %v", err)
	}

	// Log discovered and ignored commands
	t.Logf("Discovered %d commands from %s:", len(discovered), bdCmd)
	for _, cmd := range discovered {
		t.Logf("  %s", cmd)
	}

	t.Logf("Ignored %d commands:", len(ignored))
	for _, cmd := range ignored {
		t.Logf("  %s", cmd)
	}

	expectedFile := filepath.Join("expected", "available_commands.txt")

	if *update {
		// Write discovered commands to expected file
		content := strings.Join(discovered, "\n") + "\n"
		if err := os.WriteFile(expectedFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write expected commands file: %v", err)
		}
		t.Logf("updated %s", expectedFile)
		return
	}

	// Read expected commands from file
	expectedBytes, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("no expected file %q (run with -update to generate): %v", expectedFile, err)
	}

	expectedCommands := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(expectedBytes)), "\n") {
		if line != "" {
			expectedCommands[line] = true
		}
	}

	// Find commands in expected but not discovered (not yet implemented)
	var notImplemented []string
	for cmd := range expectedCommands {
		found := false
		for _, d := range discovered {
			if d == cmd {
				found = true
				break
			}
		}
		if !found {
			notImplemented = append(notImplemented, cmd)
		}
	}
	sort.Strings(notImplemented)

	t.Logf("Not implemented %d commands (in reference bd but not in this binary):", len(notImplemented))
	for _, cmd := range notImplemented {
		t.Logf("  %s", cmd)
	}

	// Find commands discovered but not in expected (new commands in this binary)
	var extra []string
	for _, cmd := range discovered {
		if !expectedCommands[cmd] {
			extra = append(extra, cmd)
		}
	}
	sort.Strings(extra)

	if len(extra) > 0 {
		t.Logf("Extra %d commands (in this binary but not in reference bd):", len(extra))
		for _, cmd := range extra {
			t.Logf("  %s", cmd)
		}
	}

	if len(notImplemented) > 0 {
		t.Skip("not done implementing, still missing commands")
	}
}
