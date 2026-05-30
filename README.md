# nano-code-go

Go migration workspace for `nano-code`.

The TypeScript implementation in `./nano-code` is the source of truth while
this repository is migrated. The Go implementation keeps the same layer
boundaries:

```text
interfaces -> application -> domain
```

Infrastructure implementations are wired at the interface/composition boundary.
Application packages should depend on domain types and application ports, not
directly on infrastructure adapters.

## CLI

Target command:

```sh
go run ./cmd/nano-code [options] "Your prompt here"
```

Compatibility target from the TypeScript CLI:

- `-y, --yolo`: approve all tool calls.
- `-v, --verbose`: show debug logs.
- `-s, --sandbox`: run commands in sandbox.
- `-S, --streaming`: stream model output.
- `-d, --allowed-domains <domains>`: comma-separated domains allowed for web
  fetch.

The default workspace root remains `./workspace`. Normal command output should
go to stdout, and logs/diagnostics should go to stderr.

## A2A Server

Target command:

```sh
go run ./cmd/nano-code-a2a
```

The Go server must keep the TypeScript A2A surface:

- `GET /.well-known/agent-card.json`: A2A Agent Card discovery.
- `POST /a2a`: JSON-RPC 2.0 endpoint supporting `message/send`.
- `GET /docs`: API documentation endpoint, if Swagger UI/OpenAPI generation is
  retained.

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

## Migration Tracking

`TODO.md` is the authoritative migration checklist. Detailed compatibility
tables are in [docs/migration.md](docs/migration.md).

## Development

Useful commands:

```sh
make fmt
make test
make lint
make run
make run-a2a
```

`make test` runs `go test ./...`. `make lint` expects `golangci-lint` to be
installed.
