# Divinity

Divinity is a terminal-first orchestration system for coding agents. It runs the same task through one or more configured agents in isolated Git worktrees, validates the results, captures diffs and logs, and recommends the strongest candidate while keeping final approval with the user.

This repository currently contains the Phase 1 MVP foundation.

## What Works Now

- `divinity` opens the interactive terminal workspace and bootstraps local Divinity files if needed.
- `divinity app` opens the same interactive workspace explicitly.
- `divinity init` creates `.divinity/` and a starter `divinity.yaml`.
- `divinity doctor` checks Git, config, agents, environment variables, and local model endpoints.
- `divinity agents` lists configured agents and their key connection details.
- `divinity presets` prints copyable setup snippets for common agents.
- `divinity run "task"` runs selected agents in isolated worktrees.
- `divinity compare "task"` runs the same task across multiple agents.
- `divinity diff [run-id]` prints the recommended candidate diff.
- `divinity apply [run-id] --yes` applies an approved candidate diff to the current workspace.
- Agent output, validation logs, diffs, and run metadata are saved under `.divinity/runs/`.
- `divinity status` lists previous runs.
- `divinity review [run-id]` prints a saved comparison summary.
- The interactive TUI supports `/run`, `/agents`, `/status`, `/review`, `/diff`, `/logs`, `/apply yes`, `/config`, `/clear`, `/help`, and `/quit`.

## Requirements

- Go 1.22 or newer
- Git
- At least one real agent command configured in `divinity.yaml`

The MVP intentionally requires Git because worktrees are the safety boundary.

## Quick Start

```sh
go install ./...
divinity
```

Running `divinity` creates `.divinity/` and a starter `divinity.yaml` if they do not exist, then opens the interactive TUI.

On a new project, the TUI opens a provider setup wizard. Type a number, press Enter, answer the prompts, and Divinity writes `divinity.yaml` for you.

Provider choices include:

```text
1. Claude Code
2. Gemini CLI
3. OpenCode
4. Aider
5. OpenAI-compatible API
6. Groq
7. Ollama
8. Skip setup
```

You can reopen the wizard anytime:

```text
/setup
```

You can also print copyable snippets outside the TUI:

```powershell
divinity presets
divinity presets gemini
divinity presets claude
divinity presets opencode
divinity presets aider
divinity presets groq
divinity presets ollama
```

Copy one snippet into `divinity.yaml`, then check it:

```powershell
divinity doctor
divinity agents
```

Edit `divinity.yaml`:

```yaml
version: 1
agents:
  - name: gemini
    type: shell
    command: gemini
    args:
      - --approval-mode=yolo
      - --prompt
      - |
        You are Gemini CLI running as a Divinity coding agent inside an isolated Git worktree.
        Make the requested code or file changes directly in the current working directory.
        Do not only describe the solution. Create, edit, and run commands as needed.

        Task:
        {{task}}
    env:
      GEMINI_SANDBOX: "false"
  - name: custom-script
    type: shell
    command: ./scripts/my-agent
    args:
      - "{{task}}"
  - name: groq-reviewer
    type: openai-compatible
    base_url: https://api.groq.com/openai/v1
    api_key_env: GROQ_API_KEY
    model: openai/gpt-oss-20b
    output_file: DIVINITY_GROQ_REVIEW.md
    system: >
      You are a senior coding reviewer. Review the task and produce a concise
      implementation plan, risks, and test recommendations.
validation:
  - name: tests
    command: go
    args: ["test", "./..."]
preferences:
  max_parallel_agents: 2
```

Run a task:

```sh
divinity
```

Inside the TUI, type a task and press Enter:

```text
Add validation for empty task names
```

After a run finishes inside the TUI:

```text
/review
/diff
/logs
/apply yes
```

You can also run the non-interactive comparison command:

```sh
divinity compare "Add validation for empty task names"
```

Review the result:

```sh
divinity review
```

Inspect and approve the winning diff:

```powershell
divinity diff
divinity apply --yes
```

`apply` refuses to run on a dirty working tree by default. It applies the patch but does not commit it.

Before handing Divinity to another tester, run:

```powershell
go run . doctor
go run . agents
go run . presets
```

`doctor` is the fastest way to catch common setup problems: missing Git commits, missing CLI commands, missing API key environment variables, and local model servers that are not running.

## Connecting Models

Divinity supports two practical agent styles:

