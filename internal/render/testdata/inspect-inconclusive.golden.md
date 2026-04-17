# Limier Inspect

- Package: requests `2.31.0` -> `2.32.0` in `pip`
- Technical verdict: `inconclusive`
- Operator recommendation: `rerun`
- Exit code: `2`
- Baseline stability: 1 repeat(s), stable `true`
- Candidate stability: 0 repeat(s), stable `false`
- Diagnostic status: structured inconclusive diagnostic available.

## Diagnostic

- Category: `docker_failure`
- Code: `candidate_docker_run_failed`
- Summary: run candidate scenario: docker exec "exercise": exit status 1
- Suggested action: Confirm Docker is available and healthy on the runner, then rerun Limier.
- Diagnostic evidence: /tmp/limier/evidence/candidate/run-1

## What To Inspect

- Open the evidence under `/tmp/limier/evidence` and start with the diagnostic summary above.
