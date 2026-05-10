# Divinity

Divinity is a terminal-first orchestration system for coding agents. It runs the same task through one or more configured agents in isolated Git worktrees, validates the results, captures diffs and logs, and recommends the strongest candidate while keeping final approval with the user.

This repository currently contains the Phase 1 MVP foundation.

## What Works Now

- `divinity init` creates `.divinity/` and a starter `divinity.yaml`.
- `divinity run "task"` runs selected agents in isolated worktrees.
- `divinity compare "task"` runs the same task across multiple agents.
- Agent output, validation logs, diffs, and run metadata are saved under `.divinity/runs/`.
- `divinity status` lists previous runs.
- `divinity review [run-id]` prints a saved comparison summary.

## Requirements

- Go 1.22 or newer
- Git
- At least one real agent command configured in `divinity.yaml`

The MVP intentionally requires Git because worktrees are the safety boundary.

## Quick Start

```sh
go install ./...
divinity init
```

Edit `divinity.yaml`:

```yaml
version: 1
agents:
  - name: gemini
    type: shell
    command: gemini
    args:
      - --prompt
      - "{{task}}"
  - name: custom-script
    type: shell
    command: ./scripts/my-agent
    args:
      - "{{task}}"
validation:
  - name: tests
    command: go
    args: ["test", "./..."]
preferences:
  max_parallel_agents: 2
```

Run a task:

```sh
divinity compare "Add validation for empty task names"
```

Review the result:

```sh
divinity review
```

## Agent Contract

Shell agents receive:

- Working directory set to their isolated worktree.
- The task as arguments through `{{task}}` template expansion.
- Environment variables:
  - `DIVINITY_TASK`
  - `DIVINITY_WORKTREE`
  - `DIVINITY_RUN_ID`
  - `DIVINITY_AGENT`
  - `DIVINITY_PROJECT_ROOT`

Supported template values:

- `{{task}}`
- `{{worktree}}`
- `{{run_id}}`
- `{{agent}}`
- `{{project_root}}`

## Project Layout

```text
internal/agent         agent adapters
internal/cli           Cobra commands
internal/config        project configuration
internal/eval          candidate scoring
internal/execx         process helpers
internal/model         persisted result structures
internal/orchestrator  run coordination
internal/store         filesystem persistence
internal/tui           terminal summaries
internal/workspace     Git worktree isolation
```

## Roadmap

Phase 1 is focused on making one useful loop reliable:

1. Run one task through multiple agents.
2. Keep every agent isolated.
3. Validate each candidate.
4. Compare diffs and outcomes.
5. Let the user approve the final implementation.

Future phases can add richer councils, API providers, local model adapters, SQLite persistence, and adaptive agent selection.
