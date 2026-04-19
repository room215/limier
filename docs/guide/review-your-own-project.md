# Review Your Own Project

Once you have seen the sample run work, the next step is to point Limier at a fixture that represents your real dependency upgrade.

## The Three Inputs Limier Needs

Every review needs three things:

1. A fixture directory that uses the dependency you care about
2. A scenario file that tells Limier how to install and exercise the fixture
3. A rules file that decides which changes are benign, review-worthy, or blocking

## Step 1: Prepare A Fixture

A fixture is a small sample application that depends on the package you want to review.

Choose the smallest realistic app that still reproduces the behavior you care about. Good fixtures are:

- deterministic
- quick to run
- easy to understand
- close enough to production behavior to be meaningful

### Fixture Requirements By Ecosystem

- `npm`: the fixture must contain a `package.json`, and the dependency under test must appear in `dependencies` or `devDependencies`
- `pip`: the fixture must contain a `requirements.txt`, and the dependency under test must be declared there
- `cargo`: the fixture must contain a `Cargo.toml`, and the dependency under test must be declared in Cargo dependency tables

::: tip Keep the target dependency simple at first
For `npm`, Limier currently expects the dependency under test to use a version-style spec. Source-backed specs such as `workspace:`, `file:`, `git:`, `github:`, or `http:` are not a good first target.
:::

## Step 2: Write A Scenario File

The scenario describes what Limier should do inside the fixture.

Here is a minimal `npm` example:

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

Rules to remember:

- you must have at least one `install` step
- `install` steps do not take a `command`; the adapter supplies it
- every non-install step must have a `command`
- `repeats: 2` is a good starting point because it helps Limier detect unstable behavior

See [Scenario File](/reference/scenario-file) for the full reference.

## Step 3: Start With The Default Rules

Most users should begin with `rules/default.yml`.

That ruleset already treats things like new download commands, shell pipes, and unexpected process execution as review or block conditions.

If your fixture has known benign noise, create a copy and add a suppression rule rather than weakening the defaults globally.

See [Rules File](/reference/rules-file) for examples.

## Step 4: Run The Review

Use `limier run` with your ecosystem, package name, versions, fixture, scenario, and rules:

```sh
limier run \
  --ecosystem npm \
  --package left-pad \
  --current 1.0.0 \
  --candidate 1.1.0 \
  --fixture path/to/fixture \
  --scenario path/to/scenario.yml \
  --rules path/to/rules.yml \
  --report out/limier/report.json \
  --summary out/limier/summary.md \
  --evidence out/limier/evidence
```

The current supported values for `--ecosystem` are:

- `npm`
- `pip`
- `cargo`

## Step 5: Read The Result

Start with `summary.md`. If the recommendation is:

- `good_to_go`, the change did not hit anything suspicious enough to stop approval with the current ruleset
- `needs_review`, inspect the findings and evidence before approving
- `block`, treat the change as suspicious until it is explained or removed
- `rerun`, fix the environment or instability first

For a more structured explanation of an existing report:

```sh
limier inspect --input out/limier/report.json
```

## Practical Tips

- Keep fixture commands deterministic. Flaky tests produce noisy reruns.
- Disable network with `network.mode: none` when the scenario does not need external access.
- Turn off `capture_host_signals` on non-Linux machines or hosted runners without `bpftrace`.
- Upload `evidence/` as a CI artifact so reviewers can inspect raw stdout, stderr, and event capture later.
