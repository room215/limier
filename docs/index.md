---
layout: home

hero:
  name: Limier
  text: "Understand what a dependency upgrade actually did"
  tagline: "Limier compares two versions of the same dependency in a controlled fixture and tells you whether the change looks safe, needs review, should be blocked, or should be rerun."
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: Review Your Project
      link: /guide/review-your-own-project
    - theme: alt
      text: CLI Reference
      link: /reference/cli

features:
  - title: Compare the same workflow twice
    details: "Limier runs your baseline and candidate dependency versions in the same fixture, with the same scenario, so the diff is easier to trust."
  - title: See a reviewer-facing outcome
    details: "The result is intentionally simple: `good_to_go`, `needs_review`, `block`, or `rerun`, with a JSON report, Markdown summary, and evidence bundle."
  - title: Use it where reviews already happen
    details: "Run Limier locally while investigating a package upgrade or wire it into CI to review dependency changes automatically."
---

## What Is Limier?

Limier is a CLI tool for one narrow job: compare a baseline dependency version with a candidate version inside the same sample application, capture what changed, and tell a reviewer what to do next.

It is especially useful when you want to answer questions like:

- Did this dependency start launching a new process?
- Did install-time behavior change?
- Did the package stop behaving the same way in a realistic sample app?
- Is this difference benign, suspicious, or too noisy to trust?

Limier is intentionally not a general-purpose application security scanner. It is focused on dependency behavior drift.

## How A Review Works

1. Pick the dependency you want to review.
2. Point Limier at a fixture that uses that dependency.
3. Give Limier a scenario that says how to install and exercise the fixture.
4. Run the same scenario against the current version and the candidate version.
5. Inspect the verdict, findings, and evidence.

## Supported Ecosystems

The current adapters are:

- `npm`
- `pip`
- `cargo`

## Start Here

- [Getting Started](/guide/getting-started) if you want a first successful run as quickly as possible.
- [Review Your Own Project](/guide/review-your-own-project) if you already know which dependency and fixture you want to test.
- [Understand Results](/guide/understand-results) if you have a report and want to know what to do with it.
- [Use With Codex](/guide/use-with-codex) if you want to package repository knowledge as a Codex skill or plugin.
- [CLI Reference](/reference/cli) if you want the exact commands and flags.