- Shell coding agents that can read and edit files: Claude Code, Gemini CLI, OpenCode, Aider, custom scripts.
- OpenAI-compatible API/local models that are best for review, planning, and judging: Groq, OpenAI-compatible APIs, Ollama, LM Studio, vLLM, OpenRouter.

Use presets to get started:

```powershell
divinity presets claude
divinity presets gemini
divinity presets opencode
divinity presets aider
divinity presets groq
divinity presets ollama
```

Cloud/API keys should be environment variables:

```powershell
$env:GROQ_API_KEY="your-key"
```

Local models need their server running first:

```powershell
ollama serve
ollama pull qwen2.5-coder:1.5b
```

## Agent Contract

### Shell Agents

Shell agents are the best choice when you want Divinity to drive a real coding tool that can edit files, such as Gemini CLI, Claude Code, OpenCode, Aider, or a custom script.

For Gemini CLI, Divinity uses headless prompt mode plus `--approval-mode=yolo` so the process can create files, edit files, and run tools without stopping for an interactive approval prompt. The default Gemini preset also sets `GEMINI_SANDBOX=false`; Divinity's safety boundary is the isolated Git worktree, and your main checkout only changes after you review and apply the resulting diff. Some Gemini CLI builds do not support `--all-files`, so the preset relies on the worktree as the current working directory instead.

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

### OpenAI-Compatible API Agents

Divinity can also call OpenAI-compatible chat completion providers. This is useful for reviewer, planner, judge, or lightweight implementer roles.

API agents currently write their response to a file in the isolated worktree, then Divinity captures that diff. They are useful immediately for review and planning. For full code editing, prefer a shell agent that can directly modify files.

Groq example:

```powershell
$env:GROQ_API_KEY="your-key"
```

```yaml
agents:
  - name: groq-reviewer
    type: openai-compatible
    base_url: https://api.groq.com/openai/v1
    api_key_env: GROQ_API_KEY
    model: openai/gpt-oss-20b
    output_file: DIVINITY_GROQ_REVIEW.md
    system: >
      You are a senior coding reviewer. Produce concrete feedback,
      implementation risks, and validation recommendations.
```

Local server examples:

```yaml
agents:
  - name: ollama-reviewer
    type: openai-compatible
    base_url: http://localhost:11434/v1
    model: qwen2.5-coder:1.5b
    output_file: DIVINITY_OLLAMA_REVIEW.md
    system: >
      You are a local coding reviewer. Inspect the task and produce practical,
      concise next steps.

  - name: lm-studio
    type: openai-compatible
    base_url: http://localhost:1234/v1
    model: local-model
    output_file: DIVINITY_LOCAL_REVIEW.md

  - name: vllm-local
    type: openai-compatible
    base_url: http://localhost:8000/v1
    model: Qwen/Qwen2.5-Coder-7B-Instruct
    output_file: DIVINITY_VLLM_REVIEW.md
```

Recommended setup for useful coding today:

```yaml
agents:
  - name: gemini-implementer
    type: shell
    command: gemini
    args:
      - --approval-mode=yolo
      - --prompt
      - |
        You are Gemini CLI running as a Divinity coding agent inside an isolated Git worktree.
        Make the requested code or file changes directly in the current working directory.
        Do not only describe the solution. Create, edit, and run commands as needed.

        Task:
        {{task}}
    env:
      GEMINI_SANDBOX: "false"

  - name: groq-reviewer
    type: openai-compatible
    base_url: https://api.groq.com/openai/v1
    api_key_env: GROQ_API_KEY
    model: openai/gpt-oss-20b
    system: >
      Review the implementation approach and identify risks, missing tests,
      and simpler alternatives.
```

That gives Divinity one agent that can edit code and one fast model that can review or judge the result.

## Testing Agentic Behavior

API agents such as Groq can reason, plan, and review, but they do not directly inspect or edit your files unless you build a tool loop around them. To test true agentic behavior in Divinity today, use a shell agent. Shell agents run inside isolated Git worktrees and can read/write files.

This repo includes a small smoke-test shell agent:

```yaml
agents:
  - name: smoke-agent
    type: shell
    command: powershell
    args:
      - -ExecutionPolicy
      - Bypass
      - -File
      - "{{project_root}}/scripts/divinity-smoke-agent.ps1"
      - "{{task}}"
```

Run it:

```powershell
go run . compare "Inspect the repo and create an agentic smoke-test report" --agent smoke-agent --no-tui
```

