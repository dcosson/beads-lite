# Shaping: Detailed Workflow Digest

## Problem

The current formula-driven workflow steps are reasonably clear, but the final
digest produced by `mol squash` is too shallow. It preserves task titles, but
throws away the most valuable execution evidence:

- Who did each step
- Which session they were in
- What changed status and when
- What findings were raised during review
- What follow-up work was created
- Which commits implemented the work
- Which tests and commands actually verified the outcome
- Whether the final acceptance criteria were explicitly defined and passed

The goal is to preserve the strong "clear steps to follow" experience while
ending with a very detailed, auditable digest.

## Requirements

| Req | Requirement | Status |
| --- | ----------- | ------ |
| R0 | The workflow must still present explicit, easy-to-follow steps for planning, review, implementation, signoff, wiring review, and final validation. | Core goal |
| R1 | Planning and review steps must capture which agent did the work, including agent name and session ID. | Must-have |
| R2 | Review-oriented steps must preserve most of the signal from disposition tables, including findings, dispositions, rationale, and status updates. | Must-have |
| R3 | The digest must preserve status transitions clearly, not just final closed/open state. | Must-have |
| R4 | At the end of a review loop, the digest must preserve convergence data by round, including issue counts and summary trend. | Must-have |
| R5 | Implementation steps must preserve commit-level detail: implementing commit, follow-up commit, reviewer notes, judgement calls, challenges, and cleanup items. | Must-have |
| R6 | The digest must capture which tests best verify each implementation step, not just generic "tests passed" language. | Must-have |
| R7 | Final acceptance criteria must be objective, predeclared, and command-based. The digest must show the exact commands run, who ran them, their session ID, and their explicit signoff. | Must-have |
| R8 | The digest should be generated automatically enough that agents do not need to hand-write a long final summary from scratch. | Must-have |
| R9 | The design should work for both planning-heavy workflows and implementation-heavy workflows without inventing a completely separate system for each. | Must-have |

## A: Digest Only From Final Issue State

| Part | Mechanism |
| ---- | --------- |
| A1 | Keep current workflow tasks as-is |
| A2 | Improve `mol squash` formatting only |
| A3 | Derive digest solely from final issue titles, descriptions, comments, and close metadata |

### Strengths

- Minimal new machinery
- Easy to ship

### Weaknesses

- Important data is missing if it was never written during execution
- Hard to preserve structured review and commit history
- Weak support for exact acceptance criteria

## B: Structured Execution Journal Per Task

| Part | Mechanism |
| ---- | --------- |
| B1 | Keep current explicit workflow steps |
| B2 | Define a structured comment or metadata schema per task step |
| B3 | Require agents to append machine-parseable execution journal entries as they work |
| B4 | Teach `mol squash` to compile those execution journal entries into a rich digest |

### Strengths

- Preserves current step-by-step UX
- Captures data at the moment it happens
- Strong fit for detailed final digest
- Can preserve both human narrative and structured fields

### Weaknesses

- Requires discipline in step updates
- Comment schema must be well-defined

## C: External Run Ledger Plus Squash Compiler

| Part | Mechanism |
| ---- | --------- |
| C1 | Keep workflow tasks mostly human-readable |
| C2 | Write a separate run ledger file for each workflow execution |
| C3 | Task comments link into ledger entries instead of carrying the full payload |
| C4 | `mol squash` compiles the ledger into the digest |

### Strengths

- Strongest structure and easiest to query
- Best for very detailed histories

### Weaknesses

- Introduces a second artifact system beside beads
- More coordination complexity
- Harder to inspect casually from issue state alone

## Fit Check

| Req | Requirement | Status | A | B | C |
| --- | ----------- | ------ | - | - | - |
| R0 | The workflow must still present explicit, easy-to-follow steps for planning, review, implementation, signoff, wiring review, and final validation. | Core goal | ✅ | ✅ | ✅ |
| R1 | Planning and review steps must capture which agent did the work, including agent name and session ID. | Must-have | ❌ | ✅ | ✅ |
| R2 | Review-oriented steps must preserve most of the signal from disposition tables, including findings, dispositions, rationale, and status updates. | Must-have | ❌ | ✅ | ✅ |
| R3 | The digest must preserve status transitions clearly, not just final closed/open state. | Must-have | ❌ | ✅ | ✅ |
| R4 | At the end of a review loop, the digest must preserve convergence data by round, including issue counts and summary trend. | Must-have | ❌ | ✅ | ✅ |
| R5 | Implementation steps must preserve commit-level detail: implementing commit, follow-up commit, reviewer notes, judgement calls, challenges, and cleanup items. | Must-have | ❌ | ✅ | ✅ |
| R6 | The digest must capture which tests best verify each implementation step, not just generic "tests passed" language. | Must-have | ❌ | ✅ | ✅ |
| R7 | Final acceptance criteria must be objective, predeclared, and command-based. The digest must show the exact commands run, who ran them, their session ID, and their explicit signoff. | Must-have | ❌ | ✅ | ✅ |
| R8 | The digest should be generated automatically enough that agents do not need to hand-write a long final summary from scratch. | Must-have | ⚠️ | ✅ | ✅ |
| R9 | The design should work for both planning-heavy workflows and implementation-heavy workflows without inventing a completely separate system for each. | Must-have | ❌ | ✅ | ⚠️ |

**Notes:**

- A fails almost every high-signal requirement because the current issue state does not contain enough structured history.
- C can satisfy the requirements, but it introduces a second artifact system and makes the workflow heavier than necessary.
- B is the best fit because it keeps the existing task-driven process while making the final digest compile from structured task evidence.

## Selected Shape

## B: Structured Execution Journal Per Task

| Part | Mechanism |
| ---- | --------- |
| B1 | Keep the current workflow formulas and explicit steps. |
| B2 | Add a required structured execution journal format that agents append during each task. |
| B3 | Distinguish journal entry kinds by workflow phase: planning, review, implementation, signoff, wiring review, acceptance. |
| B4 | Update `mol squash` to read and compile those journal entries into a rich digest with fixed sections. |
| B5 | Add explicit acceptance-criteria definition as its own first-class step artifact before implementation begins. |

## B2: Journal Entry Schema

Each workflow task should accumulate structured execution entries. These can
live in comments, but the comment body should follow a machine-parseable shape.

### Core fields for every entry

- `entry_type`
- `step_id`
- `agent_name`
- `session_id`
- `timestamp`
- `status_before`
- `status_after`
- `summary`

### Planning / review entries

- `plan_docs`
- `reviewer`
- `finding_count`
- `findings_by_severity`
- `disposition_table_refs`
- `open_questions`
- `convergence_round`
- `convergence_delta`

### Implementation entries

- `commit`
- `review_commit` or `review_ref`
- `followup_commit`
- `judgement_calls`
- `challenges`
- `cleanup_later`
- `best_tests`

### Acceptance / validation entries

- `acceptance_criteria_id`
- `command`
- `result`
- `artifacts`
- `signoff_statement`

## B3: Per-Phase Expectations

### Planning / plan review

For plan-writing and review loop tasks, journal entries should preserve:

- agent name and session ID for each writer and reviewer
- links or references to the plan docs involved
- disposition-table-derived summaries
- explicit status changes
- convergence data per round

### Seam review

For seam review tasks, journal entries should preserve:

- reviewer identity
- seam or slice reviewed
- mismatches found
- affected docs
- whether the seam re-entered the review loop
- final seam summary

### Implementation

For implementation tasks, journal entries should preserve:

- implementing agent and session ID
- commit hash for the implementation change
- short judgement-call summary
- challenges or uncertainty encountered
- cleanup items noticed but not handled
- best tests for that work
- reviewer identity and notes
- follow-up commit hashes after review

### Final signoff

For plan-work-completion-signoff and wiring review tasks, journal entries should
preserve:

- verifier identity
- signoff status
- deviations or contractual mismatches found
- follow-up beads created
- re-verification outcome after follow-up completion

## B4: Digest Shape

The squash digest should stop being a list of task titles and instead render a
fixed, high-signal report.

### Proposed digest sections

1. `Workflow Summary`
2. `Planning and Review History`
3. `Convergence Summary`
4. `Seam Review Summary`
5. `Implementation Timeline`
6. `Review Follow-ups`
7. `Plan Completion Signoff`
8. `Wiring Review`
9. `Acceptance Criteria`
10. `Outstanding Cleanup`

### Example section content

#### Workflow Summary

- workflow name
- root issue ID
- scope
- start and finish timestamps
- total agents involved

#### Planning and Review History

- step-by-step table or bullet log
- who wrote or reviewed which plan
- session IDs
- key findings
- disposition references

#### Convergence Summary

- Round 1: total findings
- Round 2: total findings
- Round N: total findings
- trend note

#### Implementation Timeline

For each implementation task or commit cluster:

- commit hash
- implementing agent + session ID
- reviewer + session ID
- review summary
- follow-up commit hash
- best verifying tests
- judgement calls
- cleanup-later notes

#### Acceptance Criteria

This should be the most rigid section in the digest:

- criteria ID
- exact command
- expected result
- actual result
- agent name
- session ID
- signoff statement

## B5: Acceptance Criteria as a First-Class Artifact

This is the most important requirement from your notes.

Acceptance criteria should be defined before implementation work is considered
complete. They should not be vague. They should be a list of exact commands and
expected outcomes.

### Proposed acceptance artifact

Each workflow should carry or reference an acceptance block with entries like:

| ID | Purpose | Command | Expected Result |
| -- | ------- | ------- | --------------- |
| AC1 | Plan review summary is up to date | `python3 scripts/check-review-summary.py docs/plans` | exits 0 |
| AC2 | Implementation work passes targeted tests | `go test ./internal/cmd -run TestCloseContinueAdvancesNextStepForEphemeralMolecule` | exits 0 |
| AC3 | Full workflow wiring is intact | `make test-e2e-all` | exits 0 |

Then the final validating agent records:

- agent name
- session ID
- exact command run
- timestamp
- pass/fail
- explicit signoff text

## Open Questions

1. Should the execution journal live entirely in comments, or should comments
   point to a structured payload attached elsewhere in the issue JSON?
2. Should squash render the digest as markdown in the issue description, or
   store structured digest fields and render on `bd show`?
3. Do we want one acceptance artifact per workflow formula, or one per specific
   workflow execution?

## Recommendation

Proceed with Shape B.

It preserves the strong current step-by-step workflow, but upgrades every step
into structured evidence that can be compiled into a detailed digest. The key
design move is to treat the workflow as an execution journal, not just a set of
tasks that later get summarized.
