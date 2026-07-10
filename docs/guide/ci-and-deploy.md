# Use Limier In CI

For GitHub Actions, use the first-party [`room215/limier-action`](https://github.com/room215/limier-action) wrapper. It installs Limier, detects dependency-relevant pull request changes, maps Dependabot metadata, publishes the build summary, uploads artifacts, and offers a companion comment action.

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
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: Run Limier dependency review
        uses: room215/limier-action@v1
```

Use raw `limier run`, `limier ci github`, and `limier render` when you are building a custom integration or using a CI system where you own metadata extraction and publishing.

## `limier ci github`

`limier ci github` is the CI-oriented command that GitHub wrappers call. It reads dependency metadata from flags or environment variables, decides whether Limier can safely run, writes CI-facing artifacts, and exits according to policy.

By default, it fails the job for `block` and `rerun` recommendations:

```sh
limier ci github --output-dir out/limier --fail-on block,rerun
```

That means an inconclusive run does not pass as safe. A `needs_review` result writes a review-required status and succeeds unless you add `needs_review` to `--fail-on`.

## What The Command Writes

`limier ci github` writes these files under `--output-dir`, which defaults to `out/limier`:

- `build-summary.md`: Markdown designed for `$GITHUB_STEP_SUMMARY`
- `comment.md`: Markdown suitable for a pull request comment
- `status.json`: machine-readable status, recommendation, paths, metadata, and policy exit code
- `summary.md`: the human-readable Limier summary
- `report.json`: the full structured report when Limier actually runs
- `evidence/`: raw stdout, stderr, and event evidence when Limier actually runs
- `pr.txt`: the pull request number when GitHub event data is available

The command renders `build-summary.md`, `comment.md`, and `status.json` even when Limier skips the behavioral diff. That lets a wrapper publish a clear result for `not_applicable`, `needs_review`, and `rerun` paths.

`comment.md` is only an artifact. `limier ci github` does not post comments, label pull requests, approve changes, or merge anything.

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

When dependency metadata is complete, the GitHub CI integration has generic default presets for `npm`, `pip`, and `cargo`. Pass `--fixture` and `--scenario` when your project needs a richer, project-specific behavioral check.

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
