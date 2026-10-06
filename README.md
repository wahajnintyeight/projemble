# Projemble

**Pick a shape. Get a scaffold. Add an agent when it helps.**

Projemble is a Go CLI/TUI for creating projects from explicit application shapes, software architectures, and templates. It makes the project structure clear up front. An optional coding agent can then build features on that foundation.

[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![Latest release](https://img.shields.io/github/v/release/wahajnintyeight/projemble)](https://github.com/wahajnintyeight/projemble/releases/latest)
[![TUI: gotui](https://img.shields.io/badge/TUI-gotui-5B8DEF)](https://github.com/metaspartan/gotui)
[![Markdown: Goldmark](https://img.shields.io/badge/Markdown-Goldmark-555555)](https://github.com/yuin/goldmark)
[![GitHub stars](https://img.shields.io/github/stars/wahajnintyeight/projemble?style=flat)](https://github.com/wahajnintyeight/projemble/stargazers)

## Install

Download the binary for your OS and CPU from [Releases](https://github.com/wahajnintyeight/projemble/releases/latest): Windows, macOS, and Linux on amd64 or arm64. Put it on your `PATH`, or run it from its download folder. Releases include `SHA256SUMS.txt` to check downloads.

Or build from source with Go **1.25 or later**:

```sh
git clone https://github.com/wahajnintyeight/projemble.git
cd projemble
go build -o projemble ./cmd/projemble
```

On Windows PowerShell, build and run the `.exe`:

```powershell
go build -o projemble.exe ./cmd/projemble
.\projemble.exe
```

On macOS or Linux:

```sh
./projemble
```

To start without building a binary, run `go run ./cmd/projemble` from the repository directory.

## Get started

1. Select **New project** and describe what you are building.
2. Choose an existing parent directory for the project.
3. Choose a **project shape** and architecture. Projemble shows a preview of the resulting blueprint.
4. Choose **Local templates** for deterministic scaffolding with no account or API key, or **AI-assisted generation** to let a configured agent extend the scaffold.
5. Review the project profile and create it.

The project directory is created under the parent directory you chose. The TUI remembers your provider, model, and project location. Use `Up`/`Down` or `j`/`k` to move through choices, `Enter` to select, and `Esc` or `b` to go back.

## Shapes and templates

The template choice follows the project shape and architecture; the architecture menu only shows options that fit.

| Project shape | Architectures | Typical use |
| --- | --- | --- |
| Go monolith | Layered, Clean / Hexagonal, Domain-Driven Design | One deployable API or service |
| Go microservices (`go-micro`) | Layered, Clean / Hexagonal, Domain-Driven Design | Services that communicate with each other |
| Go one-shot job | Pipeline | Scraping, ETL, imports, and maintenance jobs that run and exit |

## Optional agent

AI is optional. Local templates need no provider setup. For agent-assisted generation, select a provider in the TUI and enter its key, or set the provider environment variable: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `DEEPSEEK_API_KEY`, `MISTRAL_API_KEY`, `DASHSCOPE_API_KEY`, `OPENROUTER_API_KEY`, `HF_TOKEN`, or `GEMINI_API_KEY`. The TUI also supports ChatGPT sign-in for eligible accounts. Provider keys saved by the TUI are stored as plaintext in the local user config; keep that file private.

To run an agent task from the command line, set a provider key and provide a project path, model, and task. For example, in macOS/Linux:

```sh
export OPENAI_API_KEY="your-key"
go run ./cmd/projemble project agent \
  --provider openai \
  --path "$HOME/my-service" \
  --model "your-model" \
  --task "Review the project structure and add health checks."
```

Supported providers: OpenAI, Claude, DeepSeek, Mistral, Qwen, OpenRouter, Hugging Face, Gemini, and ChatGPT sign-in (`openai-web`). The agent can inspect and edit project files and run fixed Go checks (`go test`, `go vet`, and `go build`).

In the agent workspace, press `Enter` to send, `Ctrl+J` for a new line, `Ctrl+B` to show or hide the session panel, `Ctrl+L` to redraw a broken or misaligned view, `F3` to change provider, `F4` to change model, and `PageUp`/`PageDown` to review the conversation. The `Ctrl+L` refresh is also available during onboarding.

## CLI commands

```sh
# List saved project profiles
go run ./cmd/projemble project list
```

`project add` registers a profile; it does not create the project directory. The TUI creates the directory and starter files.

## Configuration

Project profiles and provider defaults live in `config.yaml` under Projemble’s operating system user-config directory: `%AppData%\projemble` on Windows, `~/Library/Application Support/projemble` on macOS, and `$XDG_CONFIG_HOME/projemble` or `~/.config/projemble` on Linux. The TUI reads saved keys from this file and falls back to provider environment variables. Keys are plaintext; keep the file private.

## Credits

- [gotui](https://github.com/metaspartan/gotui) — terminal UI; included as a local fork under [`third_party/gotui`](third_party/gotui) and distributed under its MIT license.
- [Goldmark](https://github.com/yuin/goldmark) — Markdown parsing and rendering.
- [go-yaml](https://github.com/go-yaml/yaml) — YAML configuration.
- [go-oidc](https://github.com/coreos/go-oidc) — OpenID Connect verification for browser sign-in.
- [go-runewidth](https://github.com/mattn/go-runewidth) — terminal display-width handling.

The full dependency list is in [`go.mod`](go.mod).

## Development

```sh
go test ./...
go build ./...
go vet ./...
```
