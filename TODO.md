# nano-code Go Migration TODO

## 移植方針

- 既存の `./nano-code` TypeScript 実装を仕様の正とし、同等機能を Go で段階的に再実装する。
- 既存 README のレイヤ境界を維持する: `domain -> application -> infrastructure`、`interfaces` は実行入口と入出力変換に限定する。
- Go 側は CLI と A2A サーバを同一 module 内の複数 entrypoint として扱う。
- 先にドメイン型、LLM provider 抽象、agent loop、tools の順で薄い縦断動作を作り、その後 provider/tool/A2A の互換性を詰める。
- 各移植単位は既存の `*.test.ts` に対応する Go テストを追加してから完了扱いにする。

## 想定する Go ディレクトリ構成

- [x] `go.mod` を作成し、module path と Go version を決める。
- [x] `cmd/nano-code/main.go` を CLI entrypoint として作成する。
- [x] `cmd/nano-code-a2a/main.go` を A2A server entrypoint として作成する。
- [x] `internal/domain` に message、tool、usage、LLM、A2A の型を置く。
- [x] `internal/application/agent` に agent loop を置く。
- [x] `internal/application/generation` に generation/stream collection を置く。
- [x] `internal/application/a2a` に A2AService 相当を置く。
- [x] `internal/application/ports` に approval や provider などの境界 interface を置く。
- [x] `internal/infrastructure/llm` に OpenAI、Anthropic、Google provider 実装を置く。
- [x] `internal/infrastructure/tools` に read/write/edit/exec/git/github/webFetch/A2A tools を置く。
- [x] `internal/infrastructure/process` に sandbox 実装を置く。
- [x] `internal/infrastructure/approval` に対話 approval 実装を置く。
- [x] `internal/infrastructure/a2a` に A2A client、agent registry、remote-agent tool 生成を置く。
- [x] `internal/infrastructure/prompts` に prompt 読み込み処理と markdown prompt を置く。
- [x] `internal/infrastructure/logger` に stdout/stderr 分離を意識した logger を置く。
- [x] `internal/interfaces/cli` に CLI parsing と runner 呼び出しを置く。
- [x] `internal/interfaces/a2a` に HTTP router、controller、JSON-RPC error mapping を置く。

## Phase 0: 現行仕様の棚卸し

- [x] `nano-code/README.md` の CLI、A2A、env var、remote A2A agent の仕様を Go 版 README の初期仕様として転記する。
- [x] `nano-code/package.json` の scripts を Go 版の `make` task または `go test`/`go run` コマンドへ対応付ける。
- [x] 既存 TypeScript テスト一覧を移植トラッキング表にする。
- [x] CLI option 互換表を作る: `--yolo`、`--verbose`、`--sandbox`、`--streaming`、`--allowed-domains`。
- [x] 環境変数互換表を作る: `LLM_PROVIDER`、`LLM_MODEL`、`LLM_API_KEY`、provider 固有 API key、A2A 系 env。
- [x] 既存の tool schema と tool name を一覧化し、Go 版で名前を変えない方針を確認する。

## Phase 1: Go プロジェクト土台

- [x] `go mod init` を実行する。
- [x] 開発コマンドを README に整理し、`mise.toml` で Go version とローカル環境変数を管理する。
- [x] `.gitignore` を Go binary、coverage、temporary file 向けに追加する。
- [x] Go 標準ツールによる品質チェックを追加する: `gofmt`、`go vet`。
- [x] `go test ./...` が空実装でも通る状態を作る。
- [x] `context.Context` を主要 public/internal 境界の第一引数にする方針を統一する。
- [x] stdout は通常出力、stderr はログ/診断に限定する方針を README に明記する。

## Phase 2: Domain と application core

