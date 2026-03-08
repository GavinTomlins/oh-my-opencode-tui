# Bubble Tea TUI for Oh My Opencode Model Assignment

## Goals

- Build a standalone Go TUI using `charmbracelet/bubbletea`.
- Let users assign models to oh-my-opencode agents.
- Let users choose a default model for unassigned entries.
- Match the existing config conventions in `~/.config/opencode`.
- Support future agents/categories/providers without hardcoded assumptions.
- Support provider connection flows where credentials unlock selectable models.

## Ground Truth From Existing Config

### Active generated config

- `~/.config/opencode/oh-my-opencode.json` contains active `agents` and `categories` objects keyed by name.
- Each assignment stores a `model` in `provider/model` form and may include `variant` and `prompt`.

### Profile source of truth

- `~/.config/opencode/config/oh-my-opencode-profiles.json` contains:
  - `activeProfile`
  - `profiles.{name}.agents`
  - `profiles.{name}.categories`
  - `modelDefinitions`
- Profiles may inherit via `extends`.
- The existing `omc-config` script resolves inheritance and regenerates `~/.config/opencode/oh-my-opencode.json`.

### Provider source of truth

- `~/.config/opencode/opencode.json` contains `provider.{id}` entries.
- A provider entry includes `name`, `npm`, `options`, and `models`.
- `options` may include `apiKey` and `baseURL`.
- `models` is a keyed object of provider-local model IDs.
- This file currently contains comments, so the app must treat it as JSONC-compatible input rather than strict JSON.

## Delivery Shape

### Project scaffold

- Initialize a new Go module in the empty repo.
- Add a CLI entrypoint under `cmd/oh-my-opencode-tui`.
- Add internal packages for:
  - config path discovery
  - JSON read/write and backups
  - profile inheritance resolution
  - provider and model catalog management
  - Bubble Tea application state, screens, and components

### Config domain model

- Represent agents, categories, profiles, providers, and models as dynamic maps loaded from JSON.
- Build normalized view models for the UI so new keys appear automatically.
- Preserve passthrough fields like `variant` and `prompt` instead of dropping unknown data.

### TUI information architecture

- Use a keyboard-first multi-pane layout inspired by the screenshots:
  - left navigation: `Profiles`, `Agents`, `Categories`, `Providers`, `Defaults`, `Review`
  - main panel: searchable list and detail/editor content
  - footer: visible shortcuts and current save status
- Use a searchable provider picker and searchable model picker.
- Show provider sections and model lists similar to the screenshot flow.

## Functional Plan

### 1. Load and merge current state

- Read active config from `~/.config/opencode/oh-my-opencode.json`.
- Read profiles from `~/.config/opencode/config/oh-my-opencode-profiles.json`.
- Read provider definitions from `~/.config/opencode/opencode.json`.
- Discover agents/categories from the union of:
  - active config
  - active profile
  - all profiles
- Discover models from:
  - `modelDefinitions`
  - configured provider model maps in `opencode.json`
  - app-managed provider catalogs for newly added providers

### 2. Agent and category assignment

- Show every discovered agent and category, not a fixed built-in list.
- For each entry, show current model, source, and editable metadata.
- Let the user assign a model using the exact `provider/model` format expected by existing config.
- Preserve unedited fields when saving.

### 3. Default model support

- Add a default model selector for fallback use inside the TUI.
- Because upstream config does not currently define a global default field, store this in a companion file:
  - `~/.config/opencode/config/oh-my-opencode-tui.json`
- Include commands/actions to apply the default to unassigned agents/categories.
- Clearly label the default as app-managed so it does not invent unsupported upstream JSON keys.

### 4. Provider connection and model catalogs

- Seed a built-in provider registry for providers shown in the screenshots and current config, including:
  - `opencode`
  - `openai`
  - `anthropic`
  - `google`
  - `openrouter`
  - `ollama`
  - `lmstudio`
  - generic OpenAI-compatible/custom providers
- For each provider, define metadata such as:
  - display name
  - auth type
  - default base URL
  - whether local/no-key usage is allowed
  - model catalog source type
- Credential handling rules:
  - prefer environment-variable placeholders in `opencode.json` where that matches existing config style
  - allow explicit API-key entry in the TUI for app-managed providers when needed
  - mask all secrets in the UI and review screens
  - never log secret values
- First implementation will use bundled/static model catalogs plus optional refresh hooks.
- Local providers like Ollama and LM Studio can expose local models via configurable endpoints.
- Remote providers that do not support reliable live enumeration will still be usable via curated model lists after credential entry.

### 5. Persistence strategy

- Primary save target:
  - update the active profile entry in `~/.config/opencode/config/oh-my-opencode-profiles.json`
  - re-resolve inheritance and regenerate `~/.config/opencode/oh-my-opencode.json`
- When regenerating `~/.config/opencode/oh-my-opencode.json`, preserve unrelated top-level keys already present in the file, such as `terminal_multiplexer`, and only replace the managed `agents` and `categories` sections.
- Secondary save target:
  - update provider definitions in `~/.config/opencode/opencode.json` when the user connects or edits providers
- App companion target:
  - save TUI-only state such as default model, provider connection metadata, and UI preferences in `~/.config/opencode/config/oh-my-opencode-tui.json`
- Create backups before overwriting any touched config file.

### 6. Review before write

- Add a review screen with a human-readable diff summary of:
  - agent changes
  - category changes
  - provider additions/edits
  - default-model changes
- Save only after explicit confirmation.

## File Plan

- `go.mod`
- `cmd/oh-my-opencode-tui/main.go`
- `internal/app/...` for Bubble Tea root model and navigation
- `internal/ui/...` for shared components/styles
- `internal/config/...` for file paths, IO, backups, and JSON structs
- `internal/domain/...` for normalized entities and save logic
- `internal/providers/...` for built-in provider registry and model catalogs
- `internal/profile/...` for inheritance resolution matching `omc-config`
- `internal/diff/...` for review summaries
- `internal/*/*_test.go` for unit tests
- `README.md` with usage, keybindings, and config behavior

## Verification Plan

- Unit tests for:
  - config path resolution
  - JSON and JSONC parsing and round-tripping
  - inheritance resolution
  - dynamic discovery of agents/categories/providers/models
  - save and regeneration behavior
  - backup creation
- Fixture coverage proving unrelated top-level keys in `~/.config/opencode/oh-my-opencode.json` survive a save unchanged
- Run:
  - `go test ./...`
  - `go vet ./...`
  - `go build ./...`

## Non-Goals For First Pass

- Full live remote model enumeration for every provider.
- Provider OAuth/browser login flows.
- Editing unrelated OpenCode config sections outside agents/categories/providers/default state.

## Risks and Mitigations

- Profile inheritance mismatch: mirror `omc-config` resolution logic in tests using fixture coverage.
- Unknown provider/model fields: preserve existing JSON fields on read/write.
- Secret exposure: mask API keys in UI, avoid logging secrets, and only persist intended credential fields.
- Future expansion: drive lists from loaded JSON and provider registry instead of compile-time agent enums.
