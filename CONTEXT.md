# Divinity

> Multi-agent orchestration for coding agents.

Divinity is a terminal-first orchestration system designed to coordinate multiple coding agents (cloud, local, or CLI-based) in a unified workflow.

Instead of replacing tools like Claude Code, Gemini CLI, or OpenCode, Divinity acts as an orchestration layer above them.

The core idea is simple:

**One task → multiple agents → isolated execution → evaluation → best result**

---

# Vision

Modern coding agents are powerful but isolated.

Each tool solves tasks independently, but developers still manually coordinate:

- Planning
- Architecture decisions
- Parallel experimentation
- Code review
- Testing
- Comparing implementations
- Selecting the best outcome

Divinity exists to become the **conductor of coding agents**.

Think:

- Kubernetes for coding agents
- tmux for AI workflows
- CI/CD for agent execution

Divinity should feel:

- Fast
- Reliable
- Developer-first
- Local-first
- Provider-agnostic
- Transparent

No black boxes.

The user should always understand:

- What each agent is doing
- Why a decision was made
- What changed
- How results are evaluated

---

# Core Philosophy

## 1. Divinity does not replace agents

Divinity is **not another coding model**.

Divinity orchestrates existing systems.

### CLI agents

- Claude Code
- Gemini CLI
- OpenCode
- Aider
- Custom scripts

### API providers

- OpenAI-compatible APIs
- Anthropic
- OpenAI
- Gemini
- Groq
- Together
- OpenRouter

### Local models

- Ollama
- llama.cpp
- vLLM
- LM Studio

---

## 2. Provider agnostic by design

Users should never be locked into a single provider.

Every provider should implement a shared interface.

The orchestration layer must remain independent from model vendors.

Divinity should support:

- Cloud providers
- Local models
- CLI-based coding agents
- User-defined shell agents

---

## 3. Local-first

Divinity should work without paid APIs.

Users with weak hardware or no subscriptions should still be able to use:

- Lightweight local models
- Custom shell agents
- Hybrid workflows

Example workflow:

- Gemini CLI for implementation
- Small local model for review
- Human for final approval

---

## 4. Git-native workflows

All agent execution must be isolated.

Agents should **never directly mutate the main workspace**.

Preferred strategy:

- Git worktrees
- Temporary branches
- Isolated execution environments

Example structure:

- `.divinity/worktrees/task-123/gemini`
- `.divinity/worktrees/task-123/claude`
- `.divinity/worktrees/task-123/local`

Every agent should receive:

- An isolated branch
- An isolated filesystem
- A reproducible environment

Benefits:

- Safe experimentation
- Easy rollback
- Deterministic comparisons
- No accidental corruption of the main branch

---

# Product Goals

Divinity should enable:

## Multi-agent execution

Expected workflow:

1. Analyze task
2. Create isolated worktrees
3. Run multiple agents in parallel
4. Execute validation checks
5. Compare results
6. Recommend the best implementation
7. Let the user approve the final result

---

## Agent councils

Multiple agents collaborate together.

Potential roles:

### Planner

Breaks tasks into subtasks.

### Architect

Reviews architectural impact.

### Implementer

Writes code.

### Reviewer

Critiques implementations.

### Tester

Runs validation.

### Judge

Selects the best candidate.

---

## Comparative execution

The same task should be executable across multiple agents simultaneously.

Expected comparison dimensions:

- Tests passed
- Files changed
- Speed
- Cost
- Architectural quality
- Simplicity
- Reliability

Divinity should recommend the strongest implementation while keeping the user in control.

---

## Intelligent evaluation

Selection should not be random.

Evaluation signals should include:

- Test results
- Linting
- Type safety
- Diff quality
- Architectural consistency
- Token cost
- Execution speed
- User preferences

---

# Non-goals

Divinity is **not**:

- An IDE
- A code editor
- A proprietary AI model
- A replacement for Git
- A black-box autonomous coding system

Human approval matters.

The user remains in control.

---

# Technical Stack

## Primary language

### Go

Chosen because:

- Excellent CLI ecosystem
- Simple binary distribution
- Strong concurrency model
- Good process management
- Easy cross-platform support
- Fast development velocity

---

## CLI framework

### Cobra

Used for:

- Commands
- Flags
- Subcommands
- Configuration bootstrapping

Potential commands:

