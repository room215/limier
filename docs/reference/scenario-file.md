# Scenario File

The scenario file tells Limier how to run your fixture.

It answers questions like:

- which container image should be used
- how many times should each side be repeated
- how should the dependency be installed
- which commands count as “exercising” the fixture
- should networking or host-signal capture be enabled

## Minimal Example

```yaml
version: 1
name: npm dependency review
repeats: 2
image: node:22
workdir: /workspace
network:
  mode: default
evidence:
  capture_host_signals: false
success:
  exit_code: 0
steps:
  - name: install dependency
    run: install
  - name: exercise package
    run: exercise
    command: node index.js
```

## Top-Level Fields

### `version`

Required. Must be `1`.

### `name`

Required. A human-readable name for the scenario.

### `repeats`

Optional. Defaults to `2`.

Higher values can help catch flaky behavior, but also make runs slower.

### `image`

Optional. Docker image to use for the fixture.

If omitted, Limier uses the adapter default:

- `npm`: `node:22`
- `pip`: `python:3.12`
- `cargo`: `rust:1`

### `workdir`

Optional. Defaults to `/workspace`.

This is where Limier mounts the fixture inside the container.

### `env`

Optional map of environment variables to pass into every step.

Limier also adds these built-in variables automatically:

- `LIMIER_SIDE`
- `LIMIER_RUN_INDEX`
- `LIMIER_PACKAGE`
- `LIMIER_VERSION_UNDER_TEST`

### `network.mode`

Optional. Allowed values:

- `default`
- `none`

Use `none` when your scenario should not reach the network.

### `mounts`

Optional extra bind mounts.

Each mount has:

- `source`
- `target`
- `read_only`

Relative `source` paths are resolved relative to the scenario file location.

### `evidence.capture_host_signals`

Optional. Defaults to `true`.

Set it to `false` on non-Linux systems or CI environments without `bpftrace`.

### `success.exit_code`

Optional. Defaults to `0`.

This tells Limier which overall exit code counts as a successful scenario.

## Steps

The `steps` array is required.

Rules:

- you must define at least one step
- you must include at least one `install` step
- `install` steps must not include `command`
- non-install steps must include `command`

### `run: install`

An install step uses the adapter-provided install command for the current ecosystem.

Examples:

- `npm`: `npm install`
- `pip`: create a virtualenv and install from `requirements.txt`
- `cargo`: adapter-provided Cargo install flow

### Non-install steps

For everything else, provide the command explicitly:

```yaml
steps:
  - name: install dependency
    run: install
  - name: run tests
    run: test
    command: npm test
```

## Good Scenario Design

Good scenarios are:

- deterministic
- short
- meaningful
- explicit about whether network access is allowed

Avoid long integration suites or flaky commands as a first scenario. Start small, then expand only if you need more coverage.
