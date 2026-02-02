# Gastown Requirements for bd CLI

This document lists all `bd` commands and flags used by the Gastown project. A replacement `bd` implementation must support these to be compatible.

## Global Flags

These flags can appear before any subcommand:

| Flag | Description |
|------|-------------|
| `--no-daemon` | Run without daemon process |
| `--json` | Output in JSON format |
| `-q`, `--quiet` | Quiet mode, minimal output |
| `--allow-stale` | Allow stale/cached data |

## Commands

### Core Commands

| Command | Usage | Description |
|---------|-------|-------------|
| `version` | 1 | Return version string (checked against minimum version) |
| `init` | 12 | Initialize bead database |
| `create` | 10 | Create new beads |
| `show` | 25 | Show bead details |
| `list` | 12 | List beads/issues |
| `update` | 19 | Update bead status/properties |
| `close` | 4 | Close beads |

### `config` - Configuration Management

| Subcommand | Description |
|------------|-------------|
| `config get <key>` | Get configuration value |
| `config set <key> <value>` | Set configuration value |

Known config keys used:
- `issue_prefix`
- `allowed_prefixes`
- `types.custom`
- `routing.mode`

### `slot` - Slot Operations

| Subcommand | Description |
|------------|-------------|
| `slot show <bead-id>` | Show slot data for a bead |
| `slot set <bead-id> <name> <value>` | Set a slot value |
| `slot clear <bead-id> <name>` | Clear a slot value |

### `gate` - Gate Operations

| Subcommand | Description |
|------------|-------------|
| `gate show <gate-id>` | Show gate details |
| `gate wait <gate-id> --notify <agent-id>` | Wait on a gate with notification |

### `swarm` - Swarm Operations

| Subcommand | Description |
|------------|-------------|
| `swarm status <swarm-id>` | Get swarm status |

### `mol` - Molecule Operations

| Subcommand | Description |
|------------|-------------|
| `mol current <molecule-id>` | Get current molecule state |
| `mol seed --patrol` | Seed a molecule with patrol |
| `mol wisp create <proto-id> --actor <role>` | Create a wisp |
| `mol wisp gc` | Garbage collect wisps |

### `formula` - Formula Operations

| Subcommand | Description |
|------------|-------------|
| `formula show <name>` | Show formula details |
| `formula list` | List all formulas |

### `agent` - Agent Operations

| Subcommand | Description |
|------------|-------------|
| `agent state <bead-id> <state>` | Set agent state |
| `agent heartbeat <agent-bead>` | Send agent heartbeat |

### `dep` - Dependency Operations

| Subcommand | Description |
|------------|-------------|
| `dep list <bead-id> --direction=<up|down> --type=<type>` | List dependencies |

### Other Commands

| Command | Usage | Description |
|---------|-------|-------------|
| `cook <formula-name>` | 2 | Cook/execute a formula |
| `import` | 2 | Import data |
| `migrate` | 3 | Run database migrations |
| `sync --import-only` | 1 | Sync operations |
| `prime` | 1 | Prime operations |
| `stats` | 1 | Show statistics |
| `ready` | 1 | Ready check |
| `label` | 1 | Label operations |
| `doctor` | 1 | Health/diagnostic checks |
| `blocked` | 1 | Check blocked status |

## Common Flags by Command

### `create`
- `--type <type>` - Bead type (task, epic, etc.)
- `--title <title>` - Bead title
- `--labels <labels>` - Labels to apply
- `--priority <priority>` - Priority level

### `list`
- `--status=<status>` - Filter by status (open, in_progress, hooked, etc.)
- `--type=<type>` - Filter by type
- `--json` - JSON output

### `update`
- `--status=<status>` - New status
- `--assignee=<assignee>` - Assign to agent (empty string to unassign)

### `show`
- `--json` - JSON output

### `init`
- `--prefix <prefix>` - Issue prefix
- `--quiet` - Quiet mode

### `dep list`
- `--direction=<up|down>` - Dependency direction
- `--type=<type>` - Dependency type (e.g., "tracks")
- `--json` - JSON output

### `formula show`
- `--allow-stale` - Allow stale data

## Version Requirements

Gastown checks for minimum version `0.43.0`. The version check runs:
```
bd version
```

And expects output that can be parsed as a semver string.

## Notes

- Commands are invoked via `exec.Command("bd", ...)` throughout the codebase
- The `--no-daemon` flag is frequently used to avoid daemon process overhead
- JSON output (`--json`) is expected for programmatic parsing
- Many commands operate on bead IDs passed as positional arguments