- [x] `nano-code/src/domain/types.ts` を `internal/domain` の Go 型に移植する。
- [x] `Tool` を interface または struct+func として定義し、JSON Schema parameters を保持できるようにする。
- [x] `Message` は role 別 struct か単一 struct で表現し、provider 変換時に欠落が起きないようにする。
- [x] `LanguageModel` interface を `Generate` と `Stream` に分けるか、現行同様に単一 interface にするか決める。
- [x] `LLMApiError` 相当を Go error として実装し、provider、status、code、raw body を保持する。
- [x] `nano-code/src/application/generation/generateText.ts` を移植する。
- [x] streaming chunk を集約する `CollectStreamResult` を実装する。
- [x] `nano-code/src/application/agent/Agent.ts` の agent loop を移植する。
- [x] tool approval、tool missing、tool error、max steps、context compression の挙動を既存と揃える。
- [x] `Agent` の unit test を追加し、tool call 実行、拒否、max step、streaming 集約を検証する。

## Phase 3: Prompt と設定

- [x] `nano-code/src/config.ts` 相当の default config を Go で定義する。
- [x] `baseInstructions.md` と `issueInstructions.md` を Go 側に移す。
- [x] `loadInstructions` を移植し、workspace の `AGENTS.md` 読み込み仕様を維持する。
- [x] `ISSUE_BODY`/`ISSUE_TEXT` による issue-driven prompt を CLI 側で維持する。
- [x] `allowedDomains` の default と CLI/A2A からの追加処理を整理する。
- [x] prompt loader のテストを移植する。

## Phase 4: LLM providers

- [x] provider factory を移植し、`LLM_PROVIDER` と `LLM_MODEL` 必須チェックを維持する。
- [x] `LLM_API_KEY` を provider 固有 env にフォールバック設定する挙動を維持する。
- [x] OpenAI provider を実装する。
- [x] Anthropic provider を実装する。
- [x] Google provider を実装する。
- [x] 各 provider で messages、tools、tool calls、usage、finish reason の変換をテストする。
- [x] streaming 対応の実装可否を provider ごとに確認し、未対応の場合は明示 error にする。
- [x] provider テストは外部 API に依存しない変換/HTTP mock 中心にする。

## Phase 5: Local tools

- [x] `readFile` を移植し、workspace 外 path を拒否する。
- [x] `writeFile` を移植し、workspace 外 path を拒否する。
- [x] `editFile` を移植し、置換失敗や複数一致などの既存挙動を確認する。
- [x] `execCommand` を移植し、allowlist、危険文字拒否、argument path 検証、timeout、出力 truncate を維持する。
- [x] `git` tools を移植する: branch 作成、commit、push。
- [x] `github` tools を移植する: PR 作成、issue comment 作成。
- [x] `webFetch` を移植し、allowed domain 制限と response size 制限を維持する。
- [x] `createTools` の tool 登録順と tool name を既存と揃える。
- [x] 各 tool の Go テストを既存 `*.test.ts` に対応させる。

## Phase 6: Sandbox と approval

- [x] `nano-code/src/infrastructure/process/Sandbox.ts` の挙動を調査し、Go で同等の Linux sandbox を実装する。
- [x] sandbox 有効時の env、cwd、network deny、exit code/stdout/stderr の扱いを揃える。
- [x] Linux 以外では sandbox 未対応を明示するか、既存と同じ fallback にするか決める。
- [x] `readlineApproval` 相当を Go で実装する。
- [x] yolo mode では approval を常に許可する。
- [x] approval prompt のテストを追加する。

## Phase 7: CLI

- [x] CLI library を決める。単一コマンドのため標準 `flag` を使い、設定が増える場合は Cobra/Viper を検討する。
- [x] `nano-code/src/interfaces/cli/main.ts` の option と positional prompt を移植する。
- [x] prompt 未指定時は usage error として exit code を分ける。
- [x] `--verbose` または `LOG_LEVEL=debug` で debug log を出す。
- [x] `workspaceRoot` は現行同様 `./workspace` を default にする。
- [x] signal handling を `signal.NotifyContext` で実装し、agent/provider/tool に cancellation を伝播する。
- [x] CLI test を追加し、引数 parsing、env issue prompt、allowed domains、error path を検証する。

