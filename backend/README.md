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
2. For AI generation, choose a provider, enter its API key (masked; a matching environment variable is prefilled when available), and enter a model ID. You can also choose ChatGPT plan sign-in, which opens the browser and needs no API key.
3. Enter the project name, description, and parent directory, then choose the application shape and architecture.
4. Review the profile and start generation. Projemble creates the selected starter, optionally runs the agent, and saves the profile.

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

The ChatGPT sign-in uses OpenAI's documented public-client loopback OAuth flow with PKCE and verified OIDC identity tokens. Tokens are stored under the operating system's Projemble user configuration directory with `0600` permissions on Unix systems and refreshed as needed. OpenAI's ChatGPT-plan inference route is a separate Responses API integration; it requires streamed requests with `store:false` and supports a narrower set of capabilities than API-key access. ChatGPT plan use depends on OpenAI account eligibility and granted `chatgpt.tokens.use.direct` permission; an API key uses separate Platform billing. See [OpenAI sign-in requirements](https://developers.openai.com/siwc/token-sharing-open-source/sign-in) and [current preview limitations](https://developers.openai.com/siwc/token-sharing-open-source/preview-limitations). Provider IDs and model IDs are saved with the project profile. API keys are used only for the current generation and are never saved in project YAML.

The provider-neutral agent delegates through an LLM interface, with a factory selecting adapters for OpenAI-compatible APIs, Anthropic Messages, Gemini generateContent, and OpenAI Responses streaming. Its built-in engineering guidance covers repository inspection, minimal changes, safe file boundaries, tests, and truthful reporting. Tools are constrained to project-relative file listing/reading/writing and the fixed Go checks `test`, `build`, and `vet`; it cannot run arbitrary shell commands.

## Configuration

Profiles are stored in `config.yaml` under the operating system's user configuration directory, in a `projemble` subdirectory. Typical locations are:

- Windows: `%AppData%\projemble\config.yaml`
- macOS: `~/Library/Application Support/projemble/config.yaml`
- Linux: `$XDG_CONFIG_HOME/projemble/config.yaml`, or `~/.config/projemble/config.yaml` when `XDG_CONFIG_HOME` is unset

The top-level `generation` settings store the last successful generation mode, provider, and model. Older config files without these defaults use the most recently updated project profile. Provider API keys are never written to this file; they are read from the provider's environment variable or entered for that run.

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
