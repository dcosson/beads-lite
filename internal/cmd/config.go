package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"beads-lite/internal/config"
	"beads-lite/internal/config/yamlstore"

	"github.com/spf13/cobra"
)

// newConfigCmd creates the config command with subcommands.
func newConfigCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration settings",
		Long: `Manage beads configuration settings.

Configuration is stored as flat key-value pairs. Both core keys
(actor, defaults.priority, id.prefix, etc.) and custom keys are supported.

Subcommands:
  get       Get a configuration value
  set       Set a configuration value
  list      List all configuration values
  unset     Remove a configuration value
  validate  Validate configuration`,
	}

	cmd.AddCommand(newConfigGetCmd(provider))
	cmd.AddCommand(newConfigSetCmd(provider))
	cmd.AddCommand(newConfigListCmd(provider))
	cmd.AddCommand(newConfigUnsetCmd(provider))
	cmd.AddCommand(newConfigValidateCmd(provider))

	return cmd
}

// configStore returns the config.Store for the current app, creating it lazily
// from the resolved config directory.
func configStore(provider *AppProvider) (config.Store, error) {
	app, err := provider.Get()
	if err != nil {
		return nil, err
	}
	return yamlstore.New(filepath.Join(app.ConfigDir, "config.yaml"))
}

// newConfigGetCmd creates the "config get" subcommand.
func newConfigGetCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Get the value of a configuration key.

Prints the bare value if the key is set, or "key (not set)" if missing.

Examples:
  bd config get actor
  bd config get defaults.priority
  bd config get custom.key`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			store, err := configStore(provider)
			if err != nil {
				return err
			}

			key := args[0]
			value, ok := store.Get(key)
			if key == "actor" {
				if actor := configActorOverride(value); actor != "" {
					value = actor
					ok = true
				}
			}

			if app.JSON {
				result := map[string]interface{}{
					"key":   key,
					"value": value,
				}
				if key == "actor" {
					result["location"] = "config.yaml"
				} else if !ok {
					result["value"] = ""
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			if ok {
				fmt.Fprintln(app.Out, value)
			} else {
				fmt.Fprintf(app.Out, "%s (not set)\n", key)
			}
			return nil
		},
	}

	return cmd
}

// newConfigSetCmd creates the "config set" subcommand.
func newConfigSetCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration key to a value.

Both core keys (actor, defaults.priority, etc.) and custom keys
are supported.

Examples:
  bd config set actor alice
  bd config set defaults.priority high
  bd config set custom.key myvalue`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			store, err := configStore(provider)
			if err != nil {
				return err
			}

			key := args[0]
			value := args[1]

			if err := store.Set(key, value); err != nil {
				return fmt.Errorf("setting config: %w", err)
			}

			if app.JSON {
				result := map[string]string{
					"key":   key,
					"value": value,
				}
				if key == "actor" {
					result["location"] = "config.yaml"
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Set %s = %s\n", key, value)
			return nil
		},
	}

	return cmd
}

// newConfigListCmd creates the "config list" subcommand.
func newConfigListCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Long: `List all configuration key-value pairs.

Entries are sorted alphabetically by key.

Examples:
  bd config list
  bd config list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			store, err := configStore(provider)
			if err != nil {
				return err
			}

			all := store.All()
			for k, v := range referenceConfigDefaults(app.ConfigDir) {
				if _, exists := all[k]; !exists {
					all[k] = v
				}
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(all)
			}

			if len(all) == 0 {
				fmt.Fprintln(app.Out, "No configuration set")
				return nil
			}

			fmt.Fprintln(app.Out, "Configuration:")
			for _, k := range sortedKeys(all) {
				fmt.Fprintf(app.Out, "  %s = %s\n", k, all[k])
			}
			return nil
		},
	}

	return cmd
}

// newConfigUnsetCmd creates the "config unset" subcommand.
func newConfigUnsetCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a configuration value",
		Long: `Remove a configuration key.

The key is removed from the store regardless of whether it was set.