## Phase 8: A2A server

- [x] `nano-code/src/domain/a2a.ts` の型を Go に移植する。
- [x] `A2AService` を移植し、Agent Card と `message/send` の挙動を維持する。
- [x] HTTP router を選ぶ。標準 `net/http` で足りるか、OpenAPI/docs 生成の都合で router を導入するか決める。
- [x] `GET /.well-known/agent-card.json` を実装する。
- [x] `POST /a2a` JSON-RPC 2.0 endpoint を実装する。
- [x] Bearer auth を `A2A_AUTH_TOKEN` で有効化する。
- [x] `GET /docs` の扱いを決める。Go 版で Swagger UI を継続する場合は OpenAPI 生成方針を決める。
- [x] A2A controller/server tests を移植する。
- [x] `PORT`、`HOST`、`A2A_AGENT_URL`、`A2A_SANDBOX`、`A2A_ALLOWED_DOMAINS` を維持する。

## Phase 9: Remote A2A agent integration

- [x] `agents.json` を Go 側に移す。
- [x] `loadA2AAgentSources` を移植する。
- [x] `A2AClient` を移植し、Agent Card fetch と JSON-RPC invocation を実装する。
- [x] `A2AAgentRegistry.discover` を移植し、起動時に失敗した agent を skip する挙動を維持する。
- [x] `createA2ATools` を移植し、remote agent を tool として公開する。
- [x] A2A client/registry/tool 生成のテストを移植する。

## Phase 10: テスト、互換性、品質

- [x] 既存 TypeScript テストと同等の Go test coverage を揃える。
- [x] `go test ./...` を必須検証にする。
- [x] `go test -race ./...` を concurrency を含む package で実行する。
- [x] Go 標準チェックを通す: `gofmt`、`go vet`。
- [x] provider 変換の golden test を追加し、tool schema と message 変換の regressions を防ぐ。
- [x] CLI smoke test を追加する: yolo + fake model + fake tool。
- [x] A2A smoke test を追加する: agent card fetch と message/send。
- [x] network/API key が必要な integration test は build tag または env guard で通常 test から分離する。

## Phase 11: 移行完了作業

- [x] Go 版 README を更新し、Node/Bun 前提の手順を Go 手順に置き換える。
- [x] `bun run agent` 相当の `go run ./cmd/nano-code` または binary 実行例を記載する。
- [x] `bun run a2a` 相当の `go run ./cmd/nano-code-a2a` または binary 実行例を記載する。
- [x] Node 版を残す場合は `nano-code-ts` などに rename するか、deprecated と明記する。
- [x] Go 版が main 実装になった後、不要な TypeScript dependency と Bun 設定を削除する。
- [x] release build 方針を決める。必要なら GoReleaser を追加する。
- [x] CI を Go 版に切り替える: `gofmt`、`go vet`、`go test`、`go test -race`。

## Phase 12: SDK 化と依存脆弱性管理

- [x] 公式 Go SDK の有無を確認する: OpenAI `github.com/openai/openai-go/v3`、Anthropic `github.com/anthropics/anthropic-sdk-go`、Gemini `google.golang.org/genai`。
- [x] CI に `govulncheck ./...` を追加し、既知脆弱性を pull request と main push で検出する。
- [x] OpenAI provider を公式 SDK 実装へ置き換え、`domain.LanguageModel` 境界と既存の message/tool/usage 変換を維持する。
- [x] Anthropic provider を公式 SDK 実装へ置き換え、Messages API の tool use と streaming event 変換を維持する。
- [x] Google provider を公式 SDK 実装へ置き換え、Gemini API の function calling と streaming 変換を維持する。
- [x] SDK 追加後に `go mod verify`、`govulncheck ./...`、`go test ./...`、`go test -race ./...` を必須検証として実行する。

## Review follow-up: 互換性バグ修正

