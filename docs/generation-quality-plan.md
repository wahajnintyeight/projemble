# Projemble: generation quality and stack options plan

Date: 22 October 2026. Status: proposed. Scope: the deterministic starter (`internal/generator`, `internal/catalog`), the wizard (`internal/tui`), and the AI generation step (`internal/tui/save.go`, `internal/agent`).

Goal: Projemble offers a real choice at every important step (topology, framework, router, data access, migrations, messaging, auth, observability, AI stack, and so on). Every combination it offers produces code at one quality bar, and the AI step extends that code without breaking the chosen stack.

Library facts below (versions, licenses, minimum Go, maintenance) were checked on 2026-10-22 against proxy.golang.org, pkg.go.dev, and the upstream repositories. Re-verify each pin when its step is implemented (see §3.5).

---

## 1. Current findings

| # | Finding | Location |
| --- | --- | --- |
| F1 | HTTP server uses `http.ListenAndServe`: no timeouts, no graceful shutdown, fixed `:8080`, plain `log` | `generator.go` `mainGo` |
| F2 | Architecture templates are greeters; Layered calls package functions with no interfaces; no vertical slice | `generator.go` `layeredFiles`/`cleanFiles`/`dddFiles` |
| F3 | Database `Open` is never wired, sets no pool limits, pings without timeout, has no migrations; Mongo ignores `ctx` and never pings | `features.go` `databaseAdapter` |
| F4 | Generated Go is not gofmt-formatted; nothing checks it | all string templates |
| F5 | Only one choice point exists beyond architecture (`service-framework` = go-micro); most capabilities are `Supported: false` | `catalog/registry.go`, `catalog/capabilities.go` |
| F6 | Pattern scaffolds are interface-only, unwired, unbounded | `features.go` `patternScaffold` |
| F7 | Only the library template ships a test | renderers |
| F8 | Projemble does not verify the agent's result; broken output can be saved as `ready` | `tui/save.go` |
| F9 | AI task prompt has no plan step, acceptance criteria, or stack constraints | `tui/save.go` `generationTask` |
| F10 | Templates are Go string literals built with `fmt.Sprintf`: hard to read, review, format, or compose | `internal/generator/*.go` |
| F11 | Generated `go.mod` says `go 1.25.0`; current Go is 1.27.3, and many current libraries need ≥ 1.26 (validator, goose, go-redis, modernc sqlite, x/oauth2, x/time, go-micro v6) | `generator.go` `goMod` |
| F12 | go-micro is pinned at `v6.10.0`; v6.12 made default plugins an explicit import (`cmd/defaults`), and v6 now focuses on agents/MCP | `generator.go` `goMod` |

## 2. Quality bar for every generated combination

