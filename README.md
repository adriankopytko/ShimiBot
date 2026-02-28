# ShimiBot

ShimiBot is a Go-based LLM coding assistant that uses OpenAI-compatible tool calling.
It can read and write files, run shell commands, list directories, fetch webpages, and perform web searches via your configured endpoint.

## Architecture overview

Current runtime structure:

- `app/main.go`: composition root (wires dependencies and starts runtime)
- `internal/cli`: CLI flag parsing and interactive shell loop
- `internal/agent`: turn orchestration, tool-call loop, and turn/tool budgets
- `internal/llm`: provider-agnostic domain model + OpenAI adapter
- `internal/session`: session store interface and JSON file implementation
- `internal/tools`: tool implementations + registry + ToolContext/envelope boundary
- `internal/appcore`: bootstrap helpers (logger, env loading, provider config, correlation IDs)

Tool runtime boundary:

- Every tool call receives a `ToolContext` (`cwd`, `allowed_root`, `timeout`, `context`, `correlation_id`, `logger`)
- Every tool response is normalized to envelope JSON:
	- `ok`: boolean
	- `data`: successful payload
	- `error`: `{ "message": string }`
	- `meta`: execution metadata (e.g. tool name, correlation id, cwd)

Hardening features:

- File tools enforce allowed-root path guardrails
- Bash tool enforces command policy checks for blocked command patterns
- Network tools block localhost and private/link-local/multicast/unspecified IP egress by default
- Agent and tools propagate cancellation/timeouts through contexts
- Turn/tool logs include correlation IDs for traceability

Outbound network policy:

- `FetchWebPage` and `WebSearchOllama` only allow outbound targets that resolve to non-local, non-private addresses by default.
- To explicitly allow private/local egress in controlled environments, set `SHIMIBOT_ALLOW_PRIVATE_EGRESS=true`.
- Keep `SHIMIBOT_ALLOW_PRIVATE_EGRESS` unset in normal development and production use.

Log sinks:

- Default sink is `stderr` (text format)
- Optional sink `stdout` (text format)
- Optional sink `json-file` (JSON Lines with `schema_version: "v1"`, event name, and structured fields)

Configurable Bash policy (optional):

- `SHIMIBOT_BASH_DENYLIST`: deny regex patterns for Bash commands
- `SHIMIBOT_BASH_ALLOWLIST`: allow regex patterns for Bash commands (when set, commands must match at least one pattern)

Pattern list format:

- Split patterns with `,`, `;`, or newline
- Example:

```sh
export SHIMIBOT_BASH_DENYLIST='(?i)curl\s+.*\|\s*sh, (?i)wget\s+.*\|\s*bash'
export SHIMIBOT_BASH_ALLOWLIST='(?i)^ls\b; (?i)^cat\b; (?i)^echo\b'
```

## Run locally

1. Ensure you have Go 1.25 installed.
2. Configure environment variables (or create a `.env` file from `.env.example`).
3. Run:

```sh
./run_local.sh -p "Your prompt here"
```

## Required environment variables

```sh
export OPENROUTER_API_KEY="<openrouter-key>"
export OPENROUTER_BASE_URL="https://openrouter.ai/api/v1"
export AI_MODEL="anthropic/claude-haiku-4.5"
```

## Optional web-search tool variables

```sh
export OLLAMA_WEB_SEARCH_URL="https://<your-ollama-search-endpoint>"
export OLLAMA_WEB_SEARCH_API_KEY="<your-key>"
```

## Optional runtime limit variables

```sh
export SHIMIBOT_TURN_TIMEOUT="90s"
export SHIMIBOT_TOOL_TIMEOUT="30s"
export SHIMIBOT_MAX_TURNS="0"
export SHIMIBOT_MAX_TOOL_CALLS="0"
```

## Optional logging sink variables

```sh
export SHIMIBOT_LOG_SINK="stderr"        # stderr | stdout | json-file
export SHIMIBOT_LOG_FILE="/tmp/shimibot.jsonl"  # required when SHIMIBOT_LOG_SINK=json-file
```

Runtime limit flags (override env defaults):

```sh
./run_local.sh -turn-timeout=2m -tool-timeout=45s -max-turns=8 -max-tool-calls=12 -p "Your prompt"
```

Logging sink flags (override env defaults):

```sh
./run_local.sh -log-enabled -log-level=debug -log-sink=json-file -log-file=/tmp/shimibot.jsonl -p "Your prompt"
```

Notes:

- `-max-turns=0` means no turn limit.
- `-max-tool-calls=0` means no tool call limit.
