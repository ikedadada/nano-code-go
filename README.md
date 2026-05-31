# nano-code-go

Go implementation of `nano-code`.

The previous TypeScript implementation is retained under `./nano-code` as a
compatibility reference while the migration is finalized. The Go implementation
keeps the same layer boundaries:

```text
interfaces -> application -> domain
```

Infrastructure implementations are wired at the interface/composition boundary.
Application packages should depend on domain types and application ports, not
directly on infrastructure adapters.

## CLI

Run the CLI with:

```sh
go run ./cmd/nano-code [options] "Your prompt here"
```

Or build a binary:

```sh
go build -o bin/nano-code ./cmd/nano-code
bin/nano-code [options] "Your prompt here"
```

Options:

- `-y, --yolo`: approve all tool calls.
- `-v, --verbose`: show debug logs.
- `-s, --sandbox`: run commands in sandbox.
- `-S, --streaming`: stream model output.
- `-d, --allowed-domains <domains>`: comma-separated domains allowed for web
  fetch.

The default workspace root is `./workspace`. Normal command output goes to
stdout, and logs/diagnostics go to stderr.

## A2A Server

Run the A2A server with:

```sh
go run ./cmd/nano-code-a2a
```

Or build a binary:

```sh
go build -o bin/nano-code-a2a ./cmd/nano-code-a2a
bin/nano-code-a2a
```

The server exposes:

- `GET /.well-known/agent-card.json`: A2A Agent Card discovery.
- `POST /a2a`: JSON-RPC 2.0 endpoint supporting `message/send`.
- `GET /docs`: static API documentation endpoint.

Environment variables:

- `PORT`: HTTP port, default `3000`.
- `HOST`: host name used to build the default Agent Card URL, default
  `localhost`.
- `A2A_AGENT_URL`: explicit Agent Card service URL, default
  `http://{HOST}:{PORT}/a2a`.
- `A2A_AUTH_TOKEN`: optional Bearer token required by `POST /a2a`.
- `A2A_SANDBOX`: set to `true` to run tool commands through the sandbox.
- `A2A_ALLOWED_DOMAINS`: comma-separated domains allowed for web fetch.

A2A requests run non-interactively after authentication, so tool approval is
automatically granted for authenticated requests.

## LLM Environment

- `LLM_PROVIDER`: required. Supported values are `openai`, `anthropic`, and
  `google`.
- `LLM_MODEL`: required model identifier.
- `LLM_API_KEY`: generic fallback API key.
- `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GOOGLE_API_KEY`: provider-specific
  API keys. If a provider-specific key is not set, `LLM_API_KEY` is used.

The provider implementations use the official Go SDKs for OpenAI, Anthropic,
and Gemini while preserving the internal `domain.LanguageModel` interface.

## Migration Tracking

`TODO.md` is the migration checklist. Detailed compatibility tables are in
[docs/migration.md](docs/migration.md).

## Legacy TypeScript Implementation

The TypeScript implementation under `./nano-code` is deprecated and kept only
as a compatibility reference during the Go migration. New runtime changes should
target the Go commands in `./cmd`.

## Release Build

The current release build policy is to produce the two CLI binaries directly
with `go build`:

```sh
make build
```

This writes `bin/nano-code` and `bin/nano-code-a2a`. GoReleaser is not required
yet; it can be added later if tagged multi-platform archives or generated
release notes are needed.

## Development

Useful commands:

```sh
make fmt
make test
make race
make lint
make vuln
make build
make run
make run-a2a
```

`make test` runs `go test ./...`. `make race` runs `go test -race ./...`.
`make lint` uses only Go standard tooling: it checks `gofmt` output and runs
`go vet ./...`. `make vuln` runs the pinned `govulncheck` tool through
`go tool govulncheck ./...`. `make build` writes ignored binaries under `bin/`.

Provider integration tests are excluded from the default test suite because
they require network access and API keys. Run them explicitly with:

```sh
go test -tags=integration ./internal/infrastructure/llm/providers
```

Each provider test skips unless its provider API key is set. Optional model
overrides are `OPENAI_INTEGRATION_MODEL`, `ANTHROPIC_INTEGRATION_MODEL`, and
`GOOGLE_INTEGRATION_MODEL`.
