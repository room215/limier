# Use Limier In CI

Limier fits into CI best when you keep the contract small:

1. run `limier run`
2. preserve `report.json`, `summary.md`, and `evidence/`
3. optionally run `limier render` for the surface you want to publish

The report is the source of truth. Rendered outputs are just alternate presentations of the same result.

## Minimal GitHub Actions Example

This repository includes a small sample workflow for manually running the repository-owned demo assets:

```yaml
name: limier

on:
  workflow_dispatch:

jobs:
  review-upgrade:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Run repository sample
        run: sh ./examples/ci/run-sample.sh
      - name: Publish build summary
        run: cat out/limier/build-summary.md >> "$GITHUB_STEP_SUMMARY"
      - name: Upload Limier artifacts
        uses: actions/upload-artifact@v4
        with:
          name: limier-artifacts
          path: out/limier
```

The idea is simple:

- run Limier
- write a human-readable summary into the CI system
- upload the evidence bundle so someone can inspect it later

## Review Dependency Pull Requests

Limier is a better fit for a dependency-review workflow than for a Dependabot-only workflow.

The practical goal is to classify each pull request into one of three paths:

1. no dependency change
2. a machine-parsable dependency upgrade that Limier can compare
3. a dependency change that still needs human review

That keeps the control honest:

- unrelated pull requests do not spend meaningful runner time
- ordinary dependency upgrades still get an automated behavior diff
- human-authored or ambiguous dependency changes do not silently pass as if Limier reviewed them

### Recommended Workflow Shape

For GitHub pull requests, the usual setup is:

- trigger on `pull_request`
- keep the workflow required and always reachable for the default pull request activity types such as `opened`, `synchronize`, and `reopened`
- use a cheap first step or job to detect whether the pull request changed dependency manifests, lockfiles, Limier config, or the Limier workflow itself
- run Limier only when you can derive a real baseline and candidate version pair
- pair the workflow with native review policy for dependency files when an automated diff is not available

Dependabot is still a good input to that workflow because `dependabot/fetch-metadata` can provide the dependency name, ecosystem, previous version, and new version. Bot identity should be an optimization, not the top-level gate.

If you use `dependabot/fetch-metadata`, remember that once a GitHub Actions `permissions` block is present, omitted scopes default to `none`, so `pull-requests: read` must be declared explicitly.

### What To Detect First

The classifier should cheaply answer:

- Did this pull request change a dependency manifest or lockfile?
- Did it change `.limier/**` or the workflow that governs dependency review?
- Can the workflow derive `--ecosystem`, `--package`, `--current`, and `--candidate` safely?

Useful file sets typically include:

- `package.json`
- `package-lock.json`
- `requirements.txt`
- `Cargo.toml`
- `Cargo.lock`
- `.limier/**`
- `.github/workflows/limier*.yml`
- `.github/workflows/limier*.yaml`

If the answer is "no dependency change," the workflow should publish a short "not applicable" summary and succeed quickly.

If the answer is "yes, and the upgrade is machine-parsable," run `limier run` and let its verdict drive the policy.

If the answer is "yes, but there is no safe baseline/candidate pair," make that explicit in the summary and rely on native GitHub review policy such as `CODEOWNERS` or repository rulesets for those dependency files.

### Avoid Workflow-Level Path Filters On A Required Workflow

Do not make the required workflow itself conditional with trigger-level `paths` filters.

GitHub leaves required checks pending when the whole workflow is skipped by path filtering, which is a poor fit for a workflow that should be always present in branch protection. Prefer detecting changed files inside the workflow and exiting quickly when nothing relevant changed.

### Name The Check After The Policy

Prefer a required status name such as `dependency-review` over `limier`.

That keeps the UI honest:

- success can mean "not applicable" or "automated review passed"
- the step summary can explain whether Limier actually ran
- a separate reviewer-approval policy can cover new dependencies or ambiguous edits without pretending the behavioral diff happened

::: warning Avoid `pull_request_target` for the review run
The safest default is to run Limier in the `pull_request` context with a read-only `GITHUB_TOKEN` and keep commenting, labeling, or auto-merge behavior in a separate privileged follow-up workflow if you need it.

That is unprivileged in the GitHub API sense only. Limier still needs Docker daemon access to run fixtures, so this workflow should run on GitHub-hosted runners or dedicated isolated self-hosted runners rather than on broadly shared infrastructure.
:::

## Hosted Runners vs Self-Hosted Runners

For GitHub-hosted runners, assume Docker is available but full host-signal capture is not. In that environment you should typically use:

```yaml
evidence:
  capture_host_signals: false
```

Use a self-hosted Linux runner with `bpftrace` installed when you want full host telemetry.

## Run Limier From The Container Image

Release tags also publish a container image:

```sh
ghcr.io/room215/limier:<version>
```

When you run Limier from the container against a host Docker daemon, mount your repository at the same absolute path inside the container that it has on the host. That keeps fixture paths valid when Limier asks Docker to bind-mount them again.

Mounting `/var/run/docker.sock` gives the Limier container control over the host Docker daemon so it can create the review containers. Treat that as runner-level container control, not as a sandbox for untrusted pull request code.

```sh
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD:$PWD" \
  -w "$PWD" \
  ghcr.io/room215/limier:<version> \
  run \
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

If that command fails with a Docker socket permission error, make sure the host user already has access to `/var/run/docker.sock`. On Linux, a common fix is to add the Docker group inside the container with `--group-add "$(getent group docker | cut -d: -f3)"` alongside `--user`.

For the easiest containerized setup, disable host-signal capture in the scenario.
