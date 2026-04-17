# Business Plan — Limier (CI-Native Revision)

**Date:** 2026-04-15  
**Prepared for:** Sardo  
**Based on:** PRD dated 2026-04-15

---

## 1. Executive Summary

This company should not be positioned first as a GitHub app, a pull-request widget, or a generic runtime security platform.

It should be built and sold as a **CI-native dependency behavior review system** with one core product surface:

> **A standalone CLI that compares the current and candidate dependency versions under the same deterministic scenario and emits a policy-backed verdict.**

The core command is:

```bash
limier run --scenario scenario.yaml --report out/report.json --summary out/summary.md
```

That command is the product. Everything else is an adapter.

The CLI runs in Linux CI, executes the same workload against the current and candidate dependency versions, repeats runs to reduce noise, collects host-side telemetry, normalizes the results, and produces:

- a JSON report as the source of truth
- a Markdown summary as a portable artifact
- simple exit codes for CI policy
- an optional evidence bundle for upload or archive

That surface works in:

- local security analysis
- Jenkins
- GitLab CI
- Buildkite
- CircleCI
- GitHub Actions

GitHub PR comments, GitLab merge-request notes, Jenkins build summaries, and similar UX should be treated as **thin renderers** on top of the same report format, not as the product itself.

### The wedge

The right v1 wedge is:

> **Security and platform teams running dependency reviews in Linux CI, regardless of SCM.**

That is broad enough to avoid GitHub lock-in, but narrow enough to ship.

### The business thesis

Customers already have tools that answer questions like:

- Is this dependency vulnerable?
- Is it reachable?
- Did the package metadata or source look suspicious?
- Did the lockfile change?

They still often lack a good answer to the final approval question:

> **What did this dependency change actually do in our build/test/runtime environment?**

That decision gap is the commercial opportunity.

---

## 2. Product Definition in Business Terms

The company is selling a new control in the dependency review flow.

It is not selling raw eBPF telemetry. It is not selling a broad CNAPP. It is not selling package reputation scoring.

It is selling:

1. **Behavioral evidence for dependency changes**
2. **A deterministic verdict that can be consumed in CI**
3. **A portable report format that works across SCM and CI systems**
4. **A policy layer that helps security teams decide whether to approve, investigate, or block a change**

### v1 product surface

The initial commercial product is a CLI plus policy/report format:

- `limier run`
- deterministic scenario manifest
- repeated runs per side
- JSON report as the source of truth
- Markdown summary artifact
- exit codes for CI policy
- optional evidence archive/upload

### What is explicitly not the v1 product surface

The following are adapters or later surfaces, not the core product:

- GitHub PR comments
- GitLab MR comments
- Jenkins summary pages
- centralized dashboards
- browser-heavy analyst consoles

These can matter later, but none of them should define the company’s first product.

---

## 3. Exact Initial Buyer and Budget Owner

## Recommended initial buyer

The initial economic buyer should be:

> **Director / Head of Product Security, Application Security, or Platform Security at a cloud-native software company with centralized Linux CI**

This buyer is the best fit because the product is fundamentally a **merge-approval and review-evidence control** for supply-chain risk.

It is not primarily a developer productivity purchase and not primarily a SOC purchase.

## Buying committee

### Economic buyer
- Director / Head of Product Security, AppSec, or Platform Security
- owns policy, exceptions, and security tooling budget

### Technical champion
- Staff / Senior Product Security Engineer
- or Staff Platform Security Engineer
- directly feels the pain of dependency review and exception handling

### Implementation owner
- Platform Engineering Manager
- DevInfra Manager
- DevSecOps Manager
- owns runner pools, Docker images, and CI rollout

### Secondary stakeholder
- Engineering productivity or developer platform lead
- cares about CI time, workflow friction, and supportability

## Budget owner

The budget should come from a **central security tooling budget**, usually AppSec or product security, with explicit buy-in from platform engineering because the control consumes CI resources.

That budget assignment is important. This should not be sold first as a team-by-team developer subscription. The rollout surface is centralized CI policy, not individual IDE usage.

## Who should not be the first buyer

Do not optimize the first version for:

- individual developers buying from discretionary team budgets
- SOC teams without build-system ownership
- compliance-only buyers who cannot sponsor CI changes
- organizations with no central Linux CI workflow
- companies that review very few dependency changes per month

---

## 4. The Single Primary Workflow

The core workflow should be described the same way in product, marketing, and sales:

> **A security or platform team adds a CI step that runs `limier run` whenever a dependency change is proposed or manually investigated. The tool compares current vs candidate versions under a deterministic scenario, emits a verdict, publishes a machine-readable report, and returns an exit code that can be used in CI policy.**

This workflow can be triggered by:

- a dependency update branch
- a merge request or pull request
- a nightly security review job
- a manual local analysis session
- an incident-response investigation

The workflow stays the same even when the SCM or CI changes.

That stability is commercially valuable because it reduces product rework and preserves the customer story:

- **same CLI**
- **same manifest**
- **same JSON**
- **same verdict model**
- **different renderer or adapter depending on environment**

---

## 5. Proof of Pain: When This Becomes Must-Buy

This is not a universal must-buy. It becomes must-buy for a specific segment.

## The must-buy profile

The product becomes high-priority when a customer has most of the following:

- automated dependency updates already enabled
- a meaningful monthly volume of dependency changes to review
- Linux CI environments with access to meaningful credentials, signing assets, or cloud access
- existing SCA and supply-chain tools already deployed
- security and platform teams still forced to make human approval decisions with incomplete runtime evidence

## Why the pain is real

Modern supply-chain attacks increasingly target developer and CI environments rather than only production runtime. Sonatype reports that it identified more than **454,600** new malicious packages in 2025, and says developer workstations and CI/CD environments have become high-leverage targets for secret and credential theft. Microsoft’s Shai-Hulud 2.0 write-up describes malicious code running during npm `preinstall`, before tests or later checks, and explicitly calls out CI/CD and cloud-connected workloads as targets.

## Why existing controls still leave a gap

GitHub’s dependency review action is useful for vulnerabilities, licenses, and dependency diff metadata, but it is scoped to pull-request dependency changes and does not execute the dependency or compare behavioral effects across versions. Socket publicly describes its core package-risk detections as static analysis of package code and metadata. Endor Labs emphasizes reachability, remediation guidance, and upgrade impact analysis. OpenSSF Package Analysis dynamically inspects packages and tracks behavior changes over time, but it is a community-run project rather than a customer-operated CI-native review product.

The unresolved question after those tools is still:

> **Should we approve this dependency change in our environment?**

That is the specific pain the company should monetize.

## The operational pain pattern

In the target customer, dependency review often fails in one of two ways:

### Failure mode A: slow approvals
Teams leave updates open because nobody can quickly explain whether a new behavior is benign or suspicious.

### Failure mode B: low-confidence approvals
Teams merge because tests passed and static tools were green, even though the candidate version changed install/build/runtime behavior in a security-relevant way.

## The economic pain pattern

The purchase does not need to be justified only by catastrophic breach prevention. It can be justified by a mix of:

- reduced review time for common dependency changes
- better evidence for high-risk changes
- lower exception-handling overhead
- lower need for ad hoc sandboxing or one-off manual analysis
- reduced blast radius from one bad dependency approval

### A practical ROI model

Assume a target customer has:

- 1,500 dependency changes per month across critical repos
- 25% of those changes reviewed by security or platform engineers
- 4 minutes average spent on each non-trivial review
- $120/hour fully loaded reviewer cost

That equals:

- 375 reviewed changes per month
- 1,500 minutes per month
- 25 hours per month
- about **$36,000/year** in direct reviewer time

If the product saves only half of that review effort, the time-based value is already material. If it also prevents one serious CI credential-exfiltration or malicious-package incident, the economic justification becomes much stronger.

These assumptions are directional, but they are realistic enough to support a focused pilot sale.

---

## 6. Why Customers Buy This Instead of Alternatives