You should see `files=1` and a diff path under `.divinity/runs/<run-id>/smoke-agent/diff.patch`. That proves the agent ran in a worktree, inspected files, created `DIVINITY_AGENTIC_SMOKE.md`, and Divinity captured the diff.

To inspect the temporary worktree before cleanup:

```powershell
go run . compare "Inspect the repo and create an agentic smoke-test report" --agent smoke-agent --keep-worktrees
```

For real AI coding, replace `smoke-agent` with a coding CLI:

```yaml
agents:
  - name: aider
    type: shell
    command: aider
    args: ["--message", "{{task}}"]

  - name: opencode
    type: shell
    command: opencode
    args: ["run", "{{task}}"]
```

Exact arguments depend on the CLI you install, but the Divinity side is the same: command runs in the isolated worktree, edits files, then Divinity evaluates the diff.

## Ollama

Ollama exposes an OpenAI-compatible API at:

```text
http://localhost:11434/v1
```

Install Ollama, then pull a small coding model:

```powershell
ollama pull qwen2.5-coder:1.5b
```

Add it to `divinity.yaml`:

```yaml
agents:
  - name: ollama-reviewer
    type: openai-compatible
    base_url: http://localhost:11434/v1
    model: qwen2.5-coder:1.5b
    output_file: DIVINITY_OLLAMA_REVIEW.md
```

Test it:

```powershell
go run . compare "Review the repo and suggest the next implementation step" --agent ollama-reviewer --no-tui
```

This gives you local private reasoning, but it is still a reviewer/planner style agent. To make Ollama truly agentic, Divinity needs a tool loop that lets the model ask to read files, write files, and run commands inside the isolated worktree. That is the next capability to build.

### Agentic Ollama Tool Loop

Divinity also includes an experimental `agentic` agent type. It lets an OpenAI-compatible model request controlled tools inside the isolated worktree:

- `list_files`
- `read_file`
- `write_file`
- `run_command`
- `finish`

Example:

```yaml
agents:
  - name: ollama-agentic
    type: agentic
    base_url: http://localhost:11434/v1
    model: qwen2.5-coder:1.5b
    max_steps: 8
    allowed_commands:
      - git status --short
      - go test ./...
```

Run it:

```powershell
go run . compare "Create a short AGENTIC_TEST.md file explaining what Divinity is" --agent ollama-agentic --no-tui --keep-worktrees
```

The model must return JSON tool calls, so smaller models can be inconsistent. If it fails with invalid JSON, try a stronger local coding model or increase `max_steps`.

The `run_command` tool only executes commands explicitly listed in `allowed_commands`.

## Tester Setup Checklist

For a new machine or collaborator:

1. Install Go and Git.
2. Clone or copy the repo.
3. Run `go test ./...`.
4. Run `go run . doctor`.
5. Configure at least one agent in `divinity.yaml`.
6. Run `go run . agents` to confirm Divinity sees the agent.
7. Use `go run . presets <name>` if they need a starter config snippet.
8. Run a smoke test:

```powershell
go run . compare "Create a short setup test file" --agent smoke-agent --no-tui
```

For cloud reviewers, set API keys as environment variables, never directly in `divinity.yaml`.

For local models, start the server first. For Ollama:

```powershell
ollama serve
ollama pull qwen2.5-coder:1.5b
go run . doctor
```

## MVP Tester Workflow

The current MVP loop testers should use is:

1. Run `divinity doctor`.
2. Run `divinity agents`.
3. Start Divinity with `divinity`.
4. Submit a task.
5. Review the result with `/review`.
6. Inspect the patch with `/diff`.
7. Inspect logs with `/logs`.
8. Apply the approved result with `/apply yes`.
9. Run project tests.
10. Commit manually if satisfied.

The same flow is available outside the TUI:

```powershell
divinity compare "<task>"
divinity review
divinity diff
divinity apply --yes
```

After applying:

```powershell
go test ./...
git diff
git commit -am "Describe approved Divinity changes"
```

The MVP intentionally does not auto-commit. Human approval remains the final gate.

Still missing for a stronger Claude Code/OpenCode-style MVP:

- Live streaming agent logs in the TUI while a task is running.
- A robust built-in agentic editor loop for strong hosted models, not just local experimental JSON tools.
- Better model profiles/presets for Gemini CLI, Claude Code, OpenCode, Aider, Groq, Ollama, LM Studio, and vLLM.
- Reviewer/judge roles that compare implementation agents rather than only scoring simple signals.
- Safer patch review UI with file-by-file approval.

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
