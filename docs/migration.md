# nano-code Go Migration Notes

These notes capture the TypeScript behavior that the Go implementation must
preserve.

## Script Mapping

| TypeScript script | TypeScript command | Go target |
| --- | --- | --- |
| `agent` | `bun run src/bin/cli.ts` | `go run ./cmd/nano-code` |
| `a2a` | `bun run src/bin/a2a.ts` | `go run ./cmd/nano-code-a2a` |
| `check` | `biome check .` | `golangci-lint run` |
| `typecheck` | `tsc --noEmit` | `go test ./...` compile step |
| `test` | `bun test` | `go test ./...` |
| `ci` | `bun run check && bun run typecheck && bun run test` | `make lint test` |

## CLI Option Compatibility

| Option | TypeScript behavior | Go target |
| --- | --- | --- |
| `-y`, `--yolo` | approval policy always returns true | keep exact flag and behavior |
| `-v`, `--verbose` | sets debug log level | keep exact flag; also honor `LOG_LEVEL=debug` |
| `-s`, `--sandbox` | enables sandboxed command execution on Linux | keep exact flag and Linux sandbox behavior |
| `-S`, `--streaming` | uses model streaming and writes deltas to output stream | keep exact flag and streaming aggregation |
| `-d`, `--allowed-domains` | comma-separated web fetch allowlist additions | keep exact flag and parsing |
| positional prompt | joined with spaces | preserve |
| no prompt | usage error unless issue env is present | preserve with non-zero exit |

`ISSUE_BODY` takes precedence over `ISSUE_TEXT` when issue-driven mode is
active.

## Environment Compatibility

| Variable | Required | Behavior |
| --- | --- | --- |
| `LLM_PROVIDER` | yes | selects `openai`, `anthropic`, or `google` |
| `LLM_MODEL` | yes | model identifier passed to provider |
| `LLM_API_KEY` | no | fallback for provider-specific API key |
| `OPENAI_API_KEY` | provider-specific | used by OpenAI provider |
| `ANTHROPIC_API_KEY` | provider-specific | used by Anthropic provider |
| `GOOGLE_API_KEY` | provider-specific | used by Google provider |
| `LOG_LEVEL` | no | `debug` enables debug logs |
| `ISSUE_BODY` | no | issue-driven prompt body |
| `ISSUE_TEXT` | no | fallback issue-driven prompt body |
| `ISSUE_NUMBER` | no | referenced by issue instructions |
| `PORT` | no | A2A server port, default `3000` |
| `HOST` | no | A2A host for default Agent Card URL, default `localhost` |
| `A2A_AGENT_URL` | no | explicit A2A endpoint URL |
| `A2A_AUTH_TOKEN` | no | enables Bearer auth for `POST /a2a` |
| `A2A_SANDBOX` | no | `true` enables sandbox for A2A tool commands |
| `A2A_ALLOWED_DOMAINS` | no | comma-separated A2A web fetch allowlist additions |

## Tool Compatibility

Tool names and registration order are part of compatibility:

1. `readFile`
2. `writeFile`
3. `editFile`
4. `execCommand`
5. `createBranch`
6. `commit`
7. `pushBranch`
8. `createPullRequest`
9. `createIssueComment`
10. `webFetch`
11. remote A2A tools generated as `a2a_{agentID}_{skillID}`

All tool parameters are JSON Schema object definitions. The Go representation
must preserve schema names, required fields, and provider conversion behavior.

Important behavior:

- `readFile`, `writeFile`, and `editFile` restrict access to `./workspace`
  after path resolution; symlink escapes are rejected.
- `readFile` rejects non-files and files larger than 100 KB.
- `editFile` requires exactly one `oldText` match.
- `execCommand` allows only `bun`, `ls`, `git`, `gh`, and `curl`; shell
  metacharacters `;`, `&`, `` ` ``, and `$` are rejected for string commands.
- `execCommand` validates path-like arguments against `./workspace`, uses a
  30 second timeout, and truncates stdout/stderr to 2024 characters.
- GitHub and git tools shell out through `execCommand`.
- `webFetch` restricts target domains to the configured allowlist, rejects
  redirects, and the Go port enforces a response size limit to satisfy the
  migration TODO. The current TypeScript implementation does not enforce that
  limit.

## TypeScript Test Inventory

| TypeScript test | Go migration target |
| --- | --- |
| `src/domain/types.test.ts` | `internal/domain` tests |
| `src/application/generation/generateText.test.ts` | `internal/application/generation` tests |
| `src/application/a2a/A2AService.test.ts` | `internal/application/a2a` tests |
| `src/application/agent/Agent.test.ts` | `internal/application/agent` tests |
| `src/interfaces/cli/main.test.ts` | `internal/interfaces/cli` tests |
| `src/interfaces/a2a/controllers/docsController.test.ts` | A2A docs/controller tests |
| `src/interfaces/a2a/controllers/messageSendController.test.ts` | A2A JSON-RPC controller tests |
| `src/interfaces/a2a/controllers/agentCardController.test.ts` | Agent Card controller tests |
| `src/interfaces/a2a/server.test.ts` | A2A server composition tests |
| `src/interfaces/agentRunner.test.ts` | interface runner tests |
| `src/helper.test.ts` | helper tests |
| `src/infrastructure/approval/readlineApproval.test.ts` | approval tests |
| `src/infrastructure/process/Sandbox.test.ts` | sandbox tests |
| `src/infrastructure/prompts.test.ts` | prompt loader tests |
| `src/infrastructure/logger/logger.test.ts` | logger tests |
| `src/infrastructure/a2a/createA2ATools.test.ts` | remote A2A tool tests |
| `src/infrastructure/a2a/A2AClient.test.ts` | A2A client tests |
| `src/infrastructure/a2a/loadA2AAgentSources.test.ts` | agent catalog loader tests |
| `src/infrastructure/a2a/A2AAgentRegistry.test.ts` | A2A registry tests |
| `src/infrastructure/tools/webFetch.test.ts` | web fetch tests |
| `src/infrastructure/tools/execCommand.test.ts` | exec command tests |
| `src/infrastructure/tools/writeFile.test.ts` | write file tests |
| `src/infrastructure/tools/github.test.ts` | GitHub tool tests |
| `src/infrastructure/tools/git.test.ts` | git tool tests |
| `src/infrastructure/tools/index.test.ts` | tool registration tests |
| `src/infrastructure/tools/readFile.test.ts` | read file tests |
| `src/infrastructure/tools/editFile.test.ts` | edit file tests |
| `src/infrastructure/llm/providers/anthropic.test.ts` | Anthropic provider tests |
| `src/infrastructure/llm/providers/openai.test.ts` | OpenAI provider tests |
| `src/infrastructure/llm/providers/modelFactory.test.ts` | provider factory tests |
| `src/infrastructure/llm/providers/google.test.ts` | Google provider tests |