- `divinity init`
- `divinity run`
- `divinity review`
- `divinity compare`
- `divinity status`
- `divinity council`

---

## Terminal UI (TUI)

### Bubble Tea

Responsible for:

- State management
- Keyboard interactions
- Navigation
- Real-time updates

### Lip Gloss

Responsible for:

- Styling
- Borders
- Layouts
- Colors

### Bubbles

Reusable components:

- Tables
- Spinners
- Lists
- Progress bars
- Inputs

### Glamour

Markdown rendering in terminal.

---

## Persistence

### Phase 1

Filesystem-based persistence.

The `.divinity` directory stores:

- Logs
- Task history
- Execution metadata
- Diffs
- Configurations

### Phase 2

SQLite persistence.

Used for:

- Task history
- Evaluation metrics
- Cost tracking
- Agent performance
- Long-term memory

---

## Configuration

Project-level configuration should define:

- Available agents
- Roles
- Validation checks
- Model preferences
- Fallback behavior

Supported agent types:

- CLI agents
- OpenAI-compatible endpoints
- Local models
- Custom shell scripts

---

## Repository isolation

Git worktrees should be the default isolation mechanism.

Execution flow:

1. Create temporary worktree
2. Run agent
3. Execute validation
4. Generate diff
5. Present results
6. Optionally merge

The main branch should never be modified automatically.

---

# Architecture

Divinity should consist of:

## Orchestrator

Coordinates execution.

Responsible for:

- Scheduling
- Parallelism
- Task routing
- Agent lifecycle

## Task Planner

Breaks large requests into manageable tasks.

## Agent Manager

Manages adapters for:

- CLI agents
- API providers
- Local models
- Shell-based agents

## Workspace Manager

Responsible for:

- Git worktrees
- Temporary branches
- Filesystem isolation

## Evaluation Engine

Responsible for:

- Tests
- Linting
- Type checking
- Diff analysis
- Recommendation scoring

## Memory Layer

Stores:

- Historical outcomes
- Agent performance
- Project conventions
- User preferences

## Terminal UI

Provides:

- Live execution dashboard
- Logs
- Status indicators
- Result comparison
- Review experience

---

# Initial Commands

## Initialize project

Creates:

- `.divinity/`
- Project configuration
- Default settings

## Run task

Runs a coding task through one or more agents.

## Compare agents

Runs the same task through multiple agents and evaluates results.

## Review

Displays generated diffs, validation results, and recommendations.

## Status

Shows:

- Running agents
- Progress
- Logs
- Failures
- Evaluation state

---

# UI Direction

Divinity should feel premium.

Principles:

- Terminal-first
- Fast feedback
- Minimal friction
- High signal-to-noise ratio

Avoid:

- Cluttered layouts
- Excessive logging
- Overwhelming dashboards

The interface should make multi-agent execution feel intuitive and elegant.

---

# Development Principles

## Prioritize usefulness over complexity

Do not over-engineer.

A simple orchestration system that works reliably is better than an overly ambitious broken platform.

---

## Build incrementally

Start narrow.

The first useful version should only focus on:

**Run one task through multiple agents and compare outputs safely.**

Nothing more.

---

# Roadmap

## Phase 1 — MVP

Goals:

- CLI skeleton
- Gemini CLI support
- Custom shell agents
- Git worktrees
- Task execution
- Logging
- Diff generation
- Test and lint execution
- Basic terminal UI

Success criteria:

Divinity can reliably run a task, evaluate it, and present results safely.

---

## Phase 2 — Multi-agent orchestration

Goals:

- Parallel execution
- Evaluation engine
- Reviewer role
- Task planning
- Execution graphs
- Agent coordination

---

## Phase 3 — Local models

Goals:

- Ollama support
- llama.cpp support
- OpenAI-compatible adapters
- Hybrid cloud/local workflows

---

## Phase 4 — Intelligence layer

Goals:

- Persistent memory
- Adaptive agent selection
- Cost optimization
- Automatic fallbacks

Example behavior:

If Claude is unavailable:

1. Fallback to Gemini
2. If cloud fails, fallback to local model
3. Continue execution gracefully

---

# Long-term Vision

Divinity becomes:

**The operating system for coding agents.**

A unified orchestration layer where developers coordinate multiple AI systems safely, transparently, and locally.

The future is not one super-agent.

The future is **agent coordination**.