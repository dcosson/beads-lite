# 01 Workflow Digest Test Harness

Status: Draft

## Scope

This test harness covers the workflow evidence model, close/squash validation,
digest compilation, and the predefined acceptance-criteria lifecycle for the
workflow digest plan.

## Property-Based Tests

- Event append-only invariant
  - Once an event is written, later updates may append but never mutate or
    remove prior events.
- Acceptance immutability invariant
  - Once implementation has started, acceptance criteria definitions cannot
    change without an explicit amendment event.
- Digest determinism invariant
  - Given the same issue graph and workflow event history, digest compilation
    always produces byte-equivalent structured output.

## Fault Injection / Chaos

- Missing root acceptance definition during squash
- Missing required task event during close
- Missing reviewer/session metadata on a review event
- Partial digest rendering failure after successful structured compilation
- Corrupt workflow event payload on one child task

Each case must fail clearly and preserve the existing workflow data without
silently generating a degraded digest.

## Comparison / Oracle Tests

- Oracle: compile the same synthetic workflow history twice and assert identical
  structured digest output
- Oracle: compare rendered markdown against a golden rendering generated from a
  fixed digest JSON fixture

## Deterministic Simulation

Build synthetic workflow runs for:

1. planning-only workflow
2. implementation workflow with one commit and one review
3. implementation workflow with multiple commits, multiple follow-ups, and
   multiple validation runs
4. failed workflow with missing acceptance evidence

For each simulation:

- append events in order
- attempt close transitions
- attempt squash
- verify expected pass/fail behavior

## Benchmarks

- compile digest for 10, 100, and 1000 workflow events
- render markdown for equivalent digest sizes
- validate close/squash readiness for equivalent workflow sizes

Targets:

- close validation should remain effectively instant for typical workflows
- digest compilation should scale linearly with event count

## Stress / Soak Tests

- long-running append-heavy workflow histories on a single task
- repeated squash attempts on incomplete workflows
- repeated render operations from the same digest object

## Security / Robustness Tests

- shell-command text in acceptance criteria must be preserved exactly and not
  reinterpreted or mutated by renderers
- malformed event payloads must not panic the CLI
- validation errors must be descriptive but not leak unrelated issue internals

## Manual QA

1. Run a small workflow wisp manually and append realistic execution events.
2. Confirm `bd show` on a child task displays human-readable context while the
   underlying structured event list remains intact.
3. Attempt to close a step missing required evidence and confirm the error is
   actionable.
4. Attempt to squash a workflow missing acceptance criteria and confirm squash
   fails with a clear explanation.
5. Complete a fully valid workflow and inspect the digest in both terminal and
   markdown form.

## CI Tier Mapping

- Fast CI
  - schema validation unit tests
  - close/squash command validation tests
  - digest compilation deterministic tests
- Medium CI
  - renderer goldens
  - synthetic end-to-end workflow tests
- Slow CI / nightly
  - large-history benchmarks
  - stress and soak runs

## Exit Criteria

Implementation is not complete until all of the following hold:

- required workflow event validation is enforced at close time
- squash refuses incomplete workflow runs
- acceptance criteria are predefined before implementation completion
- structured digest JSON is stored on the digest issue
- markdown and terminal rendering both derive from the structured digest
- targeted command tests and end-to-end workflow tests pass
