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
