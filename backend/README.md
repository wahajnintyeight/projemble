# Projemble Backend

Projemble is a local-first Go project initializer. Its terminal wizard collects a project name, description, parent directory, application shape, and architecture; it then generates a buildable starter project and saves its profile in a local YAML file. Generation is deterministic and works without an LLM or API key.

For optional AI-assisted work, `project agent` uses a provider-neutral agent and LLM adapter layer. It can inspect and edit a generated project, run fixed Go checks, and work with API-key providers or supported local endpoints.

## Supported profiles

The catalog combines two application shapes with three architecture choices:

| Shape | Architecture | Template ID |
| --- | --- | --- |
| Monolith | Layered | `go-monolith-layered` |
| Monolith | Clean / Hexagonal | `go-monolith-clean-hexagonal` |
| Monolith | Domain-Driven Design | `go-monolith-ddd` |
| Microservices (go-micro) | Layered | `go-microservices-layered` |
| Microservices (go-micro) | Clean / Hexagonal | `go-microservices-clean-hexagonal` |
| Microservices (go-micro) | Domain-Driven Design | `go-microservices-ddd` |

## Requirements

- Go 1.25 or later
- A terminal with interactive keyboard input

## Run the wizard

From this directory, run:

```sh
go run ./cmd/projemble
```

The wizard starts by asking how to generate the starter:

1. Choose **Local templates** for deterministic, offline generation, or **AI-assisted generation** to build on that starter with an LLM.
2. For AI generation, choose a provider, enter its API key (masked and saved to local YAML; the provider environment variable is used when no saved key exists), and enter a model ID. ChatGPT plan sign-in is available to eligible Plus or Pro accounts. Free ChatGPT accounts can use local generation or AI generation with their own OpenAI Platform API key (billed separately) or another provider. While browser sign-in is pending, press `Esc` or `b` in the terminal to cancel; closing the browser tab alone does not send a cancellation callback.
3. Enter the project name, description, and parent directory, then choose the application shape and architecture.
4. Review the profile and start generation. Projemble creates the selected starter, optionally runs the agent, and saves the profile. AI-assisted runs open a live agent workspace with the selected model, file/tool activity, provider-reported input/output token counts, and a multiline prompt for follow-up changes. Press Enter to send a prompt, Ctrl+J for a new line, PageUp/PageDown to review activity, `b` for project details, or `q` to exit.

The final project directory is created inside the selected parent directory using the project name. For example, entering `E:\Softwares\Programming` and the name `waypoint` creates `E:\Softwares\Programming\waypoint`.

Use Up/Down or `j`/`k` to select an option and Enter to continue. Projemble loads the last saved generation mode, provider, and model as the next run's defaults. On the review screen, `r` changes the provider, `g` changes generation mode, and `n`, `d`, `p`, `s`, and `a` edit the name, description, path, shape, and architecture. Press `b` or Backspace to go back, or `q` to quit.

Paths use the host operating system's format. The selected parent must already exist, and the destination project directory must not exist.

## Manage profiles from the command line

List saved profiles:

```sh
go run ./cmd/projemble project list
```

Register a profile directly:

```sh
go run ./cmd/projemble project add \
  --name waypoint \
  --description "A delivery coordination platform profile" \
  --path ./waypoint \
  --template go-microservices-clean-hexagonal \
  --setting service-framework=go-micro
```

The `project add` command records the supplied path and profile; it does not create or generate the directory. The interactive wizard creates the destination directory and source files when the profile is saved.

### Optional provider-backed agent

After creating a project in the wizard, run an agent task against that directory:

```sh
export OPENAI_API_KEY="your-key"
go run ./cmd/projemble project agent --provider openai --path "$HOME/waypoint" --model "your-model" --task "Review the starter and add shipment CRUD endpoints with tests."
```

Supported provider values are `openai`, `claude`, `deepseek`, `mistral`, `qwen`, `openrouter`, `huggingface`, `gemini`, and `openai-web`. Each API provider reads its key from the usual environment variable: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `DEEPSEEK_API_KEY`, `MISTRAL_API_KEY`, `DASHSCOPE_API_KEY`, `OPENROUTER_API_KEY`, `HF_TOKEN`, or `GEMINI_API_KEY`. Override the variable with `--api-key-env`; set `--base-url` to override a provider endpoint or use a local OpenAI-compatible server.

For ChatGPT-plan authentication, sign in once in the system browser, then use the `openai-web` provider:

```sh
go run ./cmd/projemble auth login --provider openai-web
go run ./cmd/projemble project agent --provider openai-web --path "$HOME/waypoint" --model "your-model" --task "Review the starter and suggest improvements."
```

