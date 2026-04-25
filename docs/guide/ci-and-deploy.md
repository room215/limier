# Use Limier In CI

For GitHub Actions, make `limier ci github` the normal entry point. It reads pull request metadata, decides whether Limier can safely run, writes CI-facing artifacts, and exits according to policy.

Use raw `limier run` and `limier render` when you are building a custom integration or using a CI system where you own metadata extraction and publishing.

By default, `limier ci github` fails the job for `block` and `rerun` recommendations:

```sh
limier ci github --output-dir out/limier --fail-on block,rerun
```

That means an inconclusive run does not pass as safe. A `needs_review` result writes a review-required status and succeeds unless you add `needs_review` to `--fail-on`.

## GitHub Actions Happy Path

A practical pull request workflow has four responsibilities:

1. classify whether dependency-relevant files changed
2. collect dependency metadata when the pull request comes from Dependabot
3. run `limier ci github`
4. publish the generated summary and artifact bundle

This example keeps the workflow required and always reachable for ordinary pull request updates:

```yaml
name: dependency-review

on:
  pull_request:
    types: [opened, synchronize, reopened]

permissions:
  contents: read
  pull-requests: read

jobs:
  dependency-review:
    name: dependency-review
    runs-on: ubuntu-latest
    env:
      LIMIER_VERSION: v0.1.4
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: Install Limier
        shell: bash
        run: |
          set -euo pipefail
          curl -fsSL "https://github.com/room215/limier/releases/download/${LIMIER_VERSION}/limier_${LIMIER_VERSION}_linux_amd64.tar.gz" | tar -xz
          sudo install -m 0755 limier /usr/local/bin/limier

      - name: Detect dependency-relevant files
        id: dependency_files
        shell: bash
        run: |
          set -euo pipefail
          changed_files="$(git diff --name-only "${{ github.event.pull_request.base.sha }}" "${{ github.event.pull_request.head.sha }}")"
          if grep -Eq '(^|/)(package(-lock)?\.json|requirements\.txt|Cargo\.(toml|lock))$|^\.limier/|^\.github/workflows/(limier|dependency-review).*\.ya?ml$' <<<"${changed_files}"; then
            echo "changed=true" >> "$GITHUB_OUTPUT"
          else
            echo "changed=false" >> "$GITHUB_OUTPUT"
          fi

      - name: Fetch Dependabot metadata
        id: dependabot_metadata
        if: ${{ github.event.pull_request.user.login == 'dependabot[bot]' }}
        uses: dependabot/fetch-metadata@v2
        continue-on-error: true

      - name: Run Limier dependency review
        env:
          LIMIER_CI_DEPENDENCY_FILES_CHANGED: ${{ steps.dependency_files.outputs.changed }}
          DEPENDABOT_METADATA_OUTCOME: ${{ steps.dependabot_metadata.outcome }}
          DEPENDABOT_PACKAGE_ECOSYSTEM: ${{ steps.dependabot_metadata.outputs.package-ecosystem }}
          DEPENDABOT_DEPENDENCY_NAMES: ${{ steps.dependabot_metadata.outputs.dependency-names }}
          DEPENDABOT_PREVIOUS_VERSION: ${{ steps.dependabot_metadata.outputs.previous-version }}
          DEPENDABOT_NEW_VERSION: ${{ steps.dependabot_metadata.outputs.new-version }}
        run: limier ci github --output-dir out/limier

      - name: Publish build summary
        if: always()
        shell: bash
        run: |
          if [ -f out/limier/build-summary.md ]; then
            cat out/limier/build-summary.md >> "$GITHUB_STEP_SUMMARY"
          else
            echo "Limier did not write a build summary." >> "$GITHUB_STEP_SUMMARY"
          fi

      - name: Upload Limier artifacts
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: limier-artifacts
          path: out/limier
```

Pin `LIMIER_VERSION` to a release that includes `limier ci github`; `v0.1.4` is the first release with that command.

