# CLI Reference

Limier exposes four top-level commands:

- `limier run`
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
  --fixture fixtures/npm-app \
  --scenario scenarios/npm.yml \
  --rules rules/default.yml \
  --report out/limier/report.json \
  --summary out/limier/summary.md \
  --evidence out/limier/evidence
```

Flags:

- `--ecosystem`: ecosystem adapter to use
- `--package`: dependency name to compare
- `--current`: baseline version
- `--candidate`: candidate version
- `--fixture`: path to the sample application directory
- `--scenario`: path to the scenario manifest
- `--rules`: path to the rules file
- `--report`: path for the JSON report, default `report.json`
- `--summary`: path for the Markdown summary, default `summary.md`
- `--evidence`: directory for evidence files, default `evidence`

Supported ecosystems today:

- `cargo`
- `npm`
- `pip`

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
- `LIMIER_LOG_FORMAT`: `json` or text

Example:

```sh
LIMIER_LOG_LEVEL=debug LIMIER_LOG_FORMAT=json limier run ...
```
