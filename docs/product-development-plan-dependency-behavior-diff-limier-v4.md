# Product Development Plan — Limier (Ecosystem-Agnostic CLI Revision)

**Date:** 2026-04-17  
**Prepared for:** Sardo  
**Based on:** PRD dated 2026-04-15 and follow-up product review

---

## 1. Product Definition

The product is **Limier, a CLI-first dependency behavior diff tool**.

Given:

- a package manager or ecosystem such as `npm`, `pip`, `cargo`, `maven`, `gradle`, or another supported adapter
- a dependency under test
- a current version
- a candidate version
- a sample application or fixture
- one deterministic execution scenario
- a ruleset

Limier should:

1. create a **baseline execution environment** using the current dependency version
2. create a **candidate execution environment** using the newer dependency version
3. run the **same scenario** in both environments
4. collect **application logs** and **host-side eBPF telemetry**
5. normalize and compare the results
6. evaluate the diff against rules
7. emit a **JSON report**, a **Markdown summary**, and a **verdict**

This is not a GitHub-first product.

It is a portable engine that should work in:

- local security analysis
- Jenkins
- GitLab CI
- Buildkite
- CircleCI
- GitHub Actions

PR comments, merge-request notes, and build-summary integrations are adapters on top of the same report, not the product core.

---

## 2. v1 User Promise

The first version should make one promise clearly:

> **Tell me whether upgrading dependency X from version A to version B causes meaningful new behavior in the same sample app, and show me why.**

That promise should not depend on GitHub, and it should not depend on one package manager.

The v1 outcome should be practical:

- **good to go**
- **needs review**
- **block**
- **inconclusive**

---

## 3. Primary Workflow

The primary workflow for v1 is:

> **A security or platform team runs Limier in Linux CI, or manually on a Linux host, to compare one dependency upgrade inside one controlled sample application, regardless of source-control platform and regardless of supported package ecosystem.**

That workflow is intentionally narrow:

- one dependency at a time
- one fixture or sample app at a time
- one deterministic scenario at a time
- one verdict at the end

This keeps the product from becoming either:

- a GitHub feature
- a broad runtime-security platform
- a package-manager-specific point solution

---

## 4. Core CLI Surface

The core product surface should be one command:

```bash
limier run \
  --ecosystem <ecosystem> \
  --package <dependency-name> \
  --current <current-version> \
  --candidate <candidate-version> \
  --fixture <path-to-sample-app> \
  --scenario <path-to-scenario> \
  --rules <path-to-rules> \
  --report <path-to-report.json> \
  --summary <path-to-summary.md> \
  --evidence <path-to-evidence>
```

Examples of `<ecosystem>` include `npm`, `pip`, `cargo`, `maven`, or any other implemented adapter.

### Required outputs

- `report.json` — machine-readable source of truth
- `summary.md` — portable human-readable artifact
- evidence bundle or evidence directory — optional supporting material
- process exit code — CI policy signal

### Secondary commands

These are useful later, but they should not define the product:

```bash
limier render --format github-comment --input report.json
limier render --format gitlab-note --input report.json
limier inspect --input report.json
limier upload --input report.json --evidence evidence.tgz
```

---

## 5. Execution Model

### 5.1 Two-Sided A/B Comparison

Limier should execute the same sample application in two isolated environments.

#### Baseline side
- same fixture
- same base image
- same environment
- same scenario
- current dependency version

#### Candidate side
- same fixture
- same base image
- same environment
- same scenario
- candidate dependency version

For v1, these should be implemented as **Docker-based execution environments**.

The product promise is not “two special npm containers.”
The product promise is:

- two isolated executions
- same scenario
- different dependency version
- fair behavioral comparison

### 5.2 Repeated Runs

Repeated runs should exist from the start.

A single run per side is too noisy for a trustworthy verdict.

Limier should support:

- `N` runs on the baseline side
- `N` runs on the candidate side
- a small stability model that suppresses one-off noise

Recommended v1 defaults:

- default repeat count: `2`
- higher-confidence repeat count: `3`

### 5.3 Scenario Steps, Not a Heavy Phase Framework

The user should think in terms of **one scenario**.

Internally, a scenario may contain steps like:

- install
- start
- exercise
- exit

But the product should not force a large universal phase model onto the user.

For v1, it is enough to support two practical analysis modes:

#### Install analysis
Question answered:
- does the candidate version do anything suspicious while being resolved or installed?

#### App behavior analysis
Question answered:
- once installed, does the candidate version make the sample app behave differently at runtime?

A scenario may include one or both.

---

## 6. Scenario Manifest

The scenario manifest should stay simple and ecosystem-neutral.

It should describe:

