# Use Limier In CI

Limier fits into CI best when you keep the contract small:

1. run `limier run`
2. preserve `report.json`, `summary.md`, and `evidence/`
3. optionally run `limier render` for the surface you want to publish

The report is the source of truth. Rendered outputs are just alternate presentations of the same result.

## Minimal GitHub Actions Example

This repository includes a small sample workflow:

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

## Dependabot Pull Requests

Limier can review Dependabot upgrades, but that wiring belongs in your repository workflow rather than inside Limier itself.

For GitHub, the usual setup is:

- add `.github/dependabot.yml`
- trigger a workflow on `pull_request`
- gate the job with `github.event.pull_request.user.login == 'dependabot[bot]'`
- use PR metadata to fill in `--ecosystem`, `--package`, `--current`, and `--candidate`

A minimal gate looks like this:

```yaml
on:
  pull_request:
    types: [opened, synchronize, reopened]

jobs:
  limier:
    if: github.event.pull_request.user.login == 'dependabot[bot]'
```

That read-only review job is a good default, but remember that `pull_request` runs from forks and Dependabot PRs get a read-only `GITHUB_TOKEN`, so comment, label, merge, or other write-back actions should happen in a separate privileged follow-up workflow such as `workflow_run`.

::: warning Avoid `pull_request_target` for the review run
The safest default is to run Limier in the unprivileged `pull_request` context and keep commenting, labeling, or auto-merge behavior in a separate privileged follow-up workflow if you need it.
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