The ChatGPT sign-in uses OpenAI's documented public-client loopback OAuth flow with PKCE and verified OIDC identity tokens. Tokens are stored under the operating system's Projemble user configuration directory with `0600` permissions on Unix systems and refreshed as needed. OpenAI's ChatGPT-plan inference route is a separate Responses API integration; it requires streamed requests with `store:false` and supports a narrower set of capabilities than API-key access. Plan inference is for eligible Plus or Pro accounts with the granted `chatgpt.tokens.use.direct` permission; a free ChatGPT account can use Projemble's local templates or configure a provider API key, which uses separate Platform billing. The workspace restriction shown during sign-in is independent of subscription status; a ChatGPT client registration is bound to the workspace selected during registration. See [OpenAI sign-in requirements](https://developers.openai.com/siwc/token-sharing-open-source/sign-in), [account and workspace behavior](https://developers.openai.com/siwc/token-sharing-open-source), and [current preview limitations](https://developers.openai.com/siwc/token-sharing-open-source/preview-limitations). Provider IDs and model IDs are saved with the project profile. API keys entered in the TUI are saved in `provider_keys` in `config.yaml`; environment variables are used when no YAML key is saved. The YAML file contains plaintext secrets and must be kept private.

The provider-neutral agent delegates through an LLM interface, with a factory selecting adapters for OpenAI-compatible APIs, Anthropic Messages, Gemini generateContent, and OpenAI Responses streaming. Its built-in engineering guidance covers repository inspection, minimal changes, safe file boundaries, tests, and truthful reporting. Tools are constrained to project-relative file listing/reading/writing and the fixed Go checks `test`, `build`, and `vet`; it cannot run arbitrary shell commands. Token counts come from provider responses and update as each model request completes. The workspace shows the latest request's input tokens and labels the model's maximum context size unavailable when the provider API does not report it.

## Agent workspace navigation

On wide terminals, the conversation and message input occupy the left side; a session rail on the right shows the provider, model, status, usage, and context information. Narrow terminals use a stacked layout.

- **F2 / Esc:** return to saved projects.
- **F3:** change the current project's provider.
- **F4:** choose another model for the current project.
- **Ctrl+O:** toggle compact activity and the full transcript.
- **PageUp / PageDown:** review rendered conversation lines.
- Type **@** in a message to browse and attach files from the current project; select a directory to open it and a file to insert its relative path. The message box wraps long drafts and grows while space allows, then scrolls to keep the cursor visible.

Switching views during an instruction cancels that instruction and waits for it to stop. Pending instructions are cleared. Provider/model changes reuse the saved conversation and update the project's YAML profile. The model picker retrieves the provider's catalog with a timeout; **Ctrl+R** refreshes it. A custom model ID is available when discovery fails or is unsupported, including the current ChatGPT integration. Listing a model does not guarantee account quota or tool support.

The model picker supports substring search: press `/`, type to filter, Enter to return to results, then use Up/Down and Enter to select. Escape clears an active query before leaving the picker. The workspace components live in `internal/tui/workspace_layout.go`, `workspace_panel.go`, `workspace_status.go`, `workspace_input.go`, `workspace_navigation.go`, `transcript.go`, and `model_picker.go`. Catalog retrieval belongs to `internal/llm/factory/models.go`.

### Recent enhancements

**Model catalog discovery** — The model picker now retrieves available models from the provider's catalog API with bounded pagination and response size limits. Press **Ctrl+R** to refresh the catalog. When discovery fails or is unsupported (including ChatGPT), you can enter a custom model ID.

**File mentions** — Type `@` in the message composer to reference project files. The autocomplete lists files and directories relative to the current path, filtered by substring match. Selections are bounded to 500 entries to keep the UI responsive.

**Enhanced message composer** — The input area now wraps long lines visually and scrolls vertically to keep the cursor visible. Multi-line drafts are easier to edit without losing your place.

**Adaptive workspace layout** — Wide terminals (≥110 columns) show a session rail on the right with provider, model, status, token usage, and context information. Narrow terminals use a stacked layout with status at the top.

**Session panel** — The right-side panel displays real-time agent status, session token counts (input/output/total), context window information, and workspace path. Token counts update as each model request completes.

## Saved configuration

Profiles are stored in `config.yaml` under the operating system's user configuration directory, in a `projemble` subdirectory. Typical locations are:

- Windows: `%AppData%\projemble\config.yaml`
- macOS: `~/Library/Application Support/projemble/config.yaml`
- Linux: `$XDG_CONFIG_HOME/projemble/config.yaml`, or `~/.config/projemble/config.yaml` when `XDG_CONFIG_HOME` is unset

The top-level `generation` settings store the selected generation mode, provider, and model. `parent_directory` and `last_project_id` remember the creation location and selected project. Older config files without these defaults use the most recently updated project profile. Provider API keys are stored in the `provider_keys` map in this file. They are plaintext; keep the config file private and do not commit or share it. The TUI and `project agent` command load these keys automatically when the matching environment variable is unset.

Example:

```yaml
version: 1
generation:
  mode: agent
  provider: claude
  model: claude-test-model
projects:
  - id: 48a85f7c52c54b02a3241860931120fc
    name: waypoint
    description: A delivery coordination platform profile
    path: E:\Softwares\Programming\waypoint
    stack: go
    app_shape: microservices
    architecture: clean-hexagonal
    template: go-microservices-clean-hexagonal
    settings:
      service-framework: go-micro
    created_at: "2026-10-06T12:00:00Z"
    updated_at: "2026-10-06T12:00:00Z"
```

Do not store API keys or other secrets in project settings.

## Verify

```sh
go test ./...
go build ./...
go vet ./...
```

### Reopening projects and conversations

Startup shows saved projects. Select a project and press Enter to reopen it; use `n` for a new project or `r` for provider settings. Reopening an AI project restores its conversation, recent activity, and recorded token usage. Missing credentials lead to provider setup without repeating project naming or directory selection.

Agent checkpoints are stored in the `sessions` subdirectory alongside the user config, one per workspace. Checkpoints are atomic and limited to 16 MiB. They contain conversation and tool content, but not the configured provider key. Old conversations created before checkpoint support cannot be recovered. Multiple named sessions, persisted drafts, and cross-process concurrent access are not implemented yet.

An AI failure preserves the starter files and an interrupted project profile. Reopen the project and give the agent an instruction to continue. No paid request is automatically restarted at launch. Linux systems without Secret Service can supply provider environment variables.

See [agent workspace research and proposed UX](docs/agent-workspace-research.md).
