# Gas Town Components

Gas Town is the multi-agent orchestration layer built on top of beads. It adds
agent identity, work assignment, async coordination, and swarm execution to the
core issue-tracking primitives. All Gas Town concepts are implemented using
regular beads fields and custom types — there are no separate tables or storage
backends.

## Custom Types

Gas Town types were removed from beads core built-in types. They are configured
via `types.custom` in the beads config and have no built-in constants:

```
molecule, gate, convoy, merge-request, slot, agent, role, rig, event, message
```

Use string literals like `types.IssueType("gate")` when needed.

## Agents

An agent bead represents an autonomous agent in the system. Agents are regular
issues marked with the `gt:agent` label — there is no special `agent` built-in
type.

### Storage

- **Type**: `task` (with `gt:agent` label for identification)
- **Detection**: `isAgentBead()` checks for the `gt:agent` label, not the type
- **Auto-creation**: `bd agent state` auto-creates the agent bead if it doesn't
  exist (sets title, labels, role_type, rig from ID parsing)

### Agent Fields on Issue

```go
// Agent Identity Fields (agent-as-bead support)
HookBead     string     `json:"hook_bead,omitempty"`     // Current work on agent's hook (0..1)
RoleBead     string     `json:"role_bead,omitempty"`     // Role definition bead (required for agents)
AgentState   AgentState `json:"agent_state,omitempty"`   // Agent state: idle|running|stuck|stopped
LastActivity *time.Time `json:"last_activity,omitempty"` // Updated on each action (timeout detection)
RoleType     string     `json:"role_type,omitempty"`     // Agent role type (application-defined)
Rig          string     `json:"rig,omitempty"`           // Rig name (empty for town-level agents)
```

### Agent States

```go
StateIdle     = "idle"     // Waiting for work
StateSpawning = "spawning" // Starting up
StateRunning  = "running"  // Executing (general)
StateWorking  = "working"  // Actively working on a task
StateStuck    = "stuck"    // Blocked and needs help
StateDone     = "done"     // Completed current work
StateStopped  = "stopped"  // Clean shutdown
StateDead     = "dead"     // Died without clean shutdown (set by Witness via timeout)
```

### Agent ID Parsing

Role and rig are extracted from the agent ID using configured role
classifications (`agent_roles.*` in config.yaml):

- **Town-level roles** (single part after prefix): `gt-mayor`, `gt-deacon`
  - Pattern: `<prefix>-<role>` — rig is empty
- **Rig-level roles** (rig + role): `gastown-witness`, `gastown-refinery`
  - Pattern: `<prefix>-<rig>-<role>`
- **Named roles** (rig + role + name): `gastown-crew-alice`
  - Pattern: `<prefix>-<rig>-<role>-<name>`

Parsing scans from the right to find a known role, allowing rig names to
contain hyphens (e.g., `my-project-witness`).

### CLI Commands

```
bd agent state <agent> <state>   # Set state + update last_activity
bd agent heartbeat <agent>       # Update only last_activity (no state change)
bd agent show <agent>            # Show agent details (state, slots, identity)
bd agent backfill-labels         # Add role_type:/rig: labels to existing agents
```

## Slots

Slots are named attachment points on agent beads that hold references to other
beads with cardinality constraints.

### Valid Slots

There are exactly two slots:

- **hook** — Current work attached to the agent (0..1 cardinality). Writing to
  hook when occupied returns an error; use `bd slot clear` first.
- **role** — Role definition bead (required for agents). No cardinality
  enforcement.

### Storage

Slots are stored as regular fields on the Issue struct (`HookBead`, `RoleBead`).
They are not a separate table.

### The `hooked` Status

There is a special status `StatusHooked = "hooked"` for issues attached to an
agent's hook. This is part of the GUPP (Guaranteed Unambiguous Progress
Protocol) pattern.

### CLI Commands

```
bd slot set <agent> <slot> <bead>   # Attach bead to slot (enforces cardinality)
bd slot clear <agent> <slot>        # Detach bead from slot
bd slot show <agent>                # Show all slot values
```

Note: `bd slot set/clear` checks `agent.IssueType != "agent"` for validation
(not the `gt:agent` label). This is a slight inconsistency with `bd agent`
commands which use the label.

