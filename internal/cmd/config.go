package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"beads-lite/internal/config"

	"github.com/spf13/cobra"
)

// newConfigCmd creates the config command with subcommands.
func newConfigCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long: `View and modify beads configuration.

Subcommands:
  get       Get a configuration value
  set       Set a configuration value
  list      List all configuration values
  unset     Remove a configuration value
  validate  Validate configuration values`,
	}

	cmd.AddCommand(newConfigGetCmd(provider))
	cmd.AddCommand(newConfigSetCmd(provider))
	cmd.AddCommand(newConfigListCmd(provider))
	cmd.AddCommand(newConfigUnsetCmd(provider))
	cmd.AddCommand(newConfigValidateCmd(provider))

	return cmd
}

// newConfigGetCmd creates the "config get" subcommand.
func newConfigGetCmd(provider *AppProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			key := args[0]
			value, ok := app.ConfigStore.Get(key)

			if app.JSON {
				result := map[string]interface{}{
					"key": key,
				}
				if ok {
					result["value"] = value
				} else {
					result["set"] = false
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
}

// newConfigSetCmd creates the "config set" subcommand.
func newConfigSetCmd(provider *AppProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			key := args[0]
			value := args[1]

			if err := app.ConfigStore.Set(key, value); err != nil {
				return fmt.Errorf("setting config: %w", err)
			}

			if app.JSON {
				result := map[string]interface{}{
					"key":    key,
					"value":  value,
					"status": "set",
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Set %s = %s\n", key, value)
			return nil
		},
	}
}

// newConfigListCmd creates the "config list" subcommand.
func newConfigListCmd(provider *AppProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			all := app.ConfigStore.All()

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(all)
			}

			if len(all) == 0 {
				fmt.Fprintln(app.Out, "No configuration set")
				return nil
			}

			keys := make([]string, 0, len(all))
			for k := range all {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			fmt.Fprintln(app.Out, "Configuration:")
			for _, k := range keys {
				fmt.Fprintf(app.Out, "  %s = %s\n", k, all[k])
			}
			return nil
		},
	}
}

// newConfigUnsetCmd creates the "config unset" subcommand.
func newConfigUnsetCmd(provider *AppProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			key := args[0]

			if err := app.ConfigStore.Unset(key); err != nil {
				return fmt.Errorf("unsetting config: %w", err)
			}

			if app.JSON {
				result := map[string]interface{}{
					"key":    key,
					"status": "unset",
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Unset %s\n", key)
			return nil
		},
	}
}

// newConfigValidateCmd creates the "config validate" subcommand.
func newConfigValidateCmd(provider *AppProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration values",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			validationErr := config.Validate(app.ConfigStore)

			if app.JSON {
				result := map[string]interface{}{
					"valid": validationErr == nil,
				}
				if validationErr != nil {
					result["errors"] = parseValidationErrors(validationErr)
				}
				if encErr := json.NewEncoder(app.Out).Encode(result); encErr != nil {
					return encErr
				}
				if validationErr != nil {
					return fmt.Errorf("configuration validation failed")
				}
				return nil
			}

			if validationErr != nil {
				fmt.Fprintln(app.Out, validationErr.Error())
				return fmt.Errorf("configuration validation failed")
			}

			fmt.Fprintln(app.Out, "Configuration is valid.")
			return nil
		},
	}
}

// parseValidationErrors extracts individual error messages from a Validate error.
func parseValidationErrors(err error) []string {
	var errors []string
	for _, line := range strings.Split(err.Error(), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "config validation failed") {
			errors = append(errors, trimmed)
		}
	}
	return errors
}