## What The Command Writes

`limier ci github` writes these files under `--output-dir`, which defaults to `out/limier`:

- `build-summary.md`: Markdown designed for `$GITHUB_STEP_SUMMARY`
- `comment.md`: Markdown suitable for a pull request comment
- `status.json`: machine-readable status, recommendation, paths, metadata, and policy exit code
- `summary.md`: the human-readable Limier summary
- `report.json`: the full structured report when Limier actually runs
- `evidence/`: raw stdout, stderr, and event evidence when Limier actually runs
- `pr.txt`: the pull request number when GitHub event data is available

The command renders `build-summary.md`, `comment.md`, and `status.json` even when Limier skips the behavioral diff. That lets the workflow publish a clear result for `not_applicable`, `needs_review`, and `rerun` paths.

`comment.md` is only an artifact. `limier ci github` does not post comments, label pull requests, approve changes, or merge anything. Keep those privileged actions in a separate workflow if you need them.

## Metadata Contract

The command accepts dependency metadata from flags or environment variables.

Use these neutral variables when your own classifier provides the metadata:

- `LIMIER_CI_ECOSYSTEM`
- `LIMIER_CI_PACKAGE` or `LIMIER_CI_DEPENDENCY_NAMES`
- `LIMIER_CI_CURRENT` or `LIMIER_CI_PREVIOUS_VERSION`
- `LIMIER_CI_CANDIDATE` or `LIMIER_CI_NEW_VERSION`
- `LIMIER_CI_DEPENDENCY_FILES_CHANGED`

For Dependabot pull requests, pass the `dependabot/fetch-metadata` outputs through the environment:

- `DEPENDABOT_METADATA_OUTCOME`
- `DEPENDABOT_PACKAGE_ECOSYSTEM`
- `DEPENDABOT_DEPENDENCY_NAMES`
- `DEPENDABOT_PREVIOUS_VERSION`
- `DEPENDABOT_NEW_VERSION`

`DEPENDABOT_METADATA_OUTCOME` should be the step outcome from `dependabot/fetch-metadata`. If metadata lookup fails on a Dependabot-authored pull request, `limier ci github` returns `rerun` so the failure is visible instead of silently passing.

`LIMIER_CI_DEPENDENCY_FILES_CHANGED` should be `true`, `false`, or `unknown`:

- `false`: Limier writes `not_applicable` and exits successfully
- `true`: missing dependency metadata becomes `needs_review`
- `unknown` or unset: missing dependency metadata becomes `needs_review`

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
- `.github/workflows/dependency-review*.yml`
- `.github/workflows/dependency-review*.yaml`

If the answer is "no dependency change," pass `LIMIER_CI_DEPENDENCY_FILES_CHANGED=false` so `limier ci github` publishes a short `not_applicable` result and succeeds quickly.

If the answer is "yes, and the upgrade is machine-parsable," pass the dependency metadata and let `limier ci github` run the behavioral diff.

If the answer is "yes, but there is no safe baseline/candidate pair," pass `LIMIER_CI_DEPENDENCY_FILES_CHANGED=true` so `limier ci github` reports `needs_review`, then rely on native GitHub review policy such as `CODEOWNERS` or repository rulesets for those dependency files.

The default GitHub CI preset supports npm. For other ecosystems, pass `--fixture` and `--scenario` so Limier knows how to exercise the candidate package in your project.

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

## Custom CI Integrations

Use `limier run` directly when your CI system already knows the ecosystem, package name, current version, candidate version, fixture, scenario, and rules file.

```sh
limier run \
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

limier render \
  --format build-summary \
  --input out/limier/report.json \
  --output out/limier/build-summary.md

limier render \
  --format github-comment \
  --input out/limier/report.json \
  --output out/limier/comment.md
```

In this path, `report.json` is the source of truth. Rendered outputs are alternate presentations for CI summaries, comments, or chat notifications.

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
