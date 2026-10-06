# Projemble

Projemble is a local-first Go project creator. The first slice is a TUI for selecting an application shape and an architecture profile.

## Initial catalogue

Application shape and architecture are separate choices:

- **Monolith**: one deployable Go application.
- **Microservices**: multiple Go services using [go-micro](https://github.com/micro/go-micro) for service communication and runtime conventions.

The three architecture profiles are:

- **Layered**: a pragmatic handler/service/repository separation for small teams and straightforward domains.
- **Clean / Hexagonal**: domain and use cases at the center, with infrastructure behind ports and adapters.
- **Domain-Driven Design**: organize around bounded contexts and explicit domain models when the business domain warrants it.

These profiles can be selected with either app shape. For microservices, the profile describes the internal structure of each service; go-micro describes how services communicate and run. DDD is an option for complex domains, not the default for every project.

## Requirements

- Go 1.25 or later
- A terminal that supports interactive keyboard input
- A YAML config library dependency, fetched by Go modules

## Run

```sh
go run ./cmd/projemble
```

The TUI guides you through these steps:

1. Enter the project name.
2. Add a short description.
3. Enter an existing absolute parent directory for the project.
4. Choose the application shape: monolith or microservices (go-micro).
5. Choose an architecture: Layered, Clean / Hexagonal, or Domain-Driven Design.
6. Review the project details and save the profile to YAML. Projemble creates the project directory inside the chosen parent directory.

Enter a path using the host operating system's absolute-path format (for example, `E:\Softwares\Programming` on Windows or `/home/wahaj/projects` on Linux). The parent directory must already exist. Use Up/Down or `j`/`k` to choose an option and Enter to continue. On the review screen, use `n`, `d`, `p`, `s`, or `a` to edit the project name, description, location, application shape, or architecture directly. Backspace or `b` goes back one step; `q` or Ctrl+C quits. Saving creates the project directory and records its profile in YAML; source file generation and AI provider setup are later slices.

## Project configuration

Projemble stores registered projects in `config.yaml` under the operating system's user config directory. On Windows this is normally `%AppData%\projemble\config.yaml`.

```sh
go run ./cmd/projemble project add --name inventory-api --description "An inventory API" --path ./inventory-api --template go-monolith-clean-hexagonal --setting database=postgres
go run ./cmd/projemble project list
```

The YAML records the project path, selected template, stack, app shape, architecture, and additional settings. Do not put API keys or other secrets in project settings; credentials will use a separate local secret store.

```yaml
version: 1
projects:
  - id: 48a85f7c52c54b02a3241860931120fc
    name: inventory-api
    description: An inventory API
    path: C:\Users\you\Projects\inventory-api
    stack: go
    app_shape: monolith
    architecture: clean-hexagonal
    template: go-monolith-clean-hexagonal
    settings:
      database: postgres
    created_at: "2026-10-06T12:00:00Z"
    updated_at: "2026-10-06T12:00:00Z"
```
