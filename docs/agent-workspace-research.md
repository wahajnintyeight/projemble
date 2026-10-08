# Projemble: durable agent workspace

Research date: 6 October 2026. Sources are upstream documentation and source code. Recommendations below are engineering judgments; no comparative speed or usability benchmark was run.

## Recommendation

Build a project workspace with a session picker, conversation, file-change inspector, and always-available composer. Keep generation as one action inside that workspace. Prototype FluffyUI before committing to a framework migration. Bubble Tea v2 is the strongest alternative to evaluate for the agent interaction layer; tview is a practical alternative for conventional forms and panes.

The persistence repair can ship through our existing gotui renderer. Moving renderers alone would leave the original startup and save bugs intact.

## What FluffyUI offers

FluffyUI documents flex/grid layout, input widgets, Markdown views, trees, splitters, a command palette, and an FSS styling system. These cover much of the intended screen structure. Treat the README's widget counts and superiority comparison as the author's claims, rather than an independently established ranking. [Repository](https://github.com/odvcencio/FluffyUI)

Its runtime uses a widget tree, explicit measure/layout/render phases, commands, and reactive signals. The documentation explicitly warns that automatic computed dependency tracking is intended for the UI goroutine. For Projemble, provider workers should post events into the UI loop; they should never mutate widgets directly. [Architecture](https://github.com/odvcencio/FluffyUI/blob/main/docs/architecture.md)

The command palette source implements filtering, categories, shortcut labels, execution callbacks, and focus-aware keyboard handling. This maps naturally to Open project, Switch session, Change model, Provider settings, Show changes, and Run checks. A shortcut label does not itself bind a shortcut; application-level bindings still need implementation. [Command palette source](https://github.com/odvcencio/FluffyUI/blob/main/widgets/command_palette.go)

The terminal backend enables mouse and bracketed paste, buffers pasted input, and translates terminal events. This is directly relevant to our earlier input problems. It supplies building blocks, but we still need real terminal acceptance checks on Windows, macOS, and Linux. [Terminal backend](https://github.com/odvcencio/FluffyUI/blob/main/backend/tcell/tcell.go), [TextArea source](https://github.com/odvcencio/FluffyUI/blob/main/widgets/textarea.go)

Its agent server streams UI changes and exposes actions such as snapshots, key presses, text input, and resize. These are useful for external automation and end-to-end testing. Its sessions describe UI automation clients; we must implement our own coding conversations, provider authentication, tool execution, and recovery. [Agent server](https://github.com/odvcencio/FluffyUI/blob/main/docs/agent.md)

The testing documentation describes simulation backends, render captures, input injection, widget harnesses, and accessibility assertions. A prototype should exercise typing while tool events stream, multiline paste, resize, focus changes, reconnect, and cancellation. [Testing guide](https://github.com/odvcencio/FluffyUI/blob/main/docs/testing.md)

## Alternatives and agent references

| Option | Relevant strengths | Adoption decision |
| --- | --- | --- |
| FluffyUI | Composable widget tree, layouts, styling, command palette, automation tooling | Prototype the workspace and inspect platform behaviour before replacing gotui |
| Bubble Tea v2 + Bubbles + Lip Gloss | Explicit model/update/view loop, component ecosystem, terminal rendering and input facilities | Strong candidate for a clean rewrite of the UI layer if the prototype warrants migration |
| tview | Forms, tables, trees, layout primitives and conventional focus handling | Good choice for project administration; conversation presentation needs custom composition |
| Existing gotui | Already integrated; persistence fixes can ship immediately | Keep while validating the next renderer; manual input handling is accumulating maintenance cost |

Bubble Tea describes its event-driven model and supporting Bubbles/Lip Gloss libraries in its official documentation. tview documents its interactive widget set and concurrency requirements. Neither provides Projemble's storage or provider service automatically. [Bubble Tea](https://github.com/charmbracelet/bubbletea), [tview](https://github.com/rivo/tview)

Crush is the closest useful product reference: its documentation describes sessions per project, switching models while preserving context, multiple providers, LSP context, and MCP integrations. It also distinguishes global/project configuration from application state. Borrow the interaction concepts, and inspect licensing before copying any implementation. [Crush](https://github.com/charmbracelet/crush)

## Proposed interaction model

This is the target design, not a claim that every item is implemented today.

1. **Home:** recent projects, missing-directory warnings, remembered selection, New project, provider settings, and search.
2. **Workspace:** project/session sidebar; conversation in the centre; optional Changes/Checks inspector; composer anchored below the conversation.
3. **Header:** provider/model, ready/running/interrupted state, current operation, usage, and context information. Distinguish cumulative billed tokens from current context occupancy. Unknown model limits must remain labelled unknown.
4. **Conversation:** user messages and assistant replies; collapsible tool operations; explicit file paths; check output separate from prose. Show model-provided explanations or operation status when available. Do not manufacture private reasoning.
5. **Changes:** created/edited/deleted labels plus semantic colours; before/after diffs; long outputs collapsed. Colour must supplement readable labels.
6. **Composer:** typing and paste while busy, visible queued instructions, multiline editing, cancellation without losing a draft, and clear focus indication. Keep navigation shortcuts from consuming ordinary letters in focused input.
7. **Commands:** one searchable menu for project/session/model/provider changes and checks. A provider configuration screen should be reachable without creating a project.
8. **Recovery:** reopen the latest checkpoint; mark interrupted work; show a Resume action that accepts an instruction. Restarting the app should never silently restart a paid request.

At narrow widths, hide the sidebar and show it as a picker. At short heights, prioritise conversation and composer over status decoration. Test native terminal colours, low-colour terminals, Unicode widths, and pasted paths with spaces.

## Root causes in Projemble

Local inspection found four independent problems:

- `runWithInitializer` always started on the generation-mode page.
- The saved directory was absent from configuration and the location field was empty on each launch.
- `saveProjectWithProgress` saved YAML only after all AI work succeeded and removed the newly generated directory on AI failure.
- `Agent.messages` was process-local; there was no restart checkpoint.

The implemented repair adds a project home, remembered directory/selection, OS credential-store lookup, early profile saves, preserved interrupted scaffolds, and bounded conversation checkpoints. Existing YAML remains readable. Historical conversations from before this change cannot be reconstructed.

## Storage boundaries

| Data | Current repair | Later extension |
| --- | --- | --- |
| Project profiles/default provider/model/directory | Existing user YAML, optional added fields | Provider profiles, richer lifecycle transitions |
| API keys | Plaintext `provider_keys` map in user YAML; YAML value is preferred, environment variable is fallback | Encrypted secret storage if the user chooses it later |
| ChatGPT credentials | Existing authentication service | Surface expiration/reconnect status |
| Conversation/tool messages/usage/recent activity | Atomic JSON checkpoints per named session plus an active-session index in user config directory | Retention/export |
| Drafts/queued instructions/panel state | Process-local | Durable UI state, queue recovery requiring explicit resume |

The user requested API keys in YAML for simpler setup. This is implemented as plaintext under `provider_keys`. Keep that config private and out of source control. Environment variables are a fallback when no YAML value exists. Unix writes set the config file to owner read/write; Windows file permissions inherit the user config directory ACL.

Checkpoints are capped at 16 MiB, use same-directory temporary files and rename, and redact the configured API key. They contain conversation/file/tool content and therefore deserve the same privacy treatment as the project. Named sessions are indexed per workspace; concurrent writers are not supported, so only one active TUI process should write a given project session at a time.

## Framework prototype acceptance gate

Create an isolated screen prototype before migration. Demonstrate: editable composer during streamed activity; Unicode/multiline paste; model/provider dialog; expandable diff; project/session picker; width changes; cancel/reopen; and simulation-driven interaction tests. Verify native terminals on all three operating systems. Pin a reviewed version or commit and check upstream release/API stability rather than installing an unbounded latest dependency.

If FluffyUI passes, migrate the presentation layer in small screens while keeping agent/provider/storage services independent. Otherwise evaluate Bubble Tea v2 with the same acceptance cases. Web access and multiple users need a separate backend/session ownership design; exposing a terminal automation socket alone does not deliver those features.