## Heartbeat

The heartbeat mechanism tracks agent liveness via the `LastActivity` timestamp.

### How It Works

- `bd agent state` updates both `agent_state` and `last_activity`
- `bd agent heartbeat` updates only `last_activity` (no state change)
- The Witness patrol monitors `last_activity` timestamps across agents
- If `now - last_activity > timeout_threshold`, the agent is marked `dead`

### Purpose

Part of ZFC (Zombie-Free Computation) compliance. Enables timeout-based failure
detection without explicit polling — the Witness observes timestamps passively
and acts when they go stale.

## Gates

Gates are async wait conditions that block workflow steps until external
conditions are satisfied. They are issues with `type=gate` (a custom type).

### Gate Fields on Issue

```go
// Gate Fields (async coordination primitives)
AwaitType string        `json:"await_type,omitempty"` // Condition type
AwaitID   string       `json:"await_id,omitempty"`   // Condition identifier
Timeout   time.Duration `json:"timeout,omitempty"`    // Max wait time before escalation
Waiters   []string      `json:"waiters,omitempty"`    // Addresses to notify when gate clears
```

### Gate Types (Phased)

| Phase | Type    | Condition                    | Resolution                        |
|-------|---------|------------------------------|-----------------------------------|
| 1     | `human` | Manual closure required       | `bd gate resolve` or `bd close`   |
| 2     | `timer` | Time-based expiration         | Auto-resolve when `created_at + timeout < now` |
| 3     | `gh:run`| GitHub Actions workflow       | Auto-resolve when `status=completed, conclusion=success` |
| 3     | `gh:pr` | Pull request merge            | Auto-resolve when `state=MERGED`  |
| 4     | `bead`  | Cross-rig bead closure        | Auto-resolve when target bead is `closed` |

### Gate Lifecycle

1. Gates are created automatically when a formula step has a `[steps.gate]`
   section
2. The gate blocks dependent steps (via `DepBlocks` dependency)
3. Gate ID format: `<mol>.<step>.gate-<stepid>`
4. Gates are resolved manually or by `bd gate check`
5. Failed gates can be escalated (via `gt escalate`)

### Escalation

A gate is escalated (not resolved) when:
- `gh:run`: conclusion is `failure` or `canceled`
- `gh:pr`: state is `CLOSED` without merge
- Escalation calls `gt escalate` with HIGH severity

### Workflow Name Discovery

For `gh:run` gates, `await_id` can be a workflow filename (e.g.,
`release.yml`) instead of a numeric run ID. `bd gate check` auto-discovers
the most recent run for that workflow and updates the gate's `await_id` with
the discovered numeric ID.

### CLI Commands

```
bd gate list [--all]                    # Show open gates (--all includes closed)
bd gate show <gate-id>                  # Show gate details including waiters
bd gate resolve <gate-id> [--reason]    # Manually close a gate
bd gate check [--type=<type>]           # Auto-evaluate and close resolved gates
bd gate check --dry-run                 # Preview without changes
bd gate check --escalate                # Escalate failed/expired gates
bd gate add-waiter <gate-id> <address>  # Register for wake notification
```

## Swarm

A swarm coordinates parallel work on an epic's children. It analyzes the
dependency DAG to find waves of parallelizable work ("ready fronts").

### How It Works

1. **Validate**: `bd swarm validate` analyzes an epic's children and their
   `DepBlocks` dependencies for cycles, orphans, and disconnected subgraphs
2. **Ready fronts**: A topological sort groups children into waves — wave 0 has
   no dependencies, wave 1 depends only on wave 0 issues, etc.
3. **Create**: `bd swarm create` creates a molecule issue with `MolType=swarm`
   linked to the epic via `DepRelatesTo`
4. **Status**: `bd swarm status` computes progress from beads (not stored
   separately) — categorizes children as completed, active, ready, or blocked

### Key Types

