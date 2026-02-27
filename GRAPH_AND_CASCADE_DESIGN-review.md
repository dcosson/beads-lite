# Design Review: GRAPH_AND_CASCADE_DESIGN.md

## High-Severity Findings

1. **Cascade wave modeling is internally inconsistent and can produce wrong readiness ordering**
- Where: `TopologicalWavesAcrossParents` section (`GRAPH_AND_CASCADE_DESIGN.md:275-313`) and “Why Leaf Tasks?” (`:345-351`).
- Problem: The algorithm says “leaf tasks” are nodes with no **children** (`:282`), but later reasons about nodes that no other task depends on (dependency-graph sinks). Those are different concepts.
- Risk: If implemented as written, synthetic edges will often be too strict or too weak depending on hierarchy shape, and wave output will not match intended parent-level semantics.
- Fix: Define one exact concept (recommended: dependency-graph sinks within the blocker subtree), include deterministic sink-selection logic, and add tests that distinguish hierarchy leaves vs dependency sinks.

2. **Parent-level blocker can be silently ignored when blocker is not itself a parent**
- Where: cross-parent algorithm (`:297-305`).
- Problem: The design assumes `blockerID` is a parent and then looks up leaf tasks under it. But parent issues can be blocked by a regular task (`DepTypeBlocks` allows this in current model). In that case, “leafTasksUnder(blockerID)” is empty and the block can disappear.
- Risk: False-ready tasks and incorrect wave planning under valid dependency graphs.
- Fix: Define blocker normalization:
  - If blocker is a parent, use its sink tasks.
  - If blocker is a leaf/task, use that task directly.
  - If blocker has descendants, specify inclusion rules explicitly.

3. **`bd close` JSON contract changes are underspecified and likely breaking**
- Where: auto-close integration (`:233`) and close command section (`:548`).
- Problem: Current `close --json` has two output shapes (`[]IssueJSON` and `{closed, continue}`). Adding `auto_closed` “to the response” does not define where it lives for both variants or compatibility guarantees.
- Risk: Client breakage for automation that parses existing JSON.
- Fix: Specify exact schemas for:
  - `bd close --json`
  - `bd close --json --continue`
  - `bd close --json --suggest-next`
  - combinations of those flags
  Include backward-compatibility policy or versioning strategy.

4. **Auto-close recursion stop condition can miss legitimate ancestor closures**
- Where: `AutoCloseAncestors` algorithm (`:203-204`).
- Problem: It stops when encountering an already-closed parent. That prevents checking whether the next ancestor now qualifies for auto-close.
- Risk: Inconsistent hierarchy state where a grandparent remains open even when all children are closed.
- Fix: On already-closed parent, continue traversal upward (do not re-close the node, but continue evaluating ancestors).

## Medium-Severity Findings

5. **Error-handling strategy for inherited blocker lookup is not defined**
- Where: `EffectiveBlockers` algorithm (`:138-149`) and usage across `ready/blocked/show/swarm`.
- Problem: Multiple commands will call `store.Get` up parent chains. The doc does not define behavior for missing/corrupt ancestors or routing misses.
- Risk: One malformed issue can fail whole-list commands (`bd ready`, `bd blocked`, `bd graph`).
- Fix: Define policy explicitly:
  - strict mode (fail command), or
  - resilient mode (annotate issue as “unknown inherited blocker state” and continue).

6. **Config integration is incomplete for this repo’s validation/test model**
- Where: config section (`:477`) and implementation order (`:711`).
- Problem: In this codebase, new defaults usually require synchronized updates to config validation/docs/tests. The design only mentions `defaults.go`.
- Risk: Partial implementation that compiles but fails config-related tests or user validation flows.
- Fix: Add explicit tasks for `internal/config/validate.go`, config command key validators (if needed), and `internal/config/config_test.go` updates.

7. **Command/file references in doc do not match repository layout**
- Where: multiple sections reference `cmd/bd/*.go` paths.
- Problem: Actual command files are in `internal/cmd/*.go`.
- Risk: Implementation churn/confusion, especially for reviewers or parallel implementers.
- Fix: Update all file paths and anchor to current symbols (`newReadyCmd`, `getWaitingOn`, `findUnblockedDependents`, etc.).

## Low-Severity Findings

8. **One edge case claim is contradictory with model constraints**
- Where: “Parent has no children won’t auto-close” (`:241`).
- Problem: In this model, “parent” is functionally determined by having children; a zero-child node is not meaningfully a parent in runtime behavior.
- Risk: Minor confusion for implementers/test authors.
- Fix: Rephrase as “if an ancestor currently has zero resolved child links, skip auto-close and continue traversal” or remove the case.

## Open Questions

1. Should inherited blockers only affect readiness/waves, or also mutate displayed issue status to `blocked`?
- The rendering examples use blocked iconography for inherited blocks, while current status semantics are user-controlled.

2. Should auto-close apply to all issue types including `molecule` and `gate`?
- The doc says yes for all types, but this may conflict with operational workflows where parent lifecycle is manually controlled.

3. For `bd graph` global mode, should grouping key be immediate parent only, or top-most ancestor/root molecule?
- Current examples imply immediate parent groups but cross-parent ordering implies higher-level grouping concerns.

## Recommended Design Amendments Before Implementation

1. Add a precise formal definition of “blocking frontier” (the set of blocker-side tasks required to complete a parent-level block).
2. Add a strict JSON schema section for all modified command outputs with flag combinations.
3. Add failure-mode semantics (`Get` errors, cycles, missing references) for each command callsite.
4. Add compatibility section listing unchanged behaviors and intentionally changed behaviors.
5. Update implementation plan with concrete touched files in this repo (`internal/cmd`, `internal/graph`, `internal/config`, tests).
