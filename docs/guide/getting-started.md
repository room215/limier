# Getting Started

This page is the fastest path to a first successful Limier run.

## What You Need

- Docker available to the current user
- One of these ways to run Limier:
  - a built binary
  - the published container image
  - a local Go toolchain if you want to build from source
- Linux plus `bpftrace` if you want host-signal capture

::: warning Host-signal capture is Linux-only
If `capture_host_signals` is enabled and Limier cannot start that backend, the run becomes inconclusive instead of silently dropping process coverage.

On macOS, Windows, or CI runners without `bpftrace`, set `capture_host_signals: false` in your scenario.
:::

## Installation Options

### Option 1: Download A Release Binary

Download a release asset from [GitHub Releases](https://github.com/room215/limier/releases) and place `limier` somewhere on your `PATH`.

Then confirm the install:

```sh
limier version
```

### Option 2: Use The Container Image

Release tags also publish an OCI image:

```sh
ghcr.io/room215/limier:<version>
```

This is a good fit for CI or environments where you do not want to manage a local install.

### Option 3: Build From Source

If you are working from this repository directly:

```sh
go build -o ./bin/limier .
./bin/limier version
```

## First Run: Use The Included Sample

The included sample fixture lives in this repository, so the commands in this section assume you are in a local checkout of `room215/limier`.

That sample compares:

- ecosystem: `npm`
- package: `left-pad`
- current version: `1.0.0`
- candidate version: `1.1.0`
- fixture: `fixtures/npm-app`
- scenario: `scenarios/npm.yml`
- rules: `rules/default.yml`

### If You Built Limier From Source

The fastest path is still the repository-owned sample runner:

```sh
sh ./examples/ci/run-sample.sh
```

That script builds `./bin/limier`, runs the review, and renders a build summary.

### If You Installed A Release Binary

Run the same sample directly with your installed `limier` binary:

```sh
mkdir -p out/limier

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
```

### If You Are Using The Container Image

From the repository checkout, mount the checkout at the same absolute path inside the container:

```sh
mkdir -p out/limier

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

docker run --rm \
  --user "$(id -u):$(id -g)" \
  -v "$PWD:$PWD" \
  -w "$PWD" \
  ghcr.io/room215/limier:<version> \
  render \
  --format build-summary \
  --input out/limier/report.json \
  --output out/limier/build-summary.md
```

The sample writes:

- `out/limier/report.json`
- `out/limier/summary.md`
- `out/limier/build-summary.md`
- `out/limier/evidence/`

## What To Look At First

After a run:

1. Open `summary.md` for the short answer.
2. Open `report.json` if you need the full structured result.
3. Open `evidence/` when you want the raw stdout, stderr, and event capture behind the verdict.

You can also re-explain a completed report without rerunning the fixture:

```sh
./bin/limier inspect --input out/limier/report.json
```

If you installed Limier on your `PATH`, use `limier inspect` instead. If you are using the container image, run `inspect` the same way you ran `run`, with the repository checkout bind-mounted into the container.

Or render the same report again for a downstream surface:

```sh
./bin/limier render --format build-summary --input out/limier/report.json
```

If you installed Limier on your `PATH`, use `limier render` instead.

## What The Outcomes Mean

Limier gives an operator recommendation:

- `good_to_go`: nothing suspicious enough was found with the current ruleset
- `needs_review`: the change may be legitimate, but a human should inspect it
- `block`: the change matched a hard-block rule and should not be approved yet
- `rerun`: the run was inconclusive or unstable

See [Understand Results](/guide/understand-results) for how to interpret each one.

## Next Steps

- Use [Review Your Own Project](/guide/review-your-own-project) when you are ready to point Limier at a real dependency upgrade.
- Use [Scenario File](/reference/scenario-file) and [Rules File](/reference/rules-file) when you want to customize how Limier runs and evaluates your fixture.
