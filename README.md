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

## Supply Chain Guard

This repository routes public Go module downloads through Takumi Guard.
Repository configuration stays token-free: `mise.toml` sets
`GOPROXY=https://golang.flatt.tech`, and CI uses anonymous Takumi Guard mode.

CI runs with the Go module cache disabled, so `go mod download` checks current
dependencies through Takumi Guard instead of relying on `actions/setup-go`
cache hits.

Local development is configured through `mise.toml`:

```toml
[env]
GOCACHE = "/tmp/go-build"
GOPROXY = "https://golang.flatt.tech"
```

With `mise activate` enabled in your shell, direct Go commands such as
`go test ./...`, `go mod download`, and `go run ./cmd/nano-code` inherit this
setting automatically when run from the repository.

Verify the active local environment with:

```sh
mise env | grep -E '^(GOCACHE|GOPROXY)='
go env GOCACHE
go env GOPROXY
```

The expected values are `GOCACHE=/tmp/go-build` and
`GOPROXY=https://golang.flatt.tech`. Do not append `,direct` or `|direct` to
`GOPROXY`; direct fallback can bypass Takumi Guard when a module is not
available through the proxy or when a blocked module returns an error.

Use `mise` for local development so these values stay centralized in
`mise.toml`.

Developers may opt into Takumi Guard email-verified access for personal
download tracking and notifications. Follow the upstream
[Go Modules quickstart](https://shisho.dev/docs/t/guard/quickstart/golang/) to
obtain a `tg_anon_...` token, store it in `~/.netrc`, and set
`chmod 600 ~/.netrc`. Keep tokens outside this repository; do not commit them
to `go.mod`, `go.sum`, `mise.toml`, scripts, or CI logs.

After local Takumi Guard setup changes, verify this repository still resolves
its dependencies through the configured proxy:

```sh
go mod download
go mod verify
```

For a full proxy smoke test, use the upstream quickstart's
`github.com/flatt-security/hola-takumi-go@v0.1.0` check. It should be rejected
with `403 Forbidden` when Takumi Guard is active.

## Legacy TypeScript Implementation

The TypeScript implementation under `./nano-code` is deprecated and kept only
as a compatibility reference during the Go migration. New runtime changes should
target the Go commands in `./cmd`.

## Release Build

The current release build policy is to produce the two CLI binaries directly
with `go build`:

```sh
go build -o bin/nano-code ./cmd/nano-code
go build -o bin/nano-code-a2a ./cmd/nano-code-a2a
```

This writes `bin/nano-code` and `bin/nano-code-a2a`. GoReleaser is not required
yet; it can be added later if tagged multi-platform archives or generated
release notes are needed.

## Development

Useful commands:

```sh
gofmt -w ./cmd ./internal
go test ./...
go test -race ./...
go vet ./...
go tool govulncheck ./...
go build -o bin/nano-code ./cmd/nano-code
go build -o bin/nano-code-a2a ./cmd/nano-code-a2a
go run ./cmd/nano-code
go run ./cmd/nano-code-a2a
```

Formatting uses `gofmt`. Linting uses only Go standard tooling: check
`gofmt -l ./cmd ./internal` output and run `go vet ./...`. Vulnerability
checking runs the pinned `govulncheck` tool through
`go tool govulncheck ./...`. Build commands write ignored binaries under
`bin/`.

Provider integration tests are excluded from the default test suite because
they require network access and API keys. Run them explicitly with:

```sh
go test -tags=integration ./internal/infrastructure/llm/providers
```

Each provider test skips unless its provider API key is set. Optional model
overrides are `OPENAI_INTEGRATION_MODEL`, `ANTHROPIC_INTEGRATION_MODEL`, and
`GOOGLE_INTEGRATION_MODEL`.