1. `gofmt -l` reports nothing. `go vet ./...` and `go build ./...` pass. When tests are included, `go test ./...` passes too (`-race` where cgo is available).
2. **When the user includes tests (§4.19):** every package with logic has tests, and HTTP handlers are tested with `httptest` (or the framework's equivalent). When tests are excluded, the project ships no `_test.go` files. Projemble still checks the project's behaviour with its own tests (§3.7).
3. Constructors receive their dependencies. There is no package-level mutable state, and `main` only wires dependencies.
4. Every external call gets a context with a deadline. Servers and workers set timeouts and shut down gracefully.
5. Input is validated at the boundary, errors are wrapped with `%w`, and domain errors map to transport status codes in one place.
6. List endpoints, chat history, retries, fan-out, pools, request bodies and queues all have explicit limits (the efficient-code skill).
7. Config comes from environment variables or the chosen config library, with documented defaults. Secrets are never hardcoded or logged.
8. Logging goes through the chosen logger (§4.13), and request-scoped lines carry the request ID. Clients never see internal error text, and secrets never reach logs.
9. Files stay at or below 1,000 lines. Dependency direction follows the selected architecture.
10. The README accurately describes how to run and test the project, every config variable, the layout, the **chosen options**, and any extra tools or licenses they bring.
11. Only the libraries the user chose appear in `go.mod`. There are no unused dependencies, and every version is an exact pin.

---

## 3. The option model (how choices work)

### 3.1 Concepts

- **Decision step (axis):** one question in the wizard, e.g. "HTTP router". It applies only when its condition holds (e.g. "Messaging" applies to http-api and worker, not to library).
- **Option:** one answer, e.g. `chi`. Each option declares:
  - the modules it adds, with pinned versions and licenses
  - its minimum Go version
  - what it needs (cgo, Docker, a codegen CLI, an external service)
  - its rules: `requires`, `conflicts`, and `replaces` (which other steps it takes over)
  - its status
  - the template parts it contributes
  - the README section it contributes
  - the one-line agent skill it contributes
- **Status:** `recommended` (the default, marked in the UI), `supported`, `experimental` (pre-1.0 or brand-new major; shown with a badge), `planned` (visible but cannot be selected), `legacy` (never offered for new projects; documented only).
- **Preset:** a named bundle of choices that fills every step at once and can then be edited:

  | Preset | What it fills in |
  | --- | --- |
  | Minimal (stdlib) | stdlib everything; `fmt` logging; embedded migration runner; no extra dependencies beyond the database driver |
  | Recommended | the `recommended` option on every step (slog logging) |
  | Popular frameworks | gin + GORM + viper + zap |
  | Contract-first | chi + oapi-codegen + validator + pgx/sqlc + goose + slog + OTel |

  Every preset leaves **tests off** (the default, §4.19). If the user turns tests on, the preset's test style is used: stdlib + go-cmp for Minimal, testify for the others.
  | Custom | every step asked one by one |

### 3.2 Catalog data model (`internal/catalog`)

```go
type Step struct {
    ID, Name, Question string
    AppliesTo          func(Selection) bool // workload/topology/capability conditions
    Options            []Option
}

type Option struct {
    ID, Name, Summary string
    Status            OptionStatus  // Recommended, Supported, Experimental, Planned, Legacy
    Modules           []ModuleRef   // keys into the module registry
    Tools             []ModuleRef   // go.mod `tool` directives (sqlc, oapi-codegen, goose CLI...)
    Needs             []Need        // CGO, Docker, ExternalService("PostgreSQL"), CodegenStep
    Requires          []Constraint  // e.g. "database=postgresql"
    Conflicts         []Constraint
    Replaces          []string      // step IDs this option takes over (huma replaces validation + api-contract)
    Notes             string        // license, deprecation, edition warnings shown in the UI
}

type ModuleRef struct{ Path string }          // resolved through the module registry
type ModuleInfo struct {
    Path, Version, License, MinGo string
    VerifiedAt                    string        // date the pin was checked
}
type Selection map[string]string             // step ID -> option ID
```

- One module registry (`internal/catalog/modules.go`) holds every pin, so updating a version is a one-line change.
- `catalog.Resolve(Selection) (Plan, error)` applies presets, defaults, `requires`/`conflicts`/`replaces`, and returns either a resolved plan or every problem at once in a human-readable form. The wizard and the CLI both use it.
- The existing `Settings` (`stack`, `service-framework`) and the `Capabilities` list become steps. The existing `Capabilities` field keeps loading through a migration function, so old YAML profiles still work.

### 3.3 Project profile (`internal/projectstore`)

- Add `Choices map[string]string \`yaml:"choices,omitempty"\`` and `Preset string`.
- `NormalizeProject` fills missing choices with the option that reproduces today's behaviour (stdlib, go-micro for microservices, and so on), so existing profiles generate the same structure until the user changes them.
- Store the resolved plan's Go version and module pins in the profile, so later AI turns and regenerations use the same versions.

### 3.4 How options plug into generated code (seams)

Each step owns exactly one boundary in the generated code. Swapping an option changes only the files behind that boundary; domain and application code never import an optional library.

| Step | Seam in generated code | What changes when the option changes |
| --- | --- | --- |
| Router | `internal/transport/http/router.go` (or the architecture's HTTP adapter package) | route registration, middleware adapters, handler signatures |
| API contract | `api/` + generated server interfaces | handlers implement generated interfaces instead of hand routes |
| Validation | request DTO `Validate()` or tags + one `validate` helper | DTO tags or methods |
| Data access | repository adapter package | queries, generated code, scanning |
| Migrations | `migrations/` + `internal/platform/migrate` | file format and runner |
| Config | `internal/config` | loader implementation (the struct shape stays the same) |
| Logging | `internal/platform/logging`: a small project-owned `Logger` interface | the implementation behind it (fmt, log, slog, zap, zerolog, logrus) |
| Tests | `_test.go` files, test helpers, `internal/testutil` | whether tests exist, assertion style, test doubles, integration harness |
| DI | `cmd/*/main.go` / `internal/app` | manual constructor wiring vs fx modules vs do container |
| Messaging/jobs | `internal/platform/queue` + a `Publisher`/`Consumer` port | adapter implementation |
| Auth | `internal/platform/auth` middleware + `Authenticator`/`Authorizer` ports | verifier implementation |
| Observability | `internal/platform/telemetry` | providers and exporters |
| LLM/vector/channel | `internal/<pattern>` ports + `internal/adapters/<provider>` | adapter implementation |

The logging seam is a project-owned interface, so application code is identical for every logger choice:

```go
type Logger interface {
    Debug(msg string, keyvals ...any)
    Info(msg string, keyvals ...any)
    Warn(msg string, keyvals ...any)
    Error(msg string, keyvals ...any)
    With(keyvals ...any) Logger // request ID, component, etc.
}
```

Each logging option generates one implementation of this interface: about 40 lines for `fmt`/`log`, and a thin wrapper for slog, zap's `SugaredLogger`, zerolog and logrus. Middleware and services receive a `logging.Logger`, and only `logging.New(cfg)` knows which backend is used. Projects that choose slog can also get the underlying `*slog.Logger` for libraries that accept one.

### 3.5 Version and Go-toolchain policy

- Each option declares the minimum Go version its modules require. Generated `go.mod` sets `go` = the maximum of a **baseline of 1.26.0** and every chosen module's minimum. Go 1.26 and 1.27 are the supported releases today; current is 1.27.3.
- Projemble sets only the `go` line (the minimum, and at least every dependency's). It adds a `toolchain` line only when an option needs a specific patch release, so teams on mixed Go versions are not forced to download another toolchain.
- Before generating, Projemble runs `go env GOVERSION`. If the local toolchain is older than the plan needs, the wizard says which option raised the floor and offers either `toolchain` auto-download (`GOTOOLCHAIN=auto`) or an older fallback pin, where the registry has one (e.g. Connect v1 instead of v2, jwx v3 instead of v4).
- Codegen CLIs (sqlc, oapi-codegen, ogen, goose, mockery, buf plugins via `go tool`) go into `go.mod` `tool` directives so `go tool sqlc generate` works without a global install. Exceptions that need external binaries (buf, atlas, goctl, kratos, dapr) are listed under the option's `Needs` and checked before generation.
- `go run ./tools/refreshpins` (new, in the Projemble repo) queries `proxy.golang.org/<module>/@latest` for every registry entry. It prints a diff of new versions and go-directive changes but does not auto-edit. A test fails if any registry entry has `VerifiedAt` older than 120 days, which forces a scheduled review.
- Pins are exact. v0.x and brand-new majors (Connect v2, anthropic-sdk-go v2, dockertest v4, Kratos v3, Milvus client v3) are `experimental` until a newer patch release ships or 60 days pass without breaking issues.

### 3.6 Licenses and extra requirements

- The UI shows a badge for anything that is not MIT/BSD/Apache-2.0. Currently:
  - go-sql-driver/mysql, River, chromem-go: **MPL-2.0**, file-level copyleft
  - Encore: MPL-2.0
  - Consul server: **BSL 1.1**; the `consul/api` client is MPL-2.0
  - Atlas: the standard binary unlocks paid Pro features after login; only the Community Edition is Apache-2.0
- The UI also shows badges for options that need Docker, cgo, an external CLI, or a running service. The generated README lists them under "Requirements".

### 3.7 Testing many combinations without an explosion

- **Behaviour checks when tests are excluded:** Projemble keeps its own test files for each template under `internal/generator/testdata/behaviour/`. When the user excludes tests, Projemble's test suite copies those files into a temporary copy of the generated project and runs them there. The user's project never receives them, but Projemble still knows the code works. Both the "tests included" and "tests excluded" variants are part of the matrix.
- **Per-option test:** each option is generated on top of the Recommended preset with only that one step changed, so every option is built and tested once.
- **Pairwise test:** a small covering-array generator in the test package picks combinations so every pair of options across steps appears at least once (dozens of projects, not thousands). Results are cached by plan hash.
- **Preset tests:** each preset across every architecture.
- **Fast tier** (`go test -short`): render, gofmt, golden snapshots, static assertions, plan resolution. No compiling; target under 20s.
- **Slow tier:** `go mod tidy`, vet, build, test of generated projects. Shares `GOMODCACHE`/`GOCACHE`, runs with `t.Parallel()`. Docker-dependent integration tests (testcontainers) run only when `PROJEMBLE_DOCKER_TESTS=1`.
- **Golden snapshots** under `internal/generator/testdata/golden/<preset>/<architecture>/`, updated with `-update`.

### 3.8 Wizard UX (`internal/tui`)

- Order: Workload → Topology → Preset → Architecture → one screen per applicable step → Review.
- Each option row shows: name, a one-line summary, and badges (Recommended / Experimental / license / Docker / cgo / CLI / Go ≥ X).
- Options that conflict with earlier choices are shown greyed out with the reason ("requires PostgreSQL"), not hidden.
- The Review screen lists every choice, the resolved Go version, added modules and tools, and requirements. Each line can be changed in place.
- Keyboard: arrow keys move, Enter selects, `?` shows details (status notes, links), `p` switches preset. Typing in the search field filters options. This reuses the slash palette's filtering.
- The CLI gets the same steps through `projemble new --preset recommended --choice router=chi --choice data-access=sqlc`.

### 3.9 After creation, every change goes through the agent

The deterministic generator runs **once**, when the project is created. It already refuses to overwrite files, and that stays. After that, the project belongs to the user and may have been edited, so re-rendering templates would destroy work. Every later change, including changing an option chosen in the wizard, is an **agent task**.

- **Option-change commands.** The workspace offers commands built from the option registry, for example:
  - `/add tests` (with the test-style sub-choices)
  - `/switch logger zap`
  - `/add rate-limiting`
  - `/switch router chi`
  - `/add docker`
  - `/add observability otel`

  Each command shows the same option picker as the wizard, limited to choices compatible with the project's current profile, plus a free-text field for anything extra.
- **What the agent receives.** The command becomes a structured task:
  - the current choices and the requested change
  - the exact module pins from the registry
  - the seam to change (§3.4)
  - the option's skill line or packaged skill (loaded with `read_skill`)
  - the generated reference template for that option, as read-only example content the agent adapts to the project's current code instead of copying blindly
  - the instruction "change only what this option touches; keep other choices"
- **Same verification gate.** After the agent turn, the Phase 4 gate runs: gofmt, vet, build, plus tests if the project has them, with up to 2 repair rounds.
- **Profile stays truthful.** `Choices` in the project profile is updated **only after the gate passes**. If the gate fails, the profile keeps the old choice and the project is marked `needs-attention`. The README's "Chosen options" section is updated in the same agent turn.
- **Removing an option** (e.g. switching zap → fmt) also removes the now-unused modules (`go mod tidy`) and their config variables. The gate checks for this with `go mod tidy -diff`.
- **No silent regeneration.** There is no "regenerate project" button for existing projects. Creating a fresh project from the same profile into a new directory is allowed, and is useful for comparing results.

---

## 4. Decision steps and options

Verdicts: **R** = recommended default, **S** = supported option, **X** = experimental, **L** = legacy/not offered (with reason). Versions are from 2026-10-22.

### 4.1 Workload and topology (existing; unchanged)
`http-api` (monolith | microservices), `cli`, `one-shot-job`, `background-worker`, `library`.

### 4.2 Architecture (existing, made real in Phase 2)
Layered (R for small services), Clean/Hexagonal (S), DDD (S), Pipeline (jobs only). Candidate for later: **Vertical slice** (feature folders with handler + logic + store per use case).

### 4.3 Microservice communication (topology = microservices)

| Option | Module @ version | Min Go | Needs | Verdict / notes |
| --- | --- | --- | --- | --- |
| Connect | `connectrpc.com/connect` v1.21.1 · v2.0.2 | v1 < 1.26 · v2 1.26 | buf (to change the contract), `.proto` | **R** (v1; §6.5). Works on `net/http`, curl-able, serves gRPC + gRPC-Web. v2 = **X** (two weeks old, new API) |
| HTTP/JSON between services | stdlib `net/http` | 1.22 | — | **S**; the Minimal preset default. Simplest, debuggable, no codegen; typed client package generated alongside |
| gRPC | `google.golang.org/grpc` v1.84.0, protobuf v1.36.12 | 1.25 | buf CLI, `.proto` | **S**. Standard, streaming; HTTP/2 only |
| go-micro | `go-micro.dev/v6` v6.15.1 | 1.26 | optional `micro` CLI | **S**, pinned exact minor; template must import `cmd/defaults` (v6.12 change). Note v6's agent/MCP direction. v5 = L |
| go-zero | `github.com/zeromicro/go-zero` v1.10.3 | 1.24 | `goctl` CLI, `.api` DSL | **S** as a *framework-owned template*: layout produced by goctl, Projemble adds config, tests, README |
| Kratos | `github.com/go-kratos/kratos/v3` v3.0.0 | 1.25 | `kratos` CLI, `.proto` | **X** (v3 is new; docs mostly Chinese). Framework-owned template |
| NATS micro | `github.com/nats-io/nats.go/micro` (nats.go v1.54.0) | 1.26 | NATS server | **S** for messaging-first systems |
| Dapr | `github.com/dapr/go-sdk` v1.15.0 | 1.26.4 | Dapr sidecar + CLI | **S** (advanced / Kubernetes) |
| go-kit | `github.com/go-kit/kit` v0.13.0 | — | — | **L**: dormant since 2023-08 |
| Twirp | `github.com/twitchtv/twirp` v8.1.3 | — | — | **L**: no release since 2022, no go.mod |
| Encore | `encore.dev` | — | encore CLI owns build | **L** for this generator: plain `go build` doesn't work |

Sub-step **service discovery** (when not provided by the framework): env-var URLs / Kubernetes DNS (**R**), Consul `consul/api` v1.35.0 (S; BSL server), etcd `go.etcd.io/etcd/client/v3` v3.7.2 (S). mDNS is not offered on its own (it comes with go-micro).

Sub-step **resilience** for outbound calls: `failsafe-go/failsafe-go` v0.9.8 (**R**; retry, circuit breaker, timeout, bulkhead, with HTTP/gRPC adapters; pre-1.0, pin exact), `sony/gobreaker/v2` v2.4.0 (S), `cenkalti/backoff/v7` v7.0.1 (S; pin the major), hand-written bounded retry (Minimal preset).

### 4.4 HTTP router (workload = http-api)

| Option | Module @ version | Min Go | net/http | Verdict / notes |
| --- | --- | --- | --- | --- |
| stdlib ServeMux | — | 1.22 | native | **R** (Minimal), method and `{wildcard}` patterns; generated `Chain()` middleware helper |
| chi | `github.com/go-chi/chi/v5` v5.5.0 | 1.24 | yes | **R** (when route groups are wanted; Recommended preset) |
| gin | `github.com/gin-gonic/gin` v1.12.0 | 1.25 | partial (`gin.WrapH`) | **S**. Own context; binding via validator |
| echo v5 | `github.com/labstack/echo/v5` v5.4.0 | 1.25 | yes | **S**. echo v4 = **L** (end of life 2026-12-31) |
| fiber v3 | `github.com/gofiber/fiber/v3` v3.6.1 | 1.26 | **no** (fasthttp) | **S** as a *separate track*: net/http middleware (httprate, OTel, kin-openapi) needs adaptors; template uses Fiber-native middleware. fiber v2 = L |
| huma | `github.com/danielgtaylor/huma/v2` v2.39.1 | 1.25 | yes (over stdlib/chi/gin/echo/fiber) | Offered under API contract (4.5); it sits on top of the chosen router |
| gorilla/mux | v1.8.1 | — | yes | **L**: no release since 2023; stdlib/chi cover it |

### 4.5 API contract style (workload = http-api)

| Option | Module @ version | Verdict / notes |
| --- | --- | --- |
| Hand-written routes + hand-written `api/openapi.yaml` + a route-coverage test | — | **R** (Minimal) |
| Spec-first: oapi-codegen | `github.com/oapi-codegen/oapi-codegen/v2` v2.8.0 (tool directive) | **R** (Contract-first preset). Strict server for std-http/chi/gin/echo/fiber, OpenAPI 3.0/3.1 |
| Spec-first runtime validation | `github.com/getkin/kin-openapi` v0.151.0 | **S** add-on to oapi-codegen (`openapi3filter` middleware); pre-1.0 |
| Code-first: huma | `github.com/danielgtaylor/huma/v2` v2.39.1 | **S**. Generates OpenAPI 3.1 from Go types, built-in validation and RFC 9457 errors; **replaces** the Validation step |
| ogen | `github.com/ogen-go/ogen` v1.24.0 | **S**. Spec-first with no reflection, OTel built in; **owns the router**, so it conflicts with stdlib/chi/gin route mixing |
| swaggo/swag | v1.16.6 (v2 still RC) | **L**: Swagger 2.0 only; comment drift |

### 4.6 Request validation (unless replaced by huma)

| Option | Module @ version | Min Go | Verdict |
| --- | --- | --- | --- |
| Hand-written `Validate() error` returning field errors | — | any | **R** (Minimal); easiest for the AI step to extend consistently |
| go-playground/validator | `github.com/go-playground/validator/v10` v10.30.5 | 1.26 | **R** (Recommended); gin uses it already |
| ozzo-validation | `github.com/go-ozzo/ozzo-validation/v4` v4.4.1 | 1.13 | **S**: rules in code; revived in 2026 after a six-year gap |

All options produce the same `ValidationError{Fields map[string]string}`, so error-to-status mapping is the same for every choice.

### 4.7 Database engine (existing capability, unchanged list)
SQLite, PostgreSQL, MySQL/MariaDB, MongoDB (all supported); later CockroachDB, TiDB, ScyllaDB/Cassandra, DynamoDB.

Sub-step **SQLite driver**: `modernc.org/sqlite` v1.61.0 (**R**; pure Go, Go ≥ 1.26), `github.com/mattn/go-sqlite3` v1.14.52 (S; **cgo**), `github.com/ncruces/go-sqlite3` v0.35.6 (X; pure Go via wasm2go).
Other drivers (fixed per engine): `jackc/pgx/v5` v5.11.0, `go-sql-driver/mysql` v1.10.1 (MPL-2.0), `go.mongodb.org/mongo-driver/v2` v2.9.2 (v2 only).

### 4.8 Data access style (SQL engines)

| Option | Module @ version | Needs | Verdict / notes |
| --- | --- | --- | --- |
| database/sql + hand-written queries | stdlib | — | **R** (Minimal); SQLite/MySQL baseline |
| pgx native (pgxpool) | `jackc/pgx/v5` v5.11.0 | Postgres | **S**: fastest Postgres path, COPY, LISTEN |
| sqlc | `github.com/sqlc-dev/sqlc` v1.31.1 (tool) | codegen | **R** (Recommended): type-safe code from SQL; emits pgx/v5 or database/sql code; supports SQLite and MySQL too |
| sqlx | `github.com/jmoiron/sqlx` v1.4.0 | — | **S**: low magic; low activity since 2024 |
| GORM | `gorm.io/gorm` v1.31.2 + driver | — | **S**: popular. AutoMigrate is dev-only; production migrations still use the Migrations step |
| Bun | `github.com/uptrace/bun` v1.3.0 | — | **S**: SQL-first ORM, PG/MySQL/SQLite |
| ent | `entgo.io/ent` v0.14.6 | codegen | **X** (pre-1.0): schema-as-code; pairs with Atlas |
| Jet | `github.com/go-jet/jet/v2` v2.16.1 | codegen needs a live DB | **S** (advanced) |
| Bob | `github.com/stephenafamo/bob` v0.50.0 | optional codegen | **X**: sqlboiler's successor, pre-1.0 |
| Squirrel | `github.com/Masterminds/squirrel` v1.5.4 | — | Add-on only, for dynamic filters ("complete", no development) |
| sqlboiler | v4.19.7 | — | **L**: maintenance mode; maintainers point to Bob |

MongoDB has a single data access option: the official driver with typed collection helpers.

### 4.9 Migrations (SQL engines)

| Option | Module @ version | Min Go | Verdict / notes |
| --- | --- | --- | --- |
| Embedded runner (generated, `embed.FS` + `schema_migrations`) | — | any | **R** (Minimal / SQLite) |
| goose | `github.com/pressly/goose/v3` v3.29.0 | 1.26 | **R** (Recommended): SQL or Go migrations, `embed.FS`, Provider API |
| golang-migrate | `github.com/golang-migrate/migrate/v4` v4.20.1 | 1.25 | **S**: widely used; manual recovery from "dirty" state is documented in the README |
| tern | `github.com/jackc/tern/v2` v2.4.3 | 1.25 | **S** (Postgres only) |
| Atlas | `ariga.io/atlas` v1.3.0 (CLI) | 1.26 | **S** with edition warning; default with ent |
| dbmate | `github.com/amacneil/dbmate/v2` v2.37.0 | 1.26 | **S** (CLI-centric) |

MongoDB: an `EnsureIndexes(ctx)` step plus versioned index/data migrations in Go.

### 4.10 Cache / key-value (optional)
None (**R**), `redis/go-redis/v9` v9.23.0 (S; Go ≥ 1.26; works with Redis and Valkey), `redis/rueidis` v1.0.79 (S; high throughput), `valkey-io/valkey-go` v1.0.79 (S). Sub-step for the server image in compose/tests: **Valkey (R; BSD)** or Redis (license note). Generated cache code must specify key ownership, TTL, size limits and tenant-aware keys (efficient-code skill §6).

### 4.11 Messaging, background jobs, scheduling (http-api with workers, background-worker)

| Option | Module @ version | Needs | Verdict / notes |
| --- | --- | --- | --- |
| None / in-process bounded worker pool | — | — | **R** (Minimal) |
| River | `github.com/riverqueue/river` v0.49.0 | Postgres | **S** when DB = Postgres: transactional enqueue; MPL-2.0 (badge; not Recommended per §6.6); pre-1.0 |
| asynq | `github.com/hibiken/asynq` v0.26.0 | Redis/Valkey | **S** (verify the tag before pinning) |
| NATS JetStream | `github.com/nats-io/nats.go` v1.54.0 (`jetstream`) | NATS | **S** |
| Kafka | `github.com/twmb/franz-go` v1.22.2 | Kafka | **S** (R Kafka client); `IBM/sarama` v1.61.1 = S; `segmentio/kafka-go` = L (v0.4, slow) |
| RabbitMQ | `github.com/rabbitmq/amqp091-go` v1.16.0 | RabbitMQ | **S**: generated reconnect loop with backoff |
| Watermill | `github.com/ThreeDotsLabs/watermill` v1.5.3 | a backend | **S**: pub/sub abstraction, CQRS helpers |
| Temporal | `go.temporal.io/sdk` v1.50.0 | Temporal server | **S** (advanced durable workflows) |

Sub-step **scheduling**: none (R), `go-co-op/gocron/v2` v2.22.0 (S; R when scheduling is needed). `robfig/cron/v3` = L (unmaintained since 2021).

Every messaging option implements the same `Publisher`/`Consumer` ports. Each one must have limits on concurrency, retries with backoff, a dead-letter or failure path, idempotency keys, and graceful drain.

### 4.12 Configuration

| Option | Module @ version | Verdict |
| --- | --- | --- |
| stdlib `os.LookupEnv` + generated parser | — | **R** (Minimal) |
| caarlos0/env | `github.com/caarlos0/env/v11` v11.4.1 | **R** (Recommended): struct tags, defaults, required, no dependencies |
| koanf | `github.com/knadh/koanf/v2` v2.4.1 | **S**: files + env + flags, modular |
| viper | `github.com/spf13/viper` v1.21.0 | **S**: familiar; heavy, global state, lowercases keys |
| envconfig / cleanenv | — | **L**: stale releases |

### 4.13 Logging (every option sits behind the `logging.Logger` seam, §3.4)

| Option | Module @ version | Output | Verdict / notes |
| --- | --- | --- | --- |
| **fmt** | stdlib `fmt` | `2026-10-22T10:00:00Z INFO request done method=GET status=200 request_id=…` to stdout (errors to stderr) | **Default** for the step and the Minimal preset. No dependencies; the generated implementation formats key=value pairs, quotes values containing spaces, and filters by `LOG_LEVEL` |
| stdlib `log` | stdlib `log` | same line format, with `log` flags and prefix | **S**: for users who prefer the `log` package |
| slog | stdlib `log/slog` | JSON or text handler (`LOG_FORMAT`) | **R** in the Recommended and Contract-first presets: structured, standard, works with OTel |
| zap | `go.uber.org/zap` v1.28.0 | JSON / console | **S**: very fast, sampling; wrapper uses `SugaredLogger.Infow` etc. |
| zerolog | `github.com/rs/zerolog` v1.35.1 | JSON / console writer | **S**: zero-allocation JSON |
| logrus | `github.com/sirupsen/logrus` | JSON / text | *Pin and status to verify before offering*; logrus is widely used but in maintenance mode. Offer as S or L depending on that check |
| None | — | discards everything | **S** for libraries and tiny CLIs; the interface stays so a logger can be added later |

Add-ons (multi-select, shown only when they apply):
- **Pretty dev output:** `github.com/lmittmann/tint` v1.2.1 (slog only, when `LOG_FORMAT=text`).
- **OTel log export:** `go.opentelemetry.io/contrib/bridges/otelslog` v0.21.0 (slog only, when observability = OTel). zap and zerolog use their own OTel bridges, which must be researched before they are offered.
- **Request-scoped fields:** request ID, route, and user ID (when auth is chosen), added through `With(...)` in middleware. Same for every logger.

Libraries are the exception: the library workload defaults to **None** and exposes an optional `Logger` hook (or an `*slog.Logger` option) instead of logging on its own.

### 4.14 Dependency injection
Manual constructor wiring (**R**), `go.uber.org/fx` v1.24.0 (S; lifecycle hooks; runtime errors), `github.com/samber/do/v2` v2.1.0 (S). `google/wire` = **L** (archived 2025-08-25).

### 4.15 CLI framework (workload = cli; also `cmd/migrate`, `cmd/seed`)
`spf13/cobra` v1.10.2 (**R**; subcommands, completion), `urfave/cli/v3` v3.14.0 (S), `alecthomas/kong` v1.16.1 (S), stdlib `flag` (S; Minimal). `peterbourgon/ff/v4` = X (still beta).

### 4.16 Authentication and authorization (http-api, optional)

Authentication:

| Option | Module @ version | Verdict / notes |
| --- | --- | --- |
| None | — | **R** default for starters |
| Static API keys (dev) | — | **S**: constant-time compare, keys from config, never logged |
| JWT (own issuer/verifier) | `github.com/golang-jwt/jwt/v5` v5.3.1 | **S**: algorithm allow-list, `exp`/`nbf`/`aud`/`iss` validation required |
| OIDC / SSO | `github.com/coreos/go-oidc/v3` v3.21.0 + `golang.org/x/oauth2` v0.37.0 | **S** (R for SSO); Go ≥ 1.26 |
| JOSE/JWKS-heavy | `github.com/lestrrat-go/jwx/v3` v3.3.0 · v4 v4.5.0 | **S** (v3) / **X** (v4 uses json/v2: Go 1.27, or 1.26 with `GOEXPERIMENT=jsonv2`) |

Authorization: none / role checks in the application layer (**R**), `github.com/casbin/casbin/v3` v3.11.0 (S; repo now Apache Casbin; v2 = L), `github.com/openfga/go-sdk` v0.8.3 (X; needs an OpenFGA server).

### 4.17 Rate limiting (http-api, optional)
`github.com/go-chi/httprate` v0.16.1 (**R**; in-memory, works with any net/http router), `go-chi/httprate-redis` v0.7.0 (S; distributed), `golang.org/x/time/rate` v0.16.0 + generated keyed middleware with a size-limited key map (S; Minimal), `sethvargo/go-limiter` v1.2.0 (S). Fiber uses its own limiter middleware. `ulule/limiter` and `go-redis/redis_rate` = L (no releases since 2023).

Security rule for every option: the generated code determines the client IP through a trusted-proxy extractor (configured CIDRs). It never emits httprate's deprecated `KeyByIP`/`KeyByRealIP`.

### 4.18 Observability (optional)
None (R for Minimal), OpenTelemetry `go.opentelemetry.io/otel` v1.47.0: traces, metrics and logs all stable; OTLP log exporters still v0.x (**R** when observability is enabled), Prometheus `prometheus/client_golang` v1.25.0 (S; `/metrics` endpoint). Instrumentation follows the router choice (otelhttp for net/http routers, framework contrib packages otherwise).

### 4.19 Tests (optional; a step of its own)

First question: **"Include tests in the generated project?"**

| Option | What is generated | Verdict |
| --- | --- | --- |
| No tests | no `_test.go` files, no test dependencies, no test sections in README/CI | **Default** (pre-selected in every preset). Projemble still verifies behaviour with its own tests (§3.7). The AI step is told not to add tests unless the user asks |
| Yes: unit tests | tests for domain/service/handlers/config/middleware | **S** |
| Yes: unit + integration | unit tests plus database/broker tests against real services (Docker) | **S** |

Tests can be added after creation through the agent (§3.9), e.g. `/add tests`.

The sub-steps below appear only when tests are included:
- **Assertions:** stdlib `testing` + `google/go-cmp` v0.7.0 (**R** Minimal), `stretchr/testify` v1.12.1 (**R** Recommended).
- **Test doubles:** hand-written fakes (**R**), `vektra/mockery/v3` v3.8.1 (S; tool directive), `go.uber.org/mock` v0.6.0 (S). `golang/mock` = L (archived).
- **Goroutine-leak checks:** `go.uber.org/goleak` v1.3.0 in `TestMain` for worker and server packages (on by default when the workload runs goroutines; can be turned off).
- **Integration harness** (only with "unit + integration"): `testcontainers/testcontainers-go` v0.44.0 + per-engine modules (**R**; requires Docker; gated by env var), `ory/dockertest/v4` v4.0.1 (X; brand-new major).
- **Race detector in CI:** on (**R**) or off (for cgo-less CI runners).

Related effects of "No tests":
- The CI workflow (§4.20) runs gofmt, vet and build only.
- The repository contract test and architecture dependency-direction test (§5 Phase 2) are not shipped, but they still run inside Projemble's own test suite.

### 4.20 Delivery (optional, multi-select)
Dockerfile (multi-stage, `CGO_ENABLED=0` unless a cgo option is chosen, distroless/static non-root), `compose.yaml` with the chosen DB/cache/broker and healthchecks, GitHub Actions CI (gofmt, vet, test, build, service containers, module cache), Makefile or Taskfile. *To research before Phase 8:* GitLab CI, Taskfile version, Kubernetes manifests/Helm, goreleaser for CLIs.

### 4.21 AI application stack (patterns = rag / agent / chatbot)

LLM provider SDK (one or more; always behind a `Model` port):

| Option | Module @ version | Verdict / notes |
| --- | --- | --- |
| OpenAI and OpenAI-compatible endpoints | `github.com/openai/openai-go/v3` v3.85.1 | **R**; import path has changed across majors, pin `/v3` |
| Anthropic Claude | `github.com/anthropics/anthropic-sdk-go/v2` v2.1.0 | **S** (v2 released 2026-10-21, so X for the first weeks; v1 v1.84.0 fallback) |
| Google Gemini / Vertex | `google.golang.org/genai` v1.75.0 | **S** |
| AWS Bedrock | `aws-sdk-go-v2/service/bedrockruntime` v1.63.6 | **S** (Converse API) |
| Ollama (local) | via openai-go against Ollama's OpenAI-compatible endpoint | **S**: avoids pulling in the whole `ollama/ollama` module |
| sashabaranov/go-openai | — | **L**: unofficial, superseded |

Orchestration framework:

| Option | Module @ version | Verdict / notes |
| --- | --- | --- |
| None: Projemble-generated ports + bounded loops | — | **R**: smallest dependency surface; the same limits in every case |
| Genkit | `github.com/firebase/genkit/go` v1.14.0 | **S**: stable 1.x, flows, tools, RAG, dev UI |
| Eino | `github.com/cloudwego/eino` v0.9.22 | **X**: graph orchestration, ReAct/multi-agent; pre-1.0 with v0.10 alpha |
| Google ADK | `google.golang.org/adk` v1.9.0 | **S** for agent projects; Gemini-centric |
| LangChainGo | `github.com/tmc/langchaingo` v0.1.15 | **X**: many integrations, pre-1.0, large issue backlog |

Vector store (RAG):

| Option | Module @ version | Verdict / notes |
| --- | --- | --- |
| In-memory store (tests/dev) | — | **R** (dev) |
| pgvector | `github.com/pgvector/pgvector-go` v0.4.1 | **R** with Postgres |
| Qdrant | `github.com/qdrant/go-client` v1.19.4 | **S** |
| Weaviate | `weaviate-go-client/v5` v5.7.3 | **S** (v6 RC exists; plan the move) |
| Milvus | `github.com/milvus-io/milvus/client/v3` v3.0.0 | **X** (old `milvus-sdk-go` archived) |
| chromem-go (embedded) | `github.com/philippgille/chromem-go` v0.7.0 | **X** (dev only; MPL-2.0; stale tags) |
| Redis search | `go-redis/v9` FT.* | **S** (Redis 8 / Stack; RESP2) |

MCP (agent pattern, optional): `github.com/modelcontextprotocol/go-sdk` v1.8.0 (**R**; official, v1 compatibility promise), `github.com/mark3labs/mcp-go` v1.3.0 (S).

Chat channels (chatbot pattern, multi-select; each behind a `Channel` adapter): HTTP/web (R), Slack `slack-go/slack` v0.30.1 (S), Telegram `go-telegram/bot` v1.27.0 (S) or `mymmrac/telego` v1.12.1 (S), Discord `bwmarrin/discordgo` v0.29.0 (X; releases stalling), WhatsApp Cloud API via generated `net/http` client (S). `go-telegram-bot-api` and `whatsmeow` = L (abandoned / unofficial protocol).

---

## 5. Implementation phases

Each phase is one or more PRs. Tests pass after every PR, and existing profiles keep generating.

### Phase 0: Foundations (F4, F10, F11, F12)

Split into four PRs, in this order. No new wizard screens or new options yet; users see the same starters, but formatted and with a newer `go` line.

**0a. Template files + format on render (F4, F10)**
1. Move starters out of `fmt.Sprintf` strings into `internal/generator/templates/**.go.tmpl` (`text/template`, `embed`). A Go-side manifest maps each current template to its part files and output paths.
2. Pass every `.go` output through `go/format`, normalize line endings, end each file with one newline. A format failure is a template bug: return an error naming the template and write nothing.
3. Golden snapshots of the full output tree for every current template, in `testdata/golden/`, updated with `-update`.
*Acceptance:* every current template renders from files; output differs from today's only by gofmt formatting (reviewed once in the first snapshot diff); generated projects still pass vet, build and test.

**0b. Go version policy (F11, F12)**
1. Baseline `go 1.26.0`; the `go` line is computed as the maximum of the baseline and each chosen module's minimum (§3.5). No `toolchain` line unless an option needs a patch release.
2. Check the local toolchain (`go env GOVERSION`) before generating and say what to do if it is too old.
3. Re-pin go-micro to the current v6 minor, import `cmd/defaults`, and verify the microservices templates still build.
*Acceptance:* all templates build under Go 1.26 and 1.27; table tests cover the `go`-line computation.  **Done.**

**0c. Option model (F5)**
1. `internal/catalog`: `Step`, `Option`, the module registry with `VerifiedAt`, `Resolve(Selection)`, and presets (§3.1–3.2). Convert the existing `stack`, `service-framework` and capability data into steps, keeping today's choices as the defaults.
2. `internal/projectstore`: `Choices` and `Preset` fields with backward-compatible defaults, so existing YAML loads and generates the same structure (§3.3).
3. No wizard changes yet: the wizard still produces today's choices, now stored as `Choices`.
*Acceptance:* table tests for defaults, presets, `requires`/`conflicts`/`replaces`, and an old-profile load test; `Resolve` reports all problems at once.  **Done.**

**0d. Quality harness (F8 prerequisite)**
1. `assertQualityBar` replaces `verifyGenerated`: gofmt check, vet, build, and `test` (with `-race` when cgo is available) when the project has tests. Static rules: no `ListenAndServe(`, no `log.Fatal` outside `main`, no file over 1,000 lines, `go mod tidy -diff` clean. "Tests exist for each package" applies **only when the plan includes tests** (§4.19).
2. Fast tier (`go test -short`: render, gofmt, golden, plan resolution, static rules; target under 20s) and slow tier (tidy, vet, build and test of generated projects, `t.Parallel()`, shared `GOMODCACHE`/`GOCACHE`).
3. `go run ./tools/refreshpins` and the test that fails when a registry entry's `VerifiedAt` is older than 120 days.
*Acceptance:* `go test -short ./internal/generator` finishes in under 20s; the full matrix is faster than today's ~320s (measured and recorded in the PR).

### Phase 1: Production-safe HTTP runtime (F1), stdlib path
- `internal/config` (loader interface; stdlib implementation first), `internal/platform/logging` (the `Logger` interface with the `fmt` and `slog` implementations first; the others in Phase 5), `internal/platform/httpserver` (timeouts: ReadHeader 5s, Read 15s, Write 30s, Idle 60s; MaxHeaderBytes; graceful shutdown via `signal.NotifyContext`), middleware (request ID, panic recovery, access log, body-size limit, per-request deadline), `respond` (JSON + RFC 9457 problem responses), health (`/healthz`, `/readyz` with checkers).
- `main` → `run(ctx, getenv, stdout) error` so it can be tested.
- Tests (shipped when the user includes tests; otherwise kept as Projemble behaviour checks): config defaults/invalid values, middleware tables, graceful shutdown on `127.0.0.1:0`, health endpoints, and the logger's level filtering and key=value formatting.

### Phase 2: Reference slice per architecture (F2, F7)
- An example resource (`items`, §6.1) with create/get/keyset-paginated list/patch/delete in Layered, Clean and DDD layouts. Domain errors are mapped in one place. Clock and ID generator are injected.
- A repository contract test shared by every repository adapter.
- An architecture dependency-direction test generated into Clean and DDD projects.
- Tests for domain, service and handler, including pagination ties and the body-size limit.

### Phase 3: Persistence defaults (F3)
- Connection setup per engine with pool limits, `PingContext` with a timeout, readiness checker, and close on shutdown. SQLite gets WAL, busy_timeout and foreign_keys. Mongo gets ping, `SetMaxPoolSize`, disconnect.
- Migrations: the embedded runner (Minimal) and goose (Recommended). Repository adapters: database/sql (Minimal) and sqlc on pgx/database/sql (Recommended), with keyset indexes. Run the contract test against SQLite always and other engines behind an env var.

### Phase 4: AI generation pipeline (F8, F9)
Doing this early pays off: the starter is now a good example, so the AI step should be held to it.
- `generationTask` includes the chosen stack ("use only these libraries: …; do not add competing ones"), the quality bar in short form, the reference slice to copy, explicit acceptance criteria derived from the description, and a request for a short plan before editing.
- **Verification gate owned by Projemble:** after the agent turn, Projemble runs gofmt, vet, build, and test (test only when the user included tests) (plus `go tool sqlc generate`/`oapi-codegen` checks when chosen, so generated code isn't stale). Failures go back to the agent session for up to 2 repair rounds within the time budget. Status is `ready` only if the gate passes, otherwise `needs-attention` with the last failure saved for Resume. The CLI `projects agent` path uses the same gate.
- **Option-change commands (§3.9):** `/add …` and `/switch …` in the workspace build structured agent tasks from the option registry. They use the same gate, and update the profile's `Choices` only after the gate passes. The first commands are `/add tests` and `/switch logger`; the others follow as their options land in later phases.
- Each option contributes a one-line profile skill. Larger packaged skills (`go-service-quality`, `go-testing`, `sqlc`, `oapi-codegen`) are loaded through `read_skill`.

### Phase 5: Option wave 1 (most-used choices)
Router: chi, gin, echo v5. Validation: validator. Config: caarlos0/env. Logging: `log`, zap, zerolog, None (logrus after its status check). Tests: the include/exclude switch, testify, goleak. CLI: cobra. Data access: pgx native, GORM, sqlx. Migrations: golang-migrate. Rate limiting: httprate. Observability: OTel. Delivery: Dockerfile, compose, GitHub Actions.
For each option: templates behind its seam, registry pins, resolver rules, per-option test, pairwise inclusion, README section, skill line. Add the Popular-frameworks preset.

### Phase 6: Other workloads at the same bar
CLI (`Run(ctx, args, stdin, stdout, stderr) int`, exit codes, version info, cobra/urfave/kong/flag options), background worker (fixed worker pool, per-task timeout, bounded retries, drain on SIGTERM, optional health listener), one-shot job (typed stage state, stage timeouts, `--dry-run`, batching), library (`doc.go`, runnable examples, functional options, sentinel errors).

### Phase 7: API contract options
oapi-codegen spec-first (strict server per router), kin-openapi runtime validation, huma code-first (replaces Validation), ogen (owns the router). Contract-first preset. Route-coverage and spec-validity tests.

### Phase 8: Microservices options
Connect v1 (R) with a gateway → catalog example, `buf.yaml`/`buf.gen.yaml`, committed generated code, failsafe-go resilience, env/K8s discovery. Then HTTP/JSON with a generated typed client (Minimal), gRPC-Go, Connect v2 once it leaves experimental, go-micro v6 (re-pinned), NATS micro; then framework-owned templates for go-zero and Kratos, and Dapr. First, a short benchmark of Connect vs gRPC-Go vs HTTP/JSON on the example, and a check that `buf` works as a `go tool` entry (§6.5). Every option gets the same gateway → catalog example and an in-process end-to-end test where the framework allows it.

### Phase 9: Messaging, jobs, cache, scheduling
`Publisher`/`Consumer` ports and the outbox hook from DDD events; River, asynq, NATS JetStream, franz-go, sarama, amqp091, Watermill, Temporal; gocron; go-redis / rueidis / valkey-go with cache rules. Every option is tested with testcontainers behind the Docker env var.

### Phase 10: Auth and authorization
API keys, JWT (golang-jwt), OIDC (go-oidc + oauth2), jwx; Casbin v3, OpenFGA. Tests for algorithm allow-lists, expiry/audience/issuer checks, constant-time comparison, and checks that secrets never reach logs.

### Phase 11: AI application stack
Ports (`Model`, `Embedder`, `Retriever`, `VectorStore`, `Tool`, `Channel`) with bounded loops and history (max steps, top-K limit, history window, per-call timeouts, approval hook). Adapters for openai-go, anthropic v2, genai, Bedrock; Genkit / Eino / ADK / LangChainGo as framework options; pgvector, Qdrant, Weaviate, Milvus, chromem, Redis vector stores; MCP SDKs; Slack, Telegram, Discord, WhatsApp channels. Fake model/embedder for deterministic tests; no network in default tests.

### Phase 12: Remaining options and upkeep
DI options (fx, do), koanf/viper, zap/zerolog via slog, mockery/uber mock, tern/Atlas/dbmate, ent/Bun/Jet/Bob, alternative SQLite drivers, Prometheus, dockertest v4, delivery extras (§4.20 research). Ongoing: run `refreshpins` monthly, promote experimental options when they stabilize, and move deprecated ones to legacy (echo v4 after 2026-12-31).

---

## 6. Decisions

The user's rule: **pick the best option available, checked against current best practice.** Decisions 1–2 were made by the user directly. Decisions 3–7 follow that rule; revisit any of them if you disagree.

1. **Example resource: `items`** (user decision). Fields: name + optional description. After creation, the agent renames or replaces it for the real domain (§3.9).
2. **Built-in vs optional:** config, health and validation are always generated. Tests are optional and **off by default** (§4.19). Logging is a step with several loggers, **`fmt` by default** (§4.13). Changes after creation always go through the agent (§3.9).
3. **First preset shown: Recommended.** Minimal is one keypress away.
4. **Go version:** `go 1.26.0` floor, raised automatically by the options chosen (§3.5). Follow the official toolchain rules: the `go` line is the minimum required (and must be ≥ every dependency's), and Projemble does **not** add a `toolchain` line unless an option needs a specific patch release. That avoids forcing downloads on mixed-version teams. Projemble itself targets the two supported Go releases (1.26 and 1.27).
5. **Microservice communication:**
   - **Recommended: Connect (v1), over protobuf.** Reasoning: current guidance favours gRPC-style typed contracts for internal Go service-to-service calls, with plain REST/JSON at the edge. Connect speaks gRPC, gRPC-Web and its own curl-friendly protocol on plain `net/http`, so the same middleware, graceful shutdown and observability apply as in the HTTP runtime. Generated code is committed, so building the project needs no codegen tool. Changing the contract needs `buf`, which is added as a `go tool` entry if that works, otherwise listed under Requirements (verify in Phase 8).
   - **Supported:** gRPC-Go, plain HTTP/JSON (the Minimal preset default, because it needs no codegen), go-micro v6, NATS micro, go-zero, Kratos, Dapr.
   - **Evidence caveat:** the sources I found were vendor and blog posts, with no authoritative 2026 consensus, and one unverified benchmark claimed grpc-go is faster than Connect for streaming. Phase 8 includes a small benchmark on our own example before the default is final.
   - Connect v2 stays **Experimental** until it has been out for 60 days (§3.5).
6. **Licenses:** show badges and **never hide** an option for its license. Recommended defaults must be permissive (MIT/BSD/Apache-2.0) unless the engine leaves no permissive choice (the MySQL driver is MPL-2.0 and the only mainstream one, so it stays). Because of this, **River moves from Recommended to Supported** (MPL-2.0); the Recommended job-queue choice for Postgres becomes the in-process bounded worker pool, with River as a one-pick option. BSL and edition-gated tools (Consul server, Atlas) are Supported with a visible notice.
7. **Framework-owned templates (go-zero, Kratos):** allowed, labelled "framework-owned layout". They skip the Architecture step, because the framework's tool defines the layout. Projemble still adds config, README and the verification gate, and they stay Experimental until the examples pass the same quality bar.
