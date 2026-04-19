---
name: limier-cli
description: Use when working with the Limier CLI in this repository: running dependency behavior reviews, explaining existing Limier reports, rendering CI summaries, diagnosing `rerun` outcomes, or editing Limier scenario and rules files.
---

# Limier CLI

## Overview

Limier reviews one dependency upgrade at a time by running a package change inside a controlled fixture and comparing the resulting behavior. It produces an operator-facing recommendation:

- `good_to_go`
- `needs_review`
- `block`
- `rerun`

When reporting results, lead with the operator recommendation and the next action. Mention the lower-level technical verdict only when it adds useful detail.

## When To Use This Skill

Use this skill when the user wants to:

- run `limier` against a dependency upgrade
- explain or inspect an existing `report.json`
- render a GitHub, GitLab, or build-summary view from an existing report
- debug why a run returned `rerun` or another inconclusive outcome
- edit Limier fixture, scenario, or rules inputs in this repository

Do not use this skill for general application security review, dependency policy advice unrelated to Limier, or repo tasks that do not touch the CLI or its review workflow.

## Quick Command Choice

- Need a fresh review: use `run`
- Already have `report.json` and need an explanation: use `inspect`
- Already have `report.json` and need CI or PR output: use `render`
- Need a known-good smoke test from this repo: run `sh ./examples/ci/run-sample.sh`

## Preconditions

- Docker must be available to the current user.
- Host-signal capture is Linux-only and requires `bpftrace`.
- On macOS, Windows, or CI environments without `bpftrace`, make sure the scenario sets `capture_host_signals: false` or expect an inconclusive result.

For local development in this repo, prefer:

```sh
go build -o ./bin/limier .
./bin/limier version
```

## Default Repo Paths

When the user has not specified otherwise, the repository-owned sample uses:

- fixture: `fixtures/npm-app`
- scenario: `scenarios/npm.yml`
- rules: `rules/default.yml`
- outputs: `out/limier/report.json`, `out/limier/summary.md`, `out/limier/evidence/`

## Recommended Workflow

1. Confirm the ecosystem, package, current version, candidate version, fixture, scenario, and rules file.
2. Check whether the environment can satisfy the scenario, especially `capture_host_signals`.
3. Run Limier with explicit output paths under `out/limier/`.
4. Read `summary.md` first.
5. If the result is not `good_to_go`, inspect `evidence/` and any rendered explanation before making claims.
6. When useful, rerender the same `report.json` instead of rerunning the fixture.

## Core Commands

### Run a review

```sh
go run . run \
  --ecosystem npm \
  --package left-pad \
  --current 1.0.0 \
  --candidate 1.1.0 \
  --fixture fixtures/npm-app \
  --scenario scenarios/npm.yml \
  --rules rules/default.yml \
  --report out/limier/report.json \
  --summary out/limier/summary.md \
  --evidence out/limier/evidence
```

Supported ecosystems today:

- `cargo`
- `npm`
- `pip`

### Explain an existing report

```sh
go run . inspect --input out/limier/report.json
```

### Render an existing report

```sh
go run . render --format build-summary --input out/limier/report.json
```

Supported render formats today:

- `build-summary`
- `github-comment`
- `gitlab-note`

## How To Interpret Results

- `good_to_go`: the run did not find suspicious behavior with the current ruleset
- `needs_review`: a human should inspect the behavioral change before approval
- `block`: the change matched a hard-block rule and should not be approved yet
- `rerun`: the result is inconclusive or unstable and should not be treated as safe

Exit codes matter in CI:

- `0`: `good_to_go`
- `1`: `needs_review` or `block`
- `2`: `rerun` or another inconclusive result

## Handling `rerun`

Treat `rerun` as an environment or determinism problem first, not a safe outcome. Common causes:

- Docker access problems
- fixture nondeterminism
- missing Linux host-signal support while `capture_host_signals` is enabled
- scenario steps failing before comparison completes

When debugging, inspect the scenario, evidence bundle, and environment assumptions before changing rules.

## Editing Scenarios And Rules

When a task involves changing review behavior:

- read [Scenario File](../../docs/reference/scenario-file.md) before changing scenario structure
- read [Rules File](../../docs/reference/rules-file.md) before changing verdict logic
- prefer the smallest change that affects the user-visible recommendation
- protect user-visible behavior with focused tests, especially around verdict classification and report generation

## Validation

After changing Limier code or review behavior, prefer this loop:

```sh
go test ./cmd ./internal/...
go test ./...
go vet ./...
gofmt -w .
```

Use the repo sample runner for an end-to-end check when the change affects the CLI flow:

```sh
sh ./examples/ci/run-sample.sh
```