- ecosystem / package manager
- dependency under test
- sample app or fixture location
- how to materialize the baseline version
- how to materialize the candidate version
- commands to run
- environment variables
- mounted fixtures
- expected success criteria
- repeat count
- optional network constraints
- evidence collection options

A minimal scenario should feel like:

1. materialize dependency version
2. start the sample app or run the install step
3. execute one deterministic action
4. stop and collect results

The scenario format should not assume npm semantics. It should remain generic, with ecosystem details handled by adapters.

---

## 7. Ecosystem and Adapter Model

The core engine must be **ecosystem-agnostic**.

That means the plan should not treat npm as the defining adapter, or any other ecosystem as privileged.

### 7.1 Adapter Responsibilities

Each package-manager adapter should own:

- version resolution
- dependency materialization
- install or fetch command generation
- lockfile or environment handling when required
- preparation of the baseline side
- preparation of the candidate side
- adapter-specific metadata needed in the report

### 7.2 Product Rule

The engine should receive the ecosystem as an input and then delegate package-manager-specific work to the corresponding adapter.

The core diff, rule, verdict, report, and evidence systems should remain the same across ecosystems.

### 7.3 Launch Stance

The launch plan should include a **small starter set of first-class adapters**, not one privileged ecosystem.

The important product principle is:

- no single ecosystem defines the architecture
- no milestone should assume npm is “the real product”
- the adapter interface should be stable enough that additional ecosystems can be added without redesigning the core

A practical launch bundle should include at least **two materially different ecosystems**, with the exact set chosen by design-partner demand.

---

## 8. Telemetry Collection

Limier should collect two classes of evidence.

### 8.1 Application-visible evidence

- stdout / stderr
- exit codes
- timing
- scenario step logs

### 8.2 Host-side behavioral evidence

Collected outside the target environment using eBPF:

- executed processes
- important file reads and writes
- outbound network connections
- DNS lookups when feasible

The product is not “collect every kernel event.”

The product is:

- collect the smallest set of signals that explains **why** the candidate behaves differently
- keep signal quality high enough that the verdict is trusted

### 8.3 Initial signal set

For v1, the collector should focus on:

- `process.exec`
- `process.exit`
- `file.read` for sensitive locations
- `file.write` for unusual or sensitive writes
- `net.connect`
- `dns.query` when feasible

That is enough to answer the first-order “why” question.

---

## 9. Diff Model

The product should not emit a raw syscall dump as its primary output.

It should emit **typed differences** between the baseline and candidate sides.

### v1 finding classes

- new executable observed
- new outbound destination observed
- new sensitive file read observed
- new sensitive file write observed
- new write outside expected directories observed
- candidate failed or diverged from expected scenario outcome

### What the diff engine should answer

- did the candidate do something the baseline did not?
- did it happen consistently across repeated runs?
- does the change match a hard-block rule, a review rule, or a known-safe suppression?

The goal is not perfect replay.
The goal is useful, stable comparison.

---

## 10. Rule Model

The rule system should stay narrow in v1.

### Hard-block rules

Examples:

- new read of SSH keys
- new access to cloud credentials
- new outbound connection to a disallowed destination

### Review rules

Examples:

- new process execution
- new outbound domain
- new write outside workspace or cache

### Suppression rules

Examples:

- cache path differences
- temp-directory noise
- retry-related DNS chatter

The rule system should answer “why did the verdict change?”

It should not become a large policy-language project in v1.

YAML-first is enough.

---

## 11. Verdict Model

The product should separate technical classification from operator recommendation.

### Technical verdict

- `no_diff`
- `expected_diff`
- `unexpected_diff`
- `suspicious_diff`
- `inconclusive`

### Operator recommendation

- `good_to_go`
- `needs_review`
- `block`
- `rerun`

This keeps the system practical.
A real diff does not always mean a dangerous diff.

---

## 12. Report Model

The report format is a core product asset.

### 12.1 JSON report

The JSON report should be the source of truth and include:

- dependency metadata
- ecosystem / package manager
- baseline and candidate versions
- fixture identity
- scenario identity
- repeated-run settings
- normalized summaries for both sides
- typed findings
- rule hits
- technical verdict
- operator recommendation
- exit-code classification
- evidence references

### 12.2 Markdown summary

The Markdown summary should be generated from the JSON report and should include:

- what was compared
- what scenario ran
- what changed
- why the verdict was assigned
- what the reviewer should do next
- where the evidence lives

The Markdown summary should work equally well as:

- a CI artifact
- a local analysis artifact
- the input for a PR or MR comment renderer

---

## 13. Exit Codes

The CLI should keep the CI contract simple.

### Recommended v1 exit codes

- `0` — good to go / no blocking findings
- `1` — block or review-required according to configured policy
- `2` — inconclusive run or Limier failure

The JSON report should carry richer meaning than the process exit code.

---

## 14. Architecture

The architecture should match the simple product promise.

