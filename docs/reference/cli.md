# CLI Reference

Limier exposes five top-level commands:

- `limier run`
- `limier ci github`
- `limier inspect`
- `limier render`
- `limier version`

## `limier run`

Use this to compare one dependency upgrade in an isolated fixture.

```sh
limier run \
  --ecosystem npm \
  --package left-pad \
  --current 1.0.0 \
  --candidate 1.1.0 \
  --report out/limier/report.json \
  --summary out/limier/summary.md \
  --evidence out/limier/evidence
```

Flags:

- `--ecosystem`: ecosystem adapter to use
- `--package`: dependency name to compare
- `--current`: baseline version
- `--candidate`: candidate version
- `--fixture`: path to the sample application directory, or a preset. Defaults to `preset:<ecosystem>-require` for `npm`, `pip`, and `cargo`.
- `--scenario`: path to the scenario manifest, or a preset. Defaults to `preset:<ecosystem>-ci` for `npm`, `pip`, and `cargo`.
- `--rules`: path to the rules file, or a preset. Defaults to `preset:default`.
- `--report`: path for the JSON report, default `report.json`
- `--summary`: path for the Markdown summary, default `summary.md`
- `--evidence`: directory for evidence files, default `evidence`
- `--telemetry-mode`: optional telemetry override, either `required` or `off`. The scenario value applies when omitted.
- `--fail-on`: optional comma-separated recommendations that should fail the command. Empty preserves Limier's default exit-code behavior.

Supported ecosystems today:

- `cargo`
- `npm`
- `pip`

## `limier ci github`

Use this in GitHub Actions wrappers. It reads Dependabot metadata from environment variables, applies CI defaults, writes rendered outputs, and exits according to `--fail-on`.

```sh
limier ci github \
  --output-dir out/limier \
  --fail-on block,rerun
```

The command writes:

- `out/limier/report.json` when Limier runs
- `out/limier/summary.md`
- `out/limier/build-summary.md`
- `out/limier/comment.md`
- `out/limier/status.json`
- `out/limier/pr.txt` when a pull request number is available

Flags:

- `--output-dir`: directory for CI outputs, default `out/limier`
- `--fail-on`: comma-separated recommendations that should fail the command, default `block,rerun`
- `--ecosystem`, `--package`, `--current`, `--candidate`: optional metadata overrides
- `--fixture`, `--scenario`, `--rules`: optional path or preset overrides
- `--telemetry-mode`: optional telemetry override, either `required` or `off`
- `--dependency-files-changed`: whether dependency-relevant files changed, one of `true`, `false`, or `unknown`

`--dependency-files-changed` can also be supplied with `LIMIER_CI_DEPENDENCY_FILES_CHANGED`. When no dependency metadata is available, Limier only returns `not_applicable` if this signal is `false`. A `true` or missing/`unknown` signal returns `needs_review` so dependency-file pull requests do not pass as unrelated changes.

When dependency metadata is complete, the GitHub CI integration has generic default presets for `npm`, `pip`, and `cargo`. Pass `--fixture` and `--scenario` when your project needs a richer, project-specific behavioral check.

## `limier inspect`

Use this when you already have a `report.json` file and want a concise explanation without rerunning the fixture.

```sh
limier inspect --input out/limier/report.json
```

Flags:

- `--input`: existing `report.json`
- `--output`: optional output file for the inspection text

## `limier render`

Use this to turn an existing report into a downstream surface such as a CI summary or PR comment.

```sh
limier render --format build-summary --input out/limier/report.json
```

Flags:

- `--format`: one of `github-comment`, `gitlab-note`, or `build-summary`
- `--input`: existing `report.json`
- `--output`: optional output file

## `limier version`

Print the current Limier version:

```sh
limier version
```

## Logging

Limier also supports simple logging controls through environment variables:

- `LIMIER_LOG_LEVEL`: `debug`, `info`, `warn`, or `error`
- `LIMIER_LOG_FORMAT`: set to `json` for structured output; any other value, or leaving it unset, uses the default text format

Example:

```sh
LIMIER_LOG_LEVEL=debug LIMIER_LOG_FORMAT=json limier run ...
```
