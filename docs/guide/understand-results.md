# Understand Results

Limier produces a few different outputs on purpose. Each one serves a different audience.

## The Main Output Files

After `limier run`, you will usually have:

- `report.json`: the full structured result
- `summary.md`: a short human-readable summary
- `evidence/`: raw stdout, stderr, and event evidence for each run and step

You may also generate additional rendered views with `limier render`.

## What To Read First

If you only have a minute:

1. open `summary.md`
2. look at the operator recommendation
3. open the evidence bundle if the recommendation is not `good_to_go`

## Operator Recommendation

This is the decision-oriented outcome most users care about:

- `good_to_go`: approve if this matches your expectations
- `needs_review`: inspect the change before approving
- `block`: treat the change as suspicious
- `rerun`: the run was inconclusive or unstable

## Technical Verdict

Limier also records a lower-level technical verdict in the report:

- `no_diff`
- `expected_diff`
- `unexpected_diff`
- `suspicious_diff`
- `inconclusive`

If you are triaging quickly, the operator recommendation is usually the more important field.

## Exit Codes

Limier uses simple exit codes so CI can react without parsing JSON:

- `0`: `good_to_go`
- `1`: `needs_review` or `block`
- `2`: `rerun` or another inconclusive result

## Typical Review Flow

When a result is not `good_to_go`, a practical flow is:

1. read the summary
2. note which step changed
3. look at the matching stdout and stderr files in `evidence/`
4. inspect any recorded process or event evidence
5. decide whether the change is expected, suspicious, or just noisy

## Re-Explain An Existing Report

Use `inspect` when you already have `report.json` and want a compact explanation without rerunning the fixture:

```sh
limier inspect --input out/limier/report.json
```

That is especially useful for inconclusive runs because it highlights any structured diagnostic and suggested next action.

## Render For CI Surfaces

Use `render` when you want to reuse the same report in another surface:

```sh
limier render --format build-summary --input out/limier/report.json
limier render --format github-comment --input out/limier/report.json
limier render --format gitlab-note --input out/limier/report.json
```

Supported formats today are:

- `build-summary`
- `github-comment`
- `gitlab-note`

## What `rerun` Usually Means

`rerun` does not mean “safe.” It means Limier could not produce a trustworthy comparison.

Common causes include:

- Docker problems on the runner
- non-deterministic fixture behavior
- missing Linux host-signal support when `capture_host_signals` is enabled
- scenario commands that fail before the review is complete

In those cases, fix the environment or scenario first, then run the review again.