### 14.1 CLI layer
Owns:
- `limier run`
- argument parsing
- filesystem layout
- logs
- exit-code mapping

### 14.2 Scenario engine
Owns:
- manifest parsing
- validation
- deterministic step execution
- success and failure criteria

### 14.3 Package-manager adapter layer
Owns:
- ecosystem-specific dependency setup
- install or fetch behavior
- lockfile and environment handling where required
- version substitution for baseline and candidate sides

### 14.4 Environment manager
Owns:
- Docker image preparation
- baseline environment materialization
- candidate environment materialization
- repeated runs
- cleanup

### 14.5 Collector
Owns:
- host-side event capture
- integration with the native eBPF backend
- optional research backend early on if needed

### 14.6 Attribution and normalization layer
Owns:
- mapping events to the correct run and environment
- path normalization
- process normalization
- network destination normalization
- sensitivity classification

### 14.7 Diff and verdict engine
Owns:
- baseline vs candidate comparison
- repeated-run stability checks
- typed findings
- technical verdict and operator recommendation

### 14.8 Reporter
Owns:
- JSON report
- Markdown summary
- evidence manifest

### 14.9 Adapter/render layer
Owns:
- GitHub comment rendering
- GitLab note rendering
- Jenkins summary rendering

This layer is deliberately downstream from the report.

---

## 15. v1 Scope

### In scope

- Linux host only
- Docker only
- Go control plane
- one core CLI command: `limier run`
- JSON report as source of truth
- Markdown summary artifact
- simple exit codes
- host-side collection
- deterministic fixture-based execution
- repeated runs per side
- package-manager-agnostic core engine
- small launch bundle of first-class ecosystem adapters
- optional evidence archive

### Explicitly not in scope for v1

- GitHub-first product design
- a large web dashboard
- Kubernetes-first operation
- active workload blocking during execution
- broad malware attribution
- “support every package ecosystem equally on day one”
- a general runtime-security platform

---

## 16. Milestones

### Milestone 0 — Lock the generic contract

Deliverables:

- CLI contract for `limier run`
- scenario manifest v1
- JSON report schema v1
- Markdown summary template v1
- verdict and exit-code model
- package-manager adapter interface

Success gate:

A design partner can understand the product just by reading the example command and the example report.

### Milestone 1 — End-to-end core engine across multiple adapters

Deliverables:

- baseline/candidate Docker execution loop
- stable package-manager adapter interface
- at least two first-class adapters in the launch bundle
- fixture-based execution
- logs plus initial host-side signal capture
- JSON and Markdown outputs
- repeated runs per side

Success gate:

Limier can compare real dependency upgrades in more than one ecosystem and still produce the same style of believable verdict.

### Milestone 2 — Native signal quality and verdict trust

Deliverables:

- stronger attribution
- better path, process, and network normalization
- first review and block rules
- suppression handling
- evidence bundle support

Success gate:

The same suspicious pattern is detected consistently across reruns, with manageable noise.

### Milestone 3 — CI hardening and renderer layer

Deliverables:

- CI-friendly packaging
- example integrations for Jenkins, GitLab CI, Buildkite, CircleCI, and GitHub Actions
- thin renderers for PR, MR, and build-summary surfaces
- diagnostics for inconclusive runs

Success gate:

The same `report.json` can drive multiple environments without changing core behavior.

### Milestone 4 — Adapter expansion and rule-pack refinement

Deliverables:

- additional adapters added without redesigning the engine
- ecosystem-specific rule overlays only where genuinely necessary
- adapter contract hardening based on real usage

Success gate:

The architecture proves it is truly package-manager-agnostic, not merely npm-shaped with extra plugins.

---

## 17. Success Criteria

The first version should be judged by a short list of questions:

1. Can Limier clearly compare version A vs version B in the same sample app?
2. Can it explain **why** the verdict changed?
3. Does the same core report work across more than one package ecosystem?
4. Is the report understandable without reading raw telemetry?
5. Does repeated execution reduce noise enough for CI use?
6. Can the same output be used locally and in CI without changing semantics?

If the answer is yes to those questions, v1 is working.

---

## 18. Recommendation

The revised product plan should be simpler and more generic than the earlier version.

The right v1 is:

- one CLI-first engine
- one A/B comparison model
- one deterministic scenario contract
- one JSON report as source of truth
- one Markdown summary
- one verdict and recommendation system
- one package-manager-agnostic core
- a small launch bundle of first-class adapters
- PR and MR comments as thin renderers later

The right product sentence is:

> **Run the same sample application with dependency version A and version B, collect logs and host-side behavior, compare the two, and tell me whether the update is safe enough to approve — with evidence — regardless of supported package ecosystem or SCM.**

That matches the intended product more closely than either a GitHub-centric plan or an npm-shaped plan.