The sales message must be very precise.

The answer is **not** “because we have eBPF.”

The answer is:

> **Because existing tools mostly tell you what is known, reachable, or suspicious in package metadata. This product tells you what actually changed when you ran the dependency in your own CI environment under a defined workload.**

## Comparison with existing CI controls

### Existing CI controls answer:
- did the lockfile change?
- is the version vulnerable?
- did a license violate policy?
- did a job fail?

### This product answers:
- did the candidate version add a new executable?
- did it reach a new outbound destination?
- did it touch a new sensitivity class of files?
- did it change process tree shape during install, build, test, or runtime?
- are those changes policy-acceptable?

## Comparison with Socket

Socket’s public documentation says it uses static analysis to detect risky package behavior such as install scripts, network usage, environment-variable access, filesystem access, and other suspicious indicators. That is useful and valuable. But it is still a package-centric static view.

The wedge here is different:

- customer-defined workload
- deterministic A/B execution
- repeated runs to reduce noise
- host-side telemetry
- one verdict tied to the customer’s own CI environment

## Comparison with Endor Labs

Endor Labs positions strongly around reachability, remediation, and upgrade impact analysis. That is a strong adjacent category. But the public product framing is still centered on prioritization and remediation of dependency risk rather than deterministic runtime behavior diffing under customer-owned workloads.

## Comparison with OpenSSF Package Analysis

OpenSSF Package Analysis is the closest conceptual precedent. It analyzes packages in a sandbox, captures behavioral signals, and tracks changes in package behavior over time.

The commercial distinction is:

- OpenSSF Package Analysis is community-run research infrastructure
- this product is a customer-operated review control for private CI workflows
- it compares current vs candidate versions in the customer’s scenario
- it emits CI-consumable verdicts and portable evidence

## Comparison with homegrown scripts

Customers can always say, “Can’t we just run a package in a container and diff some logs?”

The honest answer is yes, for a proof of concept.

But that breaks down quickly because customers still need:

- stable normalization
- repeated-run variance handling
- evidence packing
- typed findings
- phase markers
- reusable policy packs
- deterministic report format
- adapter renderers across CI/SCM

That is where the product becomes defendable.

---

## 7. Narrow Design-Partner Profile

The design-partner profile must be narrow enough to land within one or two quarters.

## Recommended design partner

Target companies with all of the following:

- **200–1,000 engineers**
- **20–100 production repos** where dependency changes matter
- **centralized Linux CI** that can run Docker-based jobs
- **Node and/or Python** as meaningful ecosystems
- **self-hosted runners or controlled CI budgets**
- **a small but capable AppSec or platform-security team**
- **existing Dependabot and/or Renovate use**
- **a real dependency review pain point**, not just general supply-chain curiosity

## Why this profile is landable

This profile is realistic because:

- the customer already has a dependency-review workflow
- the integration surface is a CLI in CI, not a platform replacement
- the pilot can start in 10–25 repos rather than the whole organization
- security and platform both have clear owners
- the initial ecosystem scope can stay narrow

## Strong positive signals during discovery

Good signs include:

- “We already use SCA and still do manual review for suspicious updates.”
- “Our CI has credentials we worry about.”
- “We want evidence before merge, not just package metadata.”
- “We can give you a Linux runner pool and a small repo set to test.”
- “We don’t want to lock ourselves to one SCM vendor.”

## Disqualifiers for early deals

Avoid early design partners that require:

- Kubernetes-first deployment as table stakes
- Windows-first build support
- broad language coverage from day one
- fully managed cloud execution of untrusted packages in the first pilot
- massive procurement overhead before a technical trial
- a need for polished enterprise dashboards before they will evaluate core verdict quality

---

## 8. Pricing Model

The pricing model should reinforce the product truth: this is a cross-CI control, not a GitHub add-on.

## Recommended unit of pricing

The cleanest usage unit is:

> **one comparison** = one current-vs-candidate analysis run for a single scenario, including the default repeated-run plan

This is better than charging per PR because:

- not every usage is a PR
- the product supports local analysis and scheduled jobs
- it stays independent of SCM vendors
- it maps directly to compute consumption and value delivered

## Recommended commercial structure

### Phase 1: design partner pricing

Offer a fixed-fee pilot:

- **$20k–$35k** for an 8–10 week pilot
- includes onboarding, policy tuning, and limited repo rollout
- limited to Node and Python
- limited to Linux + Docker CI
- includes weekly review and verdict tuning

This is high enough to validate seriousness and low enough to land without a full platform-buying cycle.

### Phase 2: production pricing

Use an annual subscription with included usage bands.

#### Team
- **$45k/year**
- up to **10,000 comparisons/year**
- 10 named users for policy/report access
- baseline rule packs
- 30-day evidence retention in product-managed storage if enabled

#### Growth
- **$95k/year**
- up to **50,000 comparisons/year**
- 25 named users
- SSO / audit basics
- 90-day retention
- advanced policy pack controls

#### Enterprise
- **$180k+/year**
- **150,000+ comparisons/year**
- private deployment options
- premium support
- custom retention and evidence export
- advanced rollout controls

### Overage model

- **$0.75–$1.50 per additional comparison** beyond plan limits
- overages should be predictable and capped by customer policy

## Why this pricing is credible

This pricing is defensible because the product sits closer to a security control than a developer utility, and because the direct value comes from:

- reviewer time saved
- higher-confidence merge decisions
- lower exception-handling overhead
- better incident prevention for high-impact supply-chain events

It also leaves room to keep the customer’s own CI spend separate and transparent.

---

## 9. Expected CI Cost Envelope

Customers will ask a fair question:

> “What does this cost me to run in CI?”

The answer should be concrete.

## Cost components

The marginal cost of one comparison is mostly:

- Linux runner minutes
- container image build or restore time
- artifact storage for reports and evidence bundles
- optional upload/retention of evidence

## GitHub-hosted benchmark

GitHub’s current published pricing lists Linux hosted runners at **$0.006/min** for standard 2-core x64 and **$0.012/min** for 4-core x64. GitHub also states that self-hosted runners are free from GitHub-hosted minute charges, with the customer responsible for their own infrastructure cost.

Using that as a public benchmark:

### Fast triage mode
- 2-core Linux
- ~12 minutes total
- estimated runner cost: **~$0.07**

### Standard verdict mode
- 4-core Linux
- ~18 minutes total
- estimated runner cost: **~$0.22**

### Deep investigation mode
- 4-core Linux
- ~30–45 minutes total
- estimated runner cost: **~$0.36–$0.54**

These numbers are a benchmark, not a universal promise. The important commercial point is that the CI runtime cost is usually small enough to support policy-driven selective use.

## How to keep CI cost acceptable

The business plan should assume:

- not every CI run uses Limier
- the default trigger is dependency-change analysis only
- deep investigation mode is reserved for suspicious or manual cases
- self-hosted runner pools are preferred for larger customers

## Product goal for cost envelope

v1 should target:

- **p50 time-to-verdict under 15 minutes**
- **standard-mode marginal hosted-runner cost well under $1 per comparison**

If the product exceeds those limits, adoption will suffer.

---

## 10. Go-to-Market Strategy

## Positioning statement

For security and platform teams reviewing dependency changes in Linux CI, Limier is a CI-native control that compares the current and candidate dependency under the same deterministic workload and returns a portable, policy-backed verdict. Unlike PR-only dependency scanners, static package analysis, or reachability tools, it shows what the change actually did in your own build, test, and runtime environment.

## Initial sales motion

### Motion
- founder-led sales
- design-partner pilots first
- security + platform joint discovery
- technical proof quickly after first call

### Entry point
- dependency review pain
- high volume of automated dependency changes
- concern about secrets in CI/build environments
- frustration with “green tools, low confidence” approvals

### Deal shape
- start with 10–25 repos
- Node and Python only
- Linux + Docker only
- one runner pool
- one or two policy packs

## Channels

The first channels should be:

