# Projemble Backend

Projemble is a local-first Go project profile manager. Its terminal wizard collects a project name, description, parent directory, application shape, and architecture, then creates the project directory and saves the profile in a local YAML file.

> [!NOTE]
> The current version creates the directory and registers its profile. It does not generate application source files yet.

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

The wizard asks for:

1. Project name and short description
2. An existing absolute parent directory
3. Application shape and architecture
4. Review and confirmation

The final project directory is created inside the selected parent directory using the project name. For example, entering `E:\Softwares\Programming` and the name `waypoint` creates `E:\Softwares\Programming\waypoint`.

Use Up/Down or `j`/`k` to select an option and Enter to continue. On the review screen, `n`, `d`, `p`, `s`, and `a` edit the name, description, path, shape, and architecture. Press `b` or Backspace to go back, or `q` to quit.

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

The `project add` command records the supplied path and profile; it does not create the directory. The interactive wizard creates the destination directory when the profile is saved.

## Configuration

Profiles are stored in `config.yaml` under the operating system's user configuration directory, in a `projemble` subdirectory. Typical locations are:

- Windows: `%AppData%\projemble\config.yaml`
- macOS: `~/Library/Application Support/projemble/config.yaml`
- Linux: `$XDG_CONFIG_HOME/projemble/config.yaml`, or `~/.config/projemble/config.yaml` when `XDG_CONFIG_HOME` is unset

Example:

```yaml
version: 1
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
