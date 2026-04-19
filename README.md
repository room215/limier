# Limier

Limier is a fixture-based dependency behavior review tool. It compares a baseline package version with a candidate version, captures the behavior each one triggers in a controlled sample application, and turns the diff into one of four reviewer-facing outcomes:

- `good_to_go`
- `needs_review`
- `block`
- `rerun`

Limier is intentionally narrow. It is for suspicious or exploit-like dependency behavior such as new process execution, unexpected shelling out, changed install-time behavior, or other observable runtime drift. It is not a general application security scanner and it does not try to find `SQLi`, `XSS`, CSRF, or broad secure-coding flaws in the fixture itself.

Real host-signal capture is Linux-only and currently requires `bpftrace`. If `capture_host_signals` is enabled and Limier cannot start that backend, the run becomes inconclusive so process-coverage gaps are never hidden. On non-Linux systems, set `capture_host_signals: false` to use stdout/stderr-only comparison.

## Quick Start

Run the repository-owned npm sample:

```sh
sh ./examples/ci/run-sample.sh
```

That sample uses:

- fixture: `fixtures/npm-app`
- scenario: `scenarios/npm.yml`
- rules: `rules/default.yml`

The script writes:

- `out/limier/report.json`
- `out/limier/summary.md`
- `out/limier/build-summary.md`
- `out/limier/evidence/`

## Dependabot Integration

Limier can plug into Dependabot pull requests, but that integration lives in repository wiring rather than in Limier's core CLI.

For a GitHub-based setup, the repository should add:

- a `.github/dependabot.yml` file so Dependabot opens update pull requests
- a workflow or workflow job triggered by `pull_request`
- a Dependabot-only gate such as `if: github.event.pull_request.user.login == 'dependabot[bot]'`
- a metadata step such as `dependabot/fetch-metadata` to map the pull request to `--ecosystem`, `--package`, `--current`, and `--candidate`

The safest default is to run Limier in the unprivileged `pull_request` context and keep any commenting, labeling, or auto-merge behavior in a separate privileged follow-up workflow if needed. Avoid checking out and running pull request code from `pull_request_target`.

For GitHub-hosted runners, assume Docker is available but full host-signal capture is not. A stock hosted runner should therefore use a CI-specific scenario with `capture_host_signals: false`, or you should run Limier on a self-hosted Linux runner with `bpftrace` installed when full host telemetry is required.

The sample runner in `examples/ci/run-sample.sh` is intentionally hardcoded to a repository-owned demo dependency upgrade. It is a runnable example, not the Dependabot glue layer.

## Container Image

Release tags also publish a hardened OCI image to GitHub Container Registry at `ghcr.io/room215/limier` (forks should substitute their own repository path). Published image tags use normalized semver such as `0.1.1`, `0.1`, `0`, and `latest`.

The image is intentionally minimal:

- statically linked `limier` binary
- Docker CLI included because Limier shells out to `docker`
- distroless runtime
- non-root default user
- no package manager or shell in the final image

When running Limier from the container against a host Docker daemon, mount your repository at the same absolute path inside the container that it has on the host. This keeps Limier's fixture paths valid when the inner Docker daemon bind-mounts them. The example below also runs as your local UID/GID so it can write `out/limier/report.json`, `out/limier/summary.md`, and `out/limier/evidence/` back into the host checkout.

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

On Linux, if the Docker socket is group-owned, add `--group-add "$(getent group docker | cut -d: -f3)"` alongside `--user` so the mapped user can still talk to `/var/run/docker.sock`.

If your Docker socket is still not accessible in that environment, override the container user explicitly instead.

For example, this is the most portable fallback when the mounted socket is root-owned:

```sh
docker run --rm \
  --user 0:0 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD:$PWD" \
  -w "$PWD" \
  ghcr.io/room215/limier:<version> version
```

For the easiest containerized setup, set `capture_host_signals: false` in the scenario. Full host-signal capture still requires Linux plus `bpftrace` and additional host integration.

## Core Commands

Build and test with the standard Go toolchain:

```sh
go build ./...
go test ./...
go vet ./...
gofmt -w .
```

Run Limier directly:

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

Inspect or render an existing report:

```sh
go run . inspect --input out/limier/report.json
go run . render --format build-summary --input out/limier/report.json
```

## Phase 7 Assets

- Sample fixture and scenario: `fixtures/` and `scenarios/`
- Default and sample-specific rules: `rules/`
- Validation corpus and expected outcomes: `validation/corpus/`
- Launch-readiness notes and reviewer journeys: `docs/launch-readiness.md`
