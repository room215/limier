# Use With Codex

Limier itself is a normal Go CLI. It does not have a built-in agent plugin system.

If you want an AI agent to work with this repository effectively, the current Codex model is:

- a **skill** for lightweight repo-specific guidance
- a **plugin** only when you need packaging, marketplace installation, or extra integrations such as MCP servers, apps, or hooks

For this repository, the recommended starting point is the repo-local skill:

- [`.agents/skills/limier-cli/SKILL.md`](https://github.com/room215/limier/blob/main/.agents/skills/limier-cli/SKILL.md)

## What The Repo Skill Does

The `limier-cli` skill helps an agent choose the right Limier command for the task:

- `run` for a fresh dependency behavior review
- `inspect` to explain an existing `report.json`
- `render` to turn an existing report into a CI or PR-facing surface

It also captures the repo defaults an agent should know before acting:

- sample fixture: `fixtures/npm-app`
- sample scenario: `scenarios/npm.yml`
- default rules: `rules/default.yml`
- default outputs: `out/limier/`

## When A Skill Is Enough

Use the repo-local skill when you want:

- better agent behavior in this repository
- instructions that stay versioned with the codebase
- no marketplace or installation overhead
- a simple way to teach an agent about Limier workflows, outputs, and failure modes

This is the best default for maintainers working directly in the repository.

## When To Promote It To A Plugin

Create a plugin only if you want one or more of these:

- distribution outside the repo
- installable marketplace packaging
- multiple bundled skills
- MCP server configuration
- app integrations
- hooks or additional plugin assets

In Codex terms, a plugin is the packaging layer around skills and integrations. The skill usually comes first.

## Suggested Layouts

### Repo-local skill

```text
.agents/
  skills/
    limier-cli/
      SKILL.md
      agents/
        openai.yaml
```

### Repo-local plugin

```text
plugins/
  limier/
    .codex-plugin/
      plugin.json
    skills/
      ...
.agents/
  plugins/
    marketplace.json
```

## Recommended Path For This Project

1. Keep the repo-local `limier-cli` skill as the source of truth for agent guidance.
2. Expand it only when the repo gains new Limier workflows, outputs, or supporting scripts.
3. Add a plugin later if Limier needs to be distributed as a reusable Codex package rather than just documented in this repository.

## Maintaining The Skill

Update the skill when any of these change:

- supported commands or flags
- default fixture, scenario, or rules paths
- verdict semantics
- output locations
- guidance for handling `rerun` and inconclusive runs

Because the skill is checked into the repo, agent guidance can evolve alongside the CLI and docs.