- [x] `internal/infrastructure/tools/path.go` の writable path 解決で、`./workspace` が存在しない fresh checkout でも `writeFile` が `workspace/nested/file` を作成できるようにする。workspace root の symlink 解決前に必要な directory を作成し、TypeScript 実装と同じ挙動に揃える。
- [x] `internal/infrastructure/a2a/client.go` の A2A task result 抽出で、completed response に `status.message` と `artifacts` が両方ある場合は artifact text を優先する。status message は進捗や要約の可能性があるため、TypeScript 実装と同じ優先順位にする。
- [x] `internal/infrastructure/llm/providers/openai.go` の streaming tool call 集約で、OpenAI の streamed `index` / model order を保持する。buffer key の文字列 sort による順序入れ替わりを避け、複数 tool call の実行順がモデル出力順になるようにする。
- [x] `internal/infrastructure/llm/providers/anthropic.go` の streaming `tool_use` 集約で、map iteration に依存せず content block order を保持する。複数 tool call がある場合も side effect の順序が nondeterministic にならないようにする。
- [x] `internal/infrastructure/llm/providers/google.go` の streaming function call 集約で、Gemini の `Content.Parts` sequence を保持する。map iteration による順序喪失を避け、tool call slice をモデル出力順で返す。
- [x] 上記修正ごとに Go test を追加または更新し、fresh workspace write、A2A artifact 優先、各 provider の複数 streaming tool call 順序を回帰テストする。

## Review follow-up: 責務分離と Go 実装改善

- [x] CLI streaming 時に delta 出力と最終 `result.Text` 出力が重複しないようにする。streaming mode では `Agent` が逐次出力した内容を CLI が再出力しない設計にする。
- [x] A2A interface が CLI interface に依存しないよう、共通の agent wiring を interface 層の外へ切り出す。CLI と A2A はそれぞれ同じ runtime runner へ依存する形にする。
- [x] 上記修正の Go test を追加または更新し、streaming stdout が一度だけ出ることと A2A/CLI の wiring が維持されることを検証する。

## Review follow-up: provider HTTP client 整理

- [x] `internal/infrastructure/llm/providers` の production API から独自 `HTTPDoer` を削除し、SDK に渡す HTTP client は標準 `*http.Client` に統一する。
- [x] SDK 化前の未使用 HTTP helper (`postJSON` / `streamJSON` など) を削除し、provider package には現在使う helper だけを残す。
- [x] provider tests の HTTP fake は test helper の `RoundTripper` として実装し、production package にテスト都合の interface を置かない形にする。

## Review follow-up: provider helper の責務整理

- [x] `internal/infrastructure/llm/providers/http.go` を削除し、`Config` は `config.go`、stream channel helper は `stream.go` に分割する。
- [x] OpenAI と Google で共有していた `toolSchema` を削除し、各 provider の schema 変換を provider 実装内に移す。
- [x] 上記リファクタ後に provider tests と全体 Go test / vet / race を通し、挙動差分がないことを確認する。

## Supply chain follow-up: Takumi Guard メール認証

- [x] 無料のメール認証トークン (`tg_anon_...`) を使うローカル開発手順を README に追記する。`.netrc` への保存、`chmod 600 ~/.netrc`、`GOPROXY=https://golang.flatt.tech` の設定、トークンをリポジトリにコミットしない注意を含める。
- [x] メール認証トークン利用時の動作確認手順を追加する。`github.com/flatt-security/hola-takumi-go@v0.1.0` が `403 Forbidden` で拒否されることと、通常の `go mod download` / `go mod verify` が通ることを確認する。

## 完了条件

- [x] CLI で既存と同じ prompt 実行、tool call、approval、streaming が動く。
- [x] A2A server が Agent Card、auth、`message/send` を既存互換で提供する。
- [x] OpenAI、Anthropic、Google の provider factory が既存 env var で動く。
- [x] local tools と remote A2A tools の tool name/schema が既存互換である。
- [x] `go test ./...` と Go 標準チェックが通る。
- [x] CI で `govulncheck ./...` による既知脆弱性検査を実行する。
- [x] README の利用手順が Go 版だけで完結している。
