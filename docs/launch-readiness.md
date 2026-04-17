# Phase 7 Launch Readiness

## Product Scope

Limier is for one narrow review job: compare a baseline and candidate dependency version inside a controlled fixture, capture the behavior each version triggers, and tell a reviewer whether the change looks safe enough to approve, needs review, should be blocked, or should be rerun.

Limier is not a general application security scanner. It does not promise to detect `SQLi`, `XSS`, CSRF, authentication bugs, or broad secure-coding flaws in the fixture application.

When `capture_host_signals` is enabled, the current implementation requires Linux with `bpftrace` available. If that backend cannot start, Limier fails closed with an inconclusive rerun diagnostic instead of silently dropping process coverage.

## Repository-Owned Validation Corpus

The Phase 7 validation corpus lives under `validation/corpus/` and is enforced by `go test ./...`.

Current cases:

- `good-to-go-no-diff.yml`: routine benign upgrade with no typed diff, expected `good_to_go`
- `good-to-go-suppressed-benign-noise.yml`: benign stdout drift suppressed by a sample-specific ruleset, expected `good_to_go`
- `needs-review-new-helper-process.yml`: candidate introduces a new helper process, expected `needs_review`
- `block-network-fetch-during-install.yml`: candidate introduces a new `curl` fetch during install, expected `block`
- `rerun-unstable-candidate.yml`: candidate behavior is unstable across repeats, expected `rerun`

## Reviewer Journeys

- Platform engineer: run the sample workflow and confirm a routine upgrade stays `good_to_go`.
- Security engineer: inspect a report where a candidate introduces a new helper process and confirm the report surfaces the changed step, phase, and evidence path.
- Security or platform reviewer: inspect a blocked install-time network fetch and confirm the default rules make the hard-block reason explicit.
- CI operator: inspect an inconclusive report and use `summary.md`, `report.json`, and `limier inspect` to understand why the run should be rerun.

## Launch Checklist

- Docker lifecycle commands now capture bounded output instead of buffering unbounded create, start, and remove output in memory.
- CI examples point at real repository assets and use Go 1.26-compatible provisioning.
- The repository ships real `fixtures/`, `scenarios/`, and `rules/` directories.
- The validation corpus covers `good_to_go`, `needs_review`, `block`, and `rerun`.
- Default rules are shaped around suspicious dependency behavior, especially new process execution and install-time drift.
- Summary, render, and inspect outputs expose repeat stability and whether a finding happened during install or execution.
- Documentation states the narrow product scope directly and links to a runnable sample workflow.
