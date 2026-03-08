# oh-my-opencode-tui

A Bubble Tea TUI for managing `oh-my-opencode` model assignments.

## What it does

- Loads agent and category assignments from `~/.config/opencode/oh-my-opencode.json`
- Uses `~/.config/opencode/config/oh-my-opencode-profiles.json` as the editable profile source of truth
- Loads providers from `~/.config/opencode/opencode.json` (JSONC supported)
- Lets you set an app-managed default model in `~/.config/opencode/config/oh-my-opencode-tui.json`
- Writes backups before saving touched config files

## Run

```bash
go run ./cmd/oh-my-opencode-tui
```

## Navigation

### Global Keys
- `↑`/`↓` or `j`/`k` - Navigate up/down in lists
- `Tab`/`Shift+Tab` - Switch between sections (Profiles, Agents, Categories, Providers, Defaults, Review)
- `/` - Focus search box
- `u` - Undo last change
- `Ctrl+S` - Save changes
- `q` - Quit

### List Navigation
- `Enter` - Enter detail/config mode for the selected item
- `Shift+Enter` - Go back from detail mode to list mode

### In Detail Mode (Agents/Categories)
- `Enter` - Open model picker to assign a model
- `x` - Clear the current assignment
- `Shift+Enter` - Go back to list

### Model Picker
- `↑`/`↓` - Navigate through models
- `Enter` - Select the highlighted model
- `Esc` - Cancel and close picker
- Type to filter models

### Providers
- `a` - Add/connect a new provider (from catalog)
- `Enter` - Edit the selected provider
- `x` - Remove a configured provider

### Defaults
- `Enter` - Select default model
- `d` - Apply default model to all unassigned agents/categories
- `x` - Clear default model

## UI Layout

The UI has:
1. **Title bar** - Shows "Oh My Opencode TUI" with version
2. **Left sidebar** - Navigation between sections
3. **Main content** - List on left, details on right
4. **Status bar** - Shows config file paths with last modified timestamps
5. **Footer** - Available commands with reverse highlighting

Selected items are highlighted with a cyan background and black text (reverse highlighting) for clear visibility.