- direct founder outreach to AppSec / platform-security leaders
- security engineering communities and supply-chain events
- design-partner introductions from trusted operators
- content that frames the problem as a review-decision gap, not just malware detection

## Messaging that should work

- “The missing reviewer between Dependabot and production.”
- “Prove what changed before you approve the dependency.”
- “One CLI, one report format, any Linux CI.”
- “Behavioral approval for dependency changes, not just metadata scanning.”

## Messaging to avoid

- “Universal runtime security platform.”
- “Works everywhere for everything.”
- “An eBPF tool for AppSec.”
- “A GitHub plugin for supply-chain attacks.”

---

## 11. Twelve-Month Milestones

## Quarter 1
- close 2–4 design partners
- prove CLI-first workflow in customer CI
- validate JSON report schema and verdict quality
- tune Node and Python rule packs

## Quarter 2
- convert at least 1–2 pilots to annual contracts
- ship evidence retention and adapter renderers
- stabilize repeatability and supportability
- publish 2–3 strong case studies or anonymized design-partner stories

## Quarter 3
- expand from first CI systems into adjacent adapters
- improve suppression management and rollout controls
- add second-wave enterprise features only after verdict quality is trusted

## Quarter 4
- tighten packaging for production deployment
- add selective ecosystem expansion
- move from founder-led implementation to repeatable onboarding motion

---

## 12. Key Risks

## Risk 1: Interesting but not mandatory

Mitigation:
- sell to customers with real dependency review volume and security ownership
- qualify hard on CI secrets, review bottlenecks, and existing SCA fatigue

## Risk 2: Too much SCM-specific surface too early

Mitigation:
- keep JSON report and CLI as source of truth
- treat GitHub, GitLab, and other integrations as renderers

## Risk 3: Noise kills trust

Mitigation:
- repeated runs
- narrow initial finding classes
- phase-aware policies
- conservative verdict thresholds

## Risk 4: CI cost or runtime gets too high

Mitigation:
- selective triggers
- fast/standard/deep modes
- self-hosted runner preference
- focus on changed-dependency analysis only

## Risk 5: Wedge broadens too soon

Mitigation:
- hold v1 to one workflow: dependency review in Linux CI
- do not chase broad runtime-security requirements early

---

## 13. Recommendation

This is worth building, but only as a very specific company at first.

The right first company is:

- **not** a GitHub app company
- **not** a generic eBPF company
- **not** a general runtime security company

It is a company that sells:

> **CI-native behavioral approval for dependency changes**

The right first product is:

- standalone CLI
- deterministic scenario manifest
- repeated runs per side
- JSON report as source of truth
- Markdown summary as artifact
- simple exit codes for CI policy
- optional evidence bundles
- thin SCM/CI renderers on top

If you keep that discipline, you preserve the strongest part of the idea: a portable decision system that can live in any Linux CI and answer the one question existing tools still leave ambiguous.

---

## 14. Research References

### PRD foundation
- Limier PRD (2026-04-15)

### Market and category references
- GitHub Dependency Review Action: https://github.com/actions/dependency-review-action
- GitHub Docs, Actions billing and runner pricing: https://docs.github.com/en/actions/concepts/billing-and-usage and https://docs.github.com/en/billing/reference/actions-runner-pricing
- Socket FAQ and Socket for GitHub: https://docs.socket.dev/docs/faq and https://docs.socket.dev/docs/socket-for-github
- Endor Labs upgrade impact analysis and docs overview: https://docs.endorlabs.com/risk-remediation/upgrade-impact-analysis/ and https://docs.endorlabs.com/
- OpenSSF Package Analysis: https://github.com/ossf/package-analysis
- Microsoft Security Blog, Shai-Hulud 2.0: https://www.microsoft.com/en-us/security/blog/2025/12/09/shai-hulud-2-0-guidance-for-detecting-investigating-and-defending-against-the-supply-chain-attack/
- Sonatype 2026 State of the Software Supply Chain, malware section: https://www.sonatype.com/state-of-the-software-supply-chain/2026/open-source-malware