Examples:
  bd config unset actor
  bd config unset custom.key`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			store, err := configStore(provider)
			if err != nil {
				return err
			}

			key := args[0]

			if err := store.Unset(key); err != nil {
				return fmt.Errorf("unsetting config: %w", err)
			}

			if app.JSON {
				result := map[string]string{
					"key": key,
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Unset %s\n", key)
			return nil
		},
	}

	return cmd
}

// validPriorities is the set of valid priority values.
var validPriorities = map[string]bool{
	"critical": true,
	"high":     true,
	"medium":   true,
	"low":      true,
	"backlog":  true,
}

// validTypes is the set of valid issue type values.
var validTypes = map[string]bool{
	"task":     true,
	"bug":      true,
	"feature":  true,
	"epic":     true,
	"chore":    true,
	"gate":     true,
	"molecule": true,
}

// configValidators maps known keys to their validation functions.
// Each validator returns an error message if the value is invalid, or "" if valid.
// Validators that need access to the full config use configStoreValidators instead.
var configValidators = map[string]func(string) string{
	"create.require-description": func(v string) string {
		if v != "true" && v != "false" {
			return fmt.Sprintf("create.require-description: must be \"true\" or \"false\", got %q", v)
		}
		return ""
	},
	"defaults.priority": func(v string) string {
		if !validPriorities[v] {
			keys := sortedKeys(validPriorities)
			return fmt.Sprintf("defaults.priority: invalid value %q (valid: %s)", v, strings.Join(keys, ", "))
		}
		return ""
	},
}

// configStoreValidators maps known keys to validators that need access to the full config.
var configStoreValidators = map[string]func(string, config.Store) string{
	"defaults.type": func(v string, store config.Store) string {
		if validTypes[v] {
			return ""
		}
		// Check custom types
		if customStr, ok := store.Get("types.custom"); ok {
			for _, ct := range config.SplitCustomValues(customStr) {
				if ct == v {
					return ""
				}
			}
		}
		keys := sortedKeys(validTypes)
		if customStr, ok := store.Get("types.custom"); ok {
			keys = append(keys, config.SplitCustomValues(customStr)...)
		}
		return fmt.Sprintf("defaults.type: invalid value %q (valid: %s)", v, strings.Join(keys, ", "))
	},
}

// newConfigValidateCmd creates the "config validate" subcommand.
func newConfigValidateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		Long: `Validate the current configuration.

Checks that known keys have valid values. Unknown (custom) keys
are always accepted.

Examples:
  bd config validate
  bd config validate --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			store, err := configStore(provider)
			if err != nil {
				return err
			}

			all := store.All()
			errors := make([]string, 0)

			for key, value := range all {
				if validator, ok := configValidators[key]; ok {
					if msg := validator(value); msg != "" {
						errors = append(errors, msg)
					}
				}
				if validator, ok := configStoreValidators[key]; ok {
					if msg := validator(value, store); msg != "" {
						errors = append(errors, msg)
					}
				}
			}
			sort.Strings(errors)

			if app.JSON {
				result := map[string]interface{}{
					"valid":  len(errors) == 0,
					"issues": errors,
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			if len(errors) == 0 {
				fmt.Fprintln(app.Out, "Configuration is valid.")
				return nil
			}

			fmt.Fprintln(app.Out, "Configuration errors:")
			for _, e := range errors {
				fmt.Fprintf(app.Out, "  %s\n", e)
			}
			return fmt.Errorf("configuration has %d error(s)", len(errors))
		},
	}

	return cmd
}

func referenceConfigDefaults(configDir string) map[string]string {
	prefix := issuePrefixFromConfigDir(configDir)
	return map[string]string{
		"auto_compact_enabled":     "false",
		"compact_batch_size":       "50",
		"compact_model":            "claude-3-5-haiku-20241022",
		"compact_parallel_workers": "5",
		"compact_tier1_days":       "30",
		"compact_tier1_dep_levels": "2",
		"compact_tier2_commits":    "100",
		"compact_tier2_days":       "90",
		"compact_tier2_dep_levels": "5",
		"compaction_enabled":       "false",
		"issue_prefix":             prefix,
	}
}

func issuePrefixFromConfigDir(configDir string) string {
	parent := filepath.Dir(configDir)
	base := filepath.Base(parent)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "beads-sandbox"
	}
	return base
}

func configActorOverride(current string) string {
	if actor := strings.TrimSpace(os.Getenv(config.EnvActor)); actor != "" {
		return actor
	}
	return current
}

// sortedKeys returns the sorted keys of a map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
