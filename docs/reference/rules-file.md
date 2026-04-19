# Rules File

The rules file decides how Limier turns low-level findings into an operator recommendation.

It is where you define what should:

- block an upgrade immediately
- require a human review
- be suppressed as known benign noise

## Example

```yaml
version: 1
hard_block:
  - id: hard-block-new-curl-fetch
    finding: new_process_exec
    message_contains: "curl "
    reason: candidate introduced a new outbound fetch command.
review:
  - id: review-new-process
    finding: new_process_exec
    reason: candidate introduced a new process and needs a human explanation.
suppress:
  - id: suppress-known-banner
    finding: step_stdout_changed
    step: exercise package
    reason: this fixture prints a known version banner that is allowed to drift.
```

## Top-Level Sections

### `version`

Required. Must be `1`.

### `hard_block`

If a finding matches a rule here, Limier recommends `block`.

Use this for behavior you want to stop immediately, such as a new fetch command or shell pipe.

### `review`

If a finding matches a rule here, Limier recommends `needs_review`.

Use this for differences that may be legitimate but still deserve a human explanation.

### `suppress`

If a finding matches a suppression rule, Limier treats that finding as known benign noise.

Use suppressions carefully. They are most useful when one specific step has repeatable, understood noise.

## Rule Fields

Each rule supports:

- `id`: required unique identifier
- `finding`: required finding kind to match
- `step`: optional step name filter
- `message_contains`: optional substring match
- `reason`: optional human-readable explanation

## Common Finding Kinds

Current findings you are most likely to write rules for include:

- `new_process_exec`
- `missing_process_exec`
- `process_exec_count_changed`
- `candidate_failed_or_diverged`
- `scenario_exit_code_changed`
- `step_exit_code_changed`
- `step_stdout_changed`
- `step_stderr_changed`
- `step_count_changed`

## Recommended Workflow

For most teams:

1. start with `rules/default.yml`
2. run Limier on real upgrades
3. add narrow suppressions only where the output is consistently noisy
4. add hard-block rules for patterns your team never wants to allow

That approach preserves signal while still letting the ruleset adapt to your environment.