```go
MolType = "swarm"  // Molecule type for swarm coordination

type SwarmAnalysis struct {
    EpicID          string       // Target epic
    TotalIssues     int          // Child count
    ReadyFronts     []ReadyFront // Waves of parallel work
    MaxParallelism  int          // Max concurrent workers
    EstimatedSessions int        // Total issues (each ≈ 1 session)
    Swarmable       bool         // No errors = swarmable
}

type ReadyFront struct {
    Wave   int      // Wave number (0-based)
    Issues []string // Issue IDs ready in this wave
}
```

### Swarmability Requirements

- No dependency cycles
- Valid DAG structure
- Epic must have children
- Warnings (non-blocking): external dependencies, disconnected subgraphs

### CLI Commands

```
bd swarm validate <epic-id> [--verbose]           # Analyze structure
bd swarm create <epic-id> [--coordinator=<addr>]  # Create swarm molecule
bd swarm status <epic-or-swarm-id>                # Show progress
bd swarm list                                     # List active swarms
```

## Seed Patrol

The seed command verifies that patrol formulas are accessible before patrols
attempt to spawn work. It's a health check, not a creation command.

### Patrol Formulas

Three patrol formulas must be accessible for the system to function:

1. **`mol-deacon-patrol`** — Dispatches gate-ready molecules (the Deacon finds
   molecules whose gates have cleared and resumes them)
2. **`mol-witness-patrol`** — Monitors agent health (detects dead agents via
   heartbeat timeout, ZFC compliance)
3. **`mol-refinery-patrol`** — Validates/refines completed work (quality
   assurance)

### What Seed Does

`bd mol seed --patrol` loads each patrol formula through the full resolution
pipeline (search path, syntax, extends, cook) and reports success or failure.
It does not create issues or molecules.

### CLI Commands

```
bd mol seed --patrol                   # Verify all 3 patrol formulas
bd mol seed <formula-name>             # Verify a specific formula
bd mol seed <formula> --var key=value  # Verify with variable substitution
```

## Merge-Slot

A merge slot is an exclusive-access primitive for serialized conflict
resolution. It prevents multiple agents from racing to resolve merge conflicts
simultaneously ("monkey knife fights").

### How It Works

- **One per rig**: ID format `<prefix>-merge-slot`, labeled `gt:slot`
- **Mutex semantics**: Only one agent can hold the slot at a time
- **Status-based**: `open` = available, `in_progress` = held
- **Queue**: Waiters list provides priority-ordered access

### Slot Fields on Issue

```go
Holder string `json:"holder,omitempty"` // Who currently holds the slot
```

Combined with the `Waiters` field from gate support for the queue.

### CLI Commands

```
bd merge-slot create    # Create slot bead for current rig
bd merge-slot check     # Check availability (available / held by <holder>)
bd merge-slot acquire   # Try to acquire (fails if held; use --wait to queue)
bd merge-slot release   # Release after work complete
bd merge-slot wait      # Wait for slot to become available
```

## Molecule Types

The `MolType` field on Issue categorizes molecules for different coordination
patterns:

```go
MolTypeSwarm  = "swarm"  // Coordinated multi-agent work on an epic
MolTypePatrol = "patrol" // Recurring operational work (Witness, Deacon, Refinery)
MolTypeWork   = "work"   // Regular formula instance (default if empty)
```

`MolType` is filterable via `IssueFilter.MolType` and `WorkFilter.MolType`.

## Issue Fields Summary

All Gas Town fields are regular fields on the Issue struct. No separate tables.

| Field | Type | Used By | Purpose |
|-------|------|---------|---------|
| `HookBead` | string | Agents/Slots | Current work on agent's hook |
| `RoleBead` | string | Agents/Slots | Role definition bead |
| `AgentState` | AgentState | Agents | Self-reported state |
| `LastActivity` | *time.Time | Agents/Heartbeat | Liveness timestamp |
| `RoleType` | string | Agents | Role classification |
| `Rig` | string | Agents | Rig membership |
| `MolType` | MolType | Molecules | Molecule classification |
| `AwaitType` | string | Gates | Condition type |
| `AwaitID` | string | Gates | Condition identifier |
| `Timeout` | time.Duration | Gates | Max wait time |
| `Waiters` | []string | Gates/Merge-Slot | Notification queue |
| `Holder` | string | Merge-Slot | Current holder |
| `EventKind` | string | Events | Namespaced event type |
