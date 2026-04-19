# Limier CI Examples

These examples all call the same repository-owned sample runner:

```sh
sh ./examples/ci/run-sample.sh
```

That script uses real repository assets:

- `fixtures/npm-app`
- `scenarios/npm.yml`
- `rules/default.yml`

The CI contract stays intentionally small:

1. run `limier run`
2. preserve `report.json`, `summary.md`, and `evidence/`
3. optionally run `limier render` for the target surface

They are thin wrappers around the same report-driven workflow. None of them recalculate the verdict.

Docker is still required at runtime because Limier executes fixtures in containers.

## Dependabot Usage

The files in this directory are sample wrappers around the repository-owned demo assets. They are useful for proving out the CI contract, but they are not a drop-in Dependabot integration.

For a real Dependabot workflow, the repository should usually add:

1. `.github/dependabot.yml` so Dependabot opens update pull requests.
2. A `pull_request` workflow or workflow job gated on `github.event.pull_request.user.login == 'dependabot[bot]'`.
3. A metadata step such as `dependabot/fetch-metadata` so the workflow can pass the dependency name, ecosystem, previous version, and new version into `limier run`.
4. A CI-specific scenario when running on GitHub-hosted runners, because hosted runners should not be assumed to have `bpftrace` available.

A minimal Dependabot gate looks like this:

```yaml
on:
  pull_request:
    types: [opened, synchronize, reopened]

jobs:
  limier:
    if: github.event.pull_request.user.login == 'dependabot[bot]'
```

If you want Limier to post comments, add labels, or enable auto-merge, keep that behavior in a separate privileged follow-up workflow rather than running pull request code from `pull_request_target`.
