package app

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gavintomlins/oh-my-opencode-tui/internal/config"
	providercatalog "github.com/gavintomlins/oh-my-opencode-tui/internal/providers"
)

const appVersionBase = "v0.1.4"

func getAppVersion(changeCount int) string {
	if changeCount == 0 {
		return appVersionBase
	}
	return fmt.Sprintf("%s+%d", appVersionBase, changeCount)
}

type Section int

const (
	SectionProfiles Section = iota
	SectionAgents
	SectionCategories
	SectionProviders
	SectionDefaults
	SectionReview
	SectionHelp
	SectionSkills
)

type viewMode int

const (
	modeNormal viewMode = iota
	modeModelPicker
	modeProviderCatalog
	modeProviderEditor
	modeProfileCreator
	modeBulkAssign
	modeSwapProvider
)

type pickerKind int

const (
	pickerAgent pickerKind = iota
	pickerCategory
	pickerDefault
)

type pickerTarget struct {
	kind pickerKind
	key  string
}

type modelOption struct {
	ID           string
	Name         string
	ProviderID   string
	ProviderName string
	Source       string
}

type providerItem struct {
	ID         string
	Name       string
	Connected  bool
	Template   providercatalog.Template
	Provider   config.Provider
	ModelCount int
	Custom     bool
}

type providerForm struct {
	template providercatalog.Template
	existing bool
	inputs   []textinput.Model
	focus    int
}

type profileForm struct {
	inputs []textinput.Model
	focus  int
}

type bulkAssignState struct {
	selectedModel string
	targets       []string
	targetType    string // "agents" or "categories"
	targetProfile string // profile key to assign to (empty = active profile)
}

type swapProviderState struct {
	fromProvider  string
	toProvider    string
	preview       map[string]string            // old model -> new model mappings
	oldAgents     map[string]config.Assignment // for undo
	oldCategories map[string]config.Assignment // for undo
	oldDefault    string                       // for undo
}

type edit struct {
	section  Section
	key      string
	oldValue string
	newValue string
	editType string
	// Bulk operation fields for undo
	oldAgents     map[string]config.Assignment
	oldCategories map[string]config.Assignment
	oldDefault    string
}

type undoStack struct {
	edits []edit
}

func (u *undoStack) push(e edit) {
	u.edits = append(u.edits, e)
	if len(u.edits) > 50 {
		u.edits = u.edits[1:]
	}
}

func (u *undoStack) pop() (edit, bool) {
	if len(u.edits) == 0 {
		return edit{}, false
	}
	e := u.edits[len(u.edits)-1]
	u.edits = u.edits[:len(u.edits)-1]
	return e, true
}

func (u *undoStack) canUndo() bool {
	return len(u.edits) > 0
}

func (u *undoStack) clear() {
	u.edits = nil
}

func (u *undoStack) changeCount() int {
	return len(u.edits)
}

type viewState int

const (
	stateList viewState = iota
	stateDetail
)

type fileInfo struct {
	path    string
	modTime time.Time
	exists  bool
}

type Model struct {
	paths    config.Paths
	original config.Snapshot
	snapshot config.Snapshot

	activeProfile string
	agents        map[string]config.Assignment
	categories    map[string]config.Assignment
	providers     map[string]config.Provider
	defaultModel  string

	sections         []Section
	sectionIndex     int
	selection        map[Section]int
	viewState        viewState
	width            int
	height           int
	mode             viewMode
	status           string
	err              error
	search           textinput.Model
	searchFocused    bool
	pickerSelection  int
	pickerTarget     pickerTarget
	catalogSelection int
	providerForm     providerForm
	undo             undoStack
	helpSelection    int
	profileForm      profileForm
	bulkAssign       bulkAssignState
	swapProvider     swapProviderState

	builtinTemplates []providercatalog.Template
	builtinMap       map[string]providercatalog.Template
	fileInfos        map[string]fileInfo
}

func New() (Model, error) {
	paths, err := config.DiscoverPaths()
	if err != nil {
		return Model{}, err
	}
	snapshot, err := config.Load(paths)
	if err != nil {
		return Model{}, err
	}
	search := textinput.New()
	search.Placeholder = "Search"
	search.Prompt = ""
	search.CharLimit = 256
	search.Width = 32

	builtins := providercatalog.Builtins()
	builtinMap := providercatalog.BuiltinMap()

	m := Model{
		paths:            paths,
		original:         snapshot,
		snapshot:         snapshot,
		sections:         []Section{SectionProfiles, SectionAgents, SectionCategories, SectionProviders, SectionDefaults, SectionReview, SectionHelp, SectionSkills},
		sectionIndex:     0,
		selection:        map[Section]int{},
		viewState:        stateList,
		search:           search,
		builtinTemplates: builtins,
		builtinMap:       builtinMap,
		providers:        cloneProviders(snapshot.Providers.Provider),
		defaultModel:     snapshot.UIState.DefaultModel,
		undo:             undoStack{},
		fileInfos:        make(map[string]fileInfo),
	}

	m.updateFileInfos()

	activeProfile := snapshot.Profiles.ActiveProfile
	if activeProfile == "" {
		activeProfile = "default"
	}
	if err := m.loadProfile(activeProfile); err != nil {
		return Model{}, err
	}
	if m.defaultModel == "" {
		m.defaultModel = m.firstAvailableModel()
	}
	m.status = "Loaded configuration"
	return m, nil
}

func (m *Model) updateFileInfos() {
	files := map[string]string{
		"opencode": m.paths.Providers,
		"profiles": m.paths.Profiles,
		"active":   m.paths.ActiveConfig,
		"ui":       m.paths.UIState,
	}

	for name, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			m.fileInfos[name] = fileInfo{path: path, exists: false}
		} else {
			m.fileInfos[name] = fileInfo{path: path, modTime: info.ModTime(), exists: true}
		}
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if keyMatches(msg, "ctrl+c") {
			return m, tea.Quit
		}
	}

	switch m.mode {
	case modeModelPicker:
		return m.updateModelPicker(msg)
	case modeProviderCatalog:
		return m.updateProviderCatalog(msg)
	case modeProviderEditor:
		return m.updateProviderEditor(msg)
	case modeProfileCreator:
		return m.updateProfileCreator(msg)
	case modeBulkAssign:
		return m.updateBulkAssign(msg)
	case modeSwapProvider:
		return m.updateSwapProvider(msg)
	}

	if m.searchFocused {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.searchFocused = false
				m.search.Blur()
				return m, nil
			case "enter":
				m.searchFocused = false
				m.search.Blur()
				return m.activateSelection()
			}
		}
		m.search, cmd = m.search.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "tab":
			if m.viewState == stateList {
				m.sectionIndex = (m.sectionIndex + 1) % len(m.sections)
				m.selection[m.currentSection()] = 0
			}
			return m, nil
		case "shift+tab":
			if m.viewState == stateList {
				m.sectionIndex--
				if m.sectionIndex < 0 {
					m.sectionIndex = len(m.sections) - 1
				}
				m.selection[m.currentSection()] = 0
			}
			return m, nil
		case "down", "j":
			if m.currentSection() == SectionHelp {
				// Navigate help links
				if m.helpSelection < 4 {
					m.helpSelection++
				}
			} else if m.viewState == stateList {
				m.moveSelection(1)
			}
			return m, nil
		case "up", "k":
			if m.currentSection() == SectionHelp {
				// Navigate help links
				if m.helpSelection > 0 {
					m.helpSelection--
				}
			} else if m.viewState == stateList {
				m.moveSelection(-1)
			}
			return m, nil
		case "enter":
			if m.viewState == stateList {
				return m.enterMode()
			} else {
				return m.activateSelection()
			}
		case "shift+enter":
			if m.viewState == stateDetail {
				m.viewState = stateList
				m.status = "Returned to list view"
			}
			return m, nil
		case "esc":
			if m.viewState == stateDetail {
				m.viewState = stateList
				m.status = "Returned to list view"
				return m, nil
			}
		case "/":
			m.searchFocused = true
			m.search.Focus()
			return m, textinput.Blink
		case "ctrl+s":
			m.undo.clear()
			return m.save()
		case "a":
			if m.currentSection() == SectionProviders && m.viewState == stateList {
				m.mode = modeProviderCatalog
				m.catalogSelection = 0
				m.resetSearch("Search providers")
				m.searchFocused = true
				m.search.Focus()
				return m, textinput.Blink
			}
		case "n":
			if m.currentSection() == SectionProfiles && m.viewState == stateList {
				m.mode = modeProfileCreator
				m.profileForm = newProfileForm()
				return m, textinput.Blink
			}
		case "b":
			if m.viewState == stateList {
				switch m.currentSection() {
				case SectionProfiles:
					keys := m.filteredProfiles()
					profileKey := ""
					if len(keys) > 0 && m.currentSelection() < len(keys) {
						profileKey = keys[m.currentSelection()]
					}
					m.mode = modeBulkAssign
					m.bulkAssign = bulkAssignState{targetType: "agents", targetProfile: profileKey}
					m.resetSearch("Select model to assign to all agents")
					m.searchFocused = true
					m.search.Focus()
					return m, textinput.Blink
				case SectionAgents:
					m.mode = modeBulkAssign
					m.bulkAssign = bulkAssignState{targetType: "agents", targetProfile: ""}
					m.resetSearch("Select model to assign to all agents")
					m.searchFocused = true
					m.search.Focus()
					return m, textinput.Blink
				}
			}
		case "c":
			if m.viewState == stateList {
				switch m.currentSection() {
				case SectionProfiles:
					keys := m.filteredProfiles()
					profileKey := ""
					if len(keys) > 0 && m.currentSelection() < len(keys) {
						profileKey = keys[m.currentSelection()]
					}
					m.mode = modeBulkAssign
					m.bulkAssign = bulkAssignState{targetType: "categories", targetProfile: profileKey}
					m.resetSearch("Select model to assign to all categories")
					m.searchFocused = true
					m.search.Focus()
					return m, textinput.Blink
				case SectionCategories:
					m.mode = modeBulkAssign
					m.bulkAssign = bulkAssignState{targetType: "categories", targetProfile: ""}
					m.resetSearch("Select model to assign to all categories")
					m.searchFocused = true
					m.search.Focus()
					return m, textinput.Blink
				}
			}
		case "s":
			if m.currentSection() == SectionProfiles && m.viewState == stateList {
				m.mode = modeSwapProvider
				m.swapProvider = swapProviderState{}
				m.resetSearch("Select source provider to replace")
				m.searchFocused = true
				m.search.Focus()
				return m, textinput.Blink
			}
		case "x":
			m.pushUndoForClear()
			return m.clearCurrent()
		case "d":
			if m.currentSection() == SectionDefaults {
				return m.applyDefault()
			}
		case "u":
			return m.undoLast()
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading... (width=" + fmt.Sprintf("%d", m.width) + " height=" + fmt.Sprintf("%d", m.height) + ")"
	}

	if m.mode == modeModelPicker {
		return m.viewModelPicker()
	}
	if m.mode == modeProviderCatalog {
		return m.viewProviderCatalog()
	}
	if m.mode == modeProviderEditor {
		return m.viewProviderEditor()
	}
	if m.mode == modeProfileCreator {
		return m.viewProfileCreator()
	}
	if m.mode == modeBulkAssign {
		return m.viewBulkAssign()
	}
	if m.mode == modeSwapProvider {
		return m.viewSwapProvider()
	}

	var output strings.Builder
	output.WriteString("Oh My Opencode TUI " + getAppVersion(m.undo.changeCount()) + "\n")
	output.WriteString("===========================\n\n")

	for i, section := range m.sections {
		name := sectionTitle(section)
		if i == m.sectionIndex {
			output.WriteString("> " + name + "\n")
		} else {
			output.WriteString("  " + name + "\n")
		}
	}

	output.WriteString("\n--- " + sectionTitle(m.currentSection()) + " ---\n")

	switch m.currentSection() {
	case SectionProfiles:
		output.WriteString(m.viewProfilesPlain())
	case SectionAgents:
		output.WriteString(m.viewAgentsPlain())
	case SectionCategories:
		output.WriteString(m.viewCategoriesPlain())
	}

	return output.String()
}

func (m Model) viewProfilesPlain() string {
	var output strings.Builder
	keys := m.filteredProfiles()

	if len(keys) == 0 {
		output.WriteString("No profiles\n")
		return output.String()
	}

	for i, key := range keys {
		profile := m.snapshot.Profiles.Profiles[key]
		marker := "  "
		if i == m.currentSelection() {
			marker = "> "
		}

		name := key
		if key == m.activeProfile {
			name += " [active]"
		}

		agentCount := len(profile.Agents)
		categoryCount := len(profile.Categories)
		if key == m.activeProfile {
			agentCount = len(m.agents)
			categoryCount = len(m.categories)
		}

		output.WriteString(fmt.Sprintf("%s%s\n", marker, name))
		output.WriteString(fmt.Sprintf("   %s\n", profile.Description))
		output.WriteString(fmt.Sprintf("   Agents: %d, Categories: %d\n\n", agentCount, categoryCount))
	}

	return output.String()
}

func (m Model) viewAgentsPlain() string {
	var output strings.Builder
	keys := m.filteredAgents()

	if len(keys) == 0 {
		output.WriteString("No agents\n")
		return output.String()
	}

	for i, key := range keys {
		assignment := m.agents[key]
		marker := "  "
		if i == m.currentSelection() {
			marker = "> "
		}

		model := assignment.Model
		if model == "" {
			model = "(unassigned)"
		}

		output.WriteString(fmt.Sprintf("%s%s\n", marker, key))
		output.WriteString(fmt.Sprintf("   Model: %s\n\n", model))
	}

	return output.String()
}

func (m Model) viewCategoriesPlain() string {
	var output strings.Builder
	keys := m.filteredCategories()

	if len(keys) == 0 {
		output.WriteString("No categories\n")
		return output.String()
	}

	for i, key := range keys {
		assignment := m.categories[key]
		marker := "  "
		if i == m.currentSelection() {
			marker = "> "
		}

		model := assignment.Model
		if model == "" {
			model = "(unassigned)"
		}

		output.WriteString(fmt.Sprintf("%s%s\n", marker, key))
		output.WriteString(fmt.Sprintf("   Model: %s\n\n", model))
	}

	return output.String()
}

func (m Model) viewTitleBar() string {
	changeCount := m.undo.changeCount()
	titleText := fmt.Sprintf("Oh My Opencode TUI %s", getAppVersion(changeCount))
	return titleText
}

func (m Model) viewStatusBar() string {
	lines := []string{
		fmt.Sprintf("Config Files (last modified):"),
	}

	for _, name := range []string{"opencode", "profiles", "active"} {
		info, ok := m.fileInfos[name]
		if !ok {
			continue
		}
		shortPath := strings.Replace(info.path, os.Getenv("HOME"), "~", 1)
		if info.exists {
			lines = append(lines, fmt.Sprintf("  %s: %s (%s)", name, shortPath, formatTime(info.modTime)))
		} else {
			lines = append(lines, fmt.Sprintf("  %s: %s (not found)", name, shortPath))
		}
	}

	// Add version to last line
	changeCount := m.undo.changeCount()
	versionStr := fmt.Sprintf("  Version: %s", getAppVersion(changeCount))
	lines = append(lines, versionStr)

	content := strings.Join(lines, "\n")
	return statusBarStyle.Width(m.width).Render(content)
}

func formatTime(t time.Time) string {
	return t.Format("Jan 02 15:04")
}

func (m Model) viewFooter() string {
	// Show status message if present
	if m.status != "" {
		statusLine := mutedStyle.Render(m.status)
		return footerBarStyle.Width(m.width).Render(statusLine)
	}

	commands := []string{
		cmdStyle.Render(" ↑↓ ") + "navigate ",
		cmdStyle.Render(" enter ") + "select ",
		cmdStyle.Render(" esc ") + "back ",
		cmdStyle.Render(" / ") + "search ",
		cmdStyle.Render(" ctrl+s ") + "save ",
		cmdStyle.Render(" q ") + "quit ",
	}

	if m.undo.canUndo() {
		commands = append([]string{cmdStyle.Render(" u ") + "undo "}, commands...)
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, commands...)
	return footerBarStyle.Width(m.width).Render(content)
}

func (m *Model) updateModelPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	options := m.filteredModelOptions()
	m.pickerSelection = clamp(m.pickerSelection, len(options))

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "shift+enter":
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			return *m, nil
		case "down", "j":
			m.pickerSelection = moveIndex(m.pickerSelection, len(options), 1)
			return *m, nil
		case "up", "k":
			m.pickerSelection = moveIndex(m.pickerSelection, len(options), -1)
			return *m, nil
		case "enter":
			if len(options) == 0 {
				return *m, nil
			}
			selected := options[m.pickerSelection]
			switch m.pickerTarget.kind {
			case pickerAgent:
				oldModel := m.agents[m.pickerTarget.key].Model
				assignment := m.agents[m.pickerTarget.key]
				assignment.Model = selected.ID
				m.agents[m.pickerTarget.key] = assignment
				m.undo.push(edit{section: SectionAgents, key: m.pickerTarget.key, oldValue: oldModel, newValue: selected.ID, editType: "assign"})
				m.status = fmt.Sprintf("Assigned %s to agent %s", selected.ID, m.pickerTarget.key)
			case pickerCategory:
				oldModel := m.categories[m.pickerTarget.key].Model
				assignment := m.categories[m.pickerTarget.key]
				assignment.Model = selected.ID
				m.categories[m.pickerTarget.key] = assignment
				m.undo.push(edit{section: SectionCategories, key: m.pickerTarget.key, oldValue: oldModel, newValue: selected.ID, editType: "assign"})
				m.status = fmt.Sprintf("Assigned %s to category %s", selected.ID, m.pickerTarget.key)
			case pickerDefault:
				oldDefault := m.defaultModel
				m.defaultModel = selected.ID
				m.undo.push(edit{section: SectionDefaults, key: "default", oldValue: oldDefault, newValue: selected.ID, editType: "default"})
				m.status = fmt.Sprintf("Default model set to %s", selected.ID)
			}
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			return *m, nil
		}
	}

	return *m, cmd
}

func (m *Model) updateProviderCatalog(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	items := m.filteredProviderCatalog()
	m.catalogSelection = clamp(m.catalogSelection, len(items))

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "shift+enter":
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			return *m, nil
		case "down", "j":
			m.catalogSelection = moveIndex(m.catalogSelection, len(items), 1)
			return *m, nil
		case "up", "k":
			m.catalogSelection = moveIndex(m.catalogSelection, len(items), -1)
			return *m, nil
		case "enter":
			if len(items) == 0 {
				return *m, nil
			}
			m.providerForm = newProviderForm(items[m.catalogSelection], config.Provider{})
			m.mode = modeProviderEditor
			return *m, textinput.Blink
		}
	}

	return *m, cmd
}

func (m *Model) updateProviderEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "shift+enter":
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			m.status = "Cancelled provider edit"
			return *m, nil
		case "tab":
			m.providerForm.focus = (m.providerForm.focus + 1) % len(m.providerForm.inputs)
			m.focusProviderForm()
			return *m, nil
		case "shift+tab":
			m.providerForm.focus--
			if m.providerForm.focus < 0 {
				m.providerForm.focus = len(m.providerForm.inputs) - 1
			}
			m.focusProviderForm()
			return *m, nil
		case "ctrl+s", "enter":
			providerID := strings.TrimSpace(m.providerForm.inputs[0].Value())
			if providerID == "" {
				m.status = "Provider ID is required"
				return *m, nil
			}
			providerName := strings.TrimSpace(m.providerForm.inputs[1].Value())
			baseURL := strings.TrimSpace(m.providerForm.inputs[2].Value())
			apiKey := strings.TrimSpace(m.providerForm.inputs[3].Value())
			envVar := strings.TrimSpace(m.providerForm.inputs[4].Value())

			provider := providercatalog.ProviderFromTemplate(m.providerForm.template)
			if existing, ok := m.providers[providerID]; ok {
				provider = existing.Clone()
			}
			if provider.Options == nil {
				provider.Options = map[string]string{}
			}
			if provider.Models == nil {
				provider.Models = map[string]config.ProviderModel{}
			}
			if providerName != "" {
				provider.Name = providerName
			}
			if m.providerForm.template.NPM != "" {
				provider.NPM = m.providerForm.template.NPM
			}
			if baseURL != "" {
				provider.Options["baseURL"] = baseURL
			} else {
				delete(provider.Options, "baseURL")
			}
			switch {
			case envVar != "":
				provider.Options["apiKey"] = fmt.Sprintf("${%s}", envVar)
			case apiKey != "":
				provider.Options["apiKey"] = apiKey
			case m.providerForm.template.Local:
				provider.Options["apiKey"] = "local"
			default:
				delete(provider.Options, "apiKey")
			}
			m.providers[providerID] = provider
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			m.status = fmt.Sprintf("Provider %s saved", providerID)
			return *m, nil
		}
	}

	for i := range m.providerForm.inputs {
		if i == m.providerForm.focus {
			m.providerForm.inputs[i], _ = m.providerForm.inputs[i].Update(msg)
		} else {
			m.providerForm.inputs[i].Blur()
		}
		cmds = append(cmds, textinput.Blink)
	}
	return *m, tea.Batch(cmds...)
}

func (m *Model) updateProfileCreator(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "shift+enter":
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			m.status = "Cancelled profile creation"
			return *m, nil
		case "tab":
			m.profileForm.focus = (m.profileForm.focus + 1) % len(m.profileForm.inputs)
			m.focusProfileForm()
			return *m, nil
		case "shift+tab":
			m.profileForm.focus--
			if m.profileForm.focus < 0 {
				m.profileForm.focus = len(m.profileForm.inputs) - 1
			}
			m.focusProfileForm()
			return *m, nil
		case "ctrl+s", "enter":
			profileKey := strings.TrimSpace(m.profileForm.inputs[0].Value())
			if profileKey == "" {
				m.status = "Profile key is required"
				return *m, nil
			}
			if _, exists := m.snapshot.Profiles.Profiles[profileKey]; exists {
				m.status = fmt.Sprintf("Profile '%s' already exists", profileKey)
				return *m, nil
			}
			displayName := strings.TrimSpace(m.profileForm.inputs[1].Value())
			description := strings.TrimSpace(m.profileForm.inputs[2].Value())
			if displayName == "" {
				displayName = profileKey
			}
			if m.snapshot.Profiles.Profiles == nil {
				m.snapshot.Profiles.Profiles = map[string]config.Profile{}
			}
			m.snapshot.Profiles.Profiles[profileKey] = config.Profile{
				Name:        displayName,
				Description: description,
				Agents:      map[string]config.Assignment{},
				Categories:  map[string]config.Assignment{},
				Settings:    map[string]any{},
			}
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			m.status = fmt.Sprintf("Created profile: %s", profileKey)
			return *m, nil
		}
	}

	for i := range m.profileForm.inputs {
		if i == m.profileForm.focus {
			m.profileForm.inputs[i], _ = m.profileForm.inputs[i].Update(msg)
		} else {
			m.profileForm.inputs[i].Blur()
		}
		cmds = append(cmds, textinput.Blink)
	}
	return *m, tea.Batch(cmds...)
}

func (m *Model) focusProfileForm() {
	for i := range m.profileForm.inputs {
		if i == m.profileForm.focus {
			m.profileForm.inputs[i].Focus()
		} else {
			m.profileForm.inputs[i].Blur()
		}
	}
}

func (m *Model) updateBulkAssign(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	options := m.filteredModelOptions()
	m.pickerSelection = clamp(m.pickerSelection, len(options))

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "shift+enter":
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			m.status = "Cancelled bulk assign"
			return *m, nil
		case "down", "j":
			m.pickerSelection = moveIndex(m.pickerSelection, len(options), 1)
			return *m, nil
		case "up", "k":
			m.pickerSelection = moveIndex(m.pickerSelection, len(options), -1)
			return *m, nil
		case "tab":
			if m.bulkAssign.targetType == "agents" {
				m.bulkAssign.targetType = "categories"
			} else {
				m.bulkAssign.targetType = "agents"
			}
			return *m, nil
		case "enter":
			if len(options) == 0 {
				return *m, nil
			}
			selected := options[m.pickerSelection]
			count := 0

			targetProfile := m.bulkAssign.targetProfile
			if targetProfile == "" {
				targetProfile = m.activeProfile
			}

			if m.bulkAssign.targetType == "agents" {
				if targetProfile == m.activeProfile {
					oldAgents := cloneAssignments(m.agents)
					for key, assignment := range m.agents {
						oldModel := assignment.Model
						assignment.Model = selected.ID
						m.agents[key] = assignment
						if oldModel != selected.ID {
							count++
						}
					}
					m.undo.push(edit{
						editType:  "bulkAssign",
						key:       "agents",
						newValue:  selected.ID,
						oldAgents: oldAgents,
					})
				} else {
					profile := m.snapshot.Profiles.Profiles[targetProfile]
					if profile.Agents == nil {
						profile.Agents = map[string]config.Assignment{}
					}
					for key, assignment := range profile.Agents {
						oldModel := assignment.Model
						assignment.Model = selected.ID
						profile.Agents[key] = assignment
						if oldModel != selected.ID {
							count++
						}
					}
					m.snapshot.Profiles.Profiles[targetProfile] = profile
				}
				m.status = fmt.Sprintf("Assigned %s to %d agents in profile '%s' (press Ctrl+S to save)", selected.ID, count, targetProfile)
			} else {
				if targetProfile == m.activeProfile {
					oldCategories := cloneAssignments(m.categories)
					for key, assignment := range m.categories {
						oldModel := assignment.Model
						assignment.Model = selected.ID
						m.categories[key] = assignment
						if oldModel != selected.ID {
							count++
						}
					}
					m.undo.push(edit{
						editType:      "bulkAssign",
						key:           "categories",
						newValue:      selected.ID,
						oldCategories: oldCategories,
					})
				} else {
					profile := m.snapshot.Profiles.Profiles[targetProfile]
					if profile.Categories == nil {
						profile.Categories = map[string]config.Assignment{}
					}
					for key, assignment := range profile.Categories {
						oldModel := assignment.Model
						assignment.Model = selected.ID
						profile.Categories[key] = assignment
						if oldModel != selected.ID {
							count++
						}
					}
					m.snapshot.Profiles.Profiles[targetProfile] = profile
				}
				m.status = fmt.Sprintf("Assigned %s to %d categories in profile '%s' (press Ctrl+S to save)", selected.ID, count, targetProfile)
			}
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			return *m, nil
		}
	}

	return *m, cmd
}

func (m *Model) updateSwapProvider(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "shift+enter":
			m.mode = modeNormal
			m.resetSearch("Search")
			m.searchFocused = false
			m.search.Blur()
			m.swapProvider = swapProviderState{}
			m.status = "Cancelled provider swap"
			return *m, nil
		case "down", "j":
			if m.swapProvider.fromProvider == "" {
				providers := m.getProviderList()
				m.pickerSelection = moveIndex(m.pickerSelection, len(providers), 1)
			} else if m.swapProvider.toProvider == "" {
				providers := m.getProviderList()
				m.pickerSelection = moveIndex(m.pickerSelection, len(providers), 1)
			}
			return *m, nil
		case "up", "k":
			if m.swapProvider.fromProvider == "" {
				providers := m.getProviderList()
				m.pickerSelection = moveIndex(m.pickerSelection, len(providers), -1)
			} else if m.swapProvider.toProvider == "" {
				providers := m.getProviderList()
				m.pickerSelection = moveIndex(m.pickerSelection, len(providers), -1)
			}
			return *m, nil
		case "enter":
			providers := m.getProviderList()
			if len(providers) == 0 {
				return *m, nil
			}
			selected := providers[m.pickerSelection]
			if m.swapProvider.fromProvider == "" {
				m.swapProvider.fromProvider = selected
				m.pickerSelection = 0
				m.resetSearch("Select target provider")
				m.searchFocused = true
				m.search.Focus()
				return *m, nil
			} else if m.swapProvider.toProvider == "" {
				if selected == m.swapProvider.fromProvider {
					m.status = "Cannot swap to the same provider"
					return *m, nil
				}
				m.swapProvider.toProvider = selected
				count := m.performProviderSwap()

				// Push undo entry for bulk swap
				m.undo.push(edit{
					editType:      "bulkSwap",
					oldAgents:     m.swapProvider.oldAgents,
					oldCategories: m.swapProvider.oldCategories,
					oldDefault:    m.swapProvider.oldDefault,
				})

				m.mode = modeNormal
				m.resetSearch("Search")
				m.searchFocused = false
				m.search.Blur()
				m.status = fmt.Sprintf("Swapped %d models from %s to %s (press Ctrl+S to save)", count, m.swapProvider.fromProvider, m.swapProvider.toProvider)
				m.swapProvider = swapProviderState{}
				return *m, nil
			}
		}
	}

	return *m, cmd
}

func (m *Model) getProviderList() []string {
	providerSet := map[string]struct{}{}
	for _, opt := range m.allModelOptions() {
		if opt.ProviderID != "" {
			providerSet[opt.ProviderID] = struct{}{}
		}
	}
	providers := make([]string, 0, len(providerSet))
	for id := range providerSet {
		if containsFold(id, m.search.Value()) {
			providers = append(providers, id)
		}
	}
	sort.Strings(providers)
	return providers
}

func (m *Model) performProviderSwap() int {
	count := 0
	fromPrefix := m.swapProvider.fromProvider + "/"
	toPrefix := m.swapProvider.toProvider + "/"

	// Store old state for undo
	m.swapProvider.oldAgents = cloneAssignments(m.agents)
	m.swapProvider.oldCategories = cloneAssignments(m.categories)
	m.swapProvider.oldDefault = m.defaultModel

	// Swap agents
	for key, assignment := range m.agents {
		if strings.HasPrefix(assignment.Model, fromPrefix) {
			newModel := toPrefix + strings.TrimPrefix(assignment.Model, fromPrefix)
			assignment.Model = newModel
			m.agents[key] = assignment
			count++
		}
	}

	// Swap categories
	for key, assignment := range m.categories {
		if strings.HasPrefix(assignment.Model, fromPrefix) {
			newModel := toPrefix + strings.TrimPrefix(assignment.Model, fromPrefix)
			assignment.Model = newModel
			m.categories[key] = assignment
			count++
		}
	}

	// Also swap the default model if it matches
	if strings.HasPrefix(m.defaultModel, fromPrefix) {
		m.defaultModel = toPrefix + strings.TrimPrefix(m.defaultModel, fromPrefix)
		count++
	}

	return count
}

func (m *Model) undoLast() (tea.Model, tea.Cmd) {
	if !m.undo.canUndo() {
		m.status = "Nothing to undo"
		return *m, nil
	}

	e, ok := m.undo.pop()
	if !ok {
		m.status = "Nothing to undo"
		return *m, nil
	}

	switch e.editType {
	case "assign":
		switch e.section {
		case SectionAgents:
			assignment := m.agents[e.key]
			assignment.Model = e.oldValue
			m.agents[e.key] = assignment
			m.status = fmt.Sprintf("Undid: restored %s model for agent %s", e.oldValue, e.key)
		case SectionCategories:
			assignment := m.categories[e.key]
			assignment.Model = e.oldValue
			m.categories[e.key] = assignment
			m.status = fmt.Sprintf("Undid: restored %s model for category %s", e.oldValue, e.key)
		}
	case "default":
		m.defaultModel = e.oldValue
		m.status = fmt.Sprintf("Undid: restored default model to %s", e.oldValue)
	case "clear":
		switch e.section {
		case SectionAgents:
			assignment := m.agents[e.key]
			assignment.Model = e.oldValue
			m.agents[e.key] = assignment
			m.status = fmt.Sprintf("Undid: restored %s model for agent %s", e.oldValue, e.key)
		case SectionCategories:
			assignment := m.categories[e.key]
			assignment.Model = e.oldValue
			m.categories[e.key] = assignment
			m.status = fmt.Sprintf("Undid: restored %s model for category %s", e.oldValue, e.key)
		case SectionDefaults:
			m.defaultModel = e.oldValue
			m.status = fmt.Sprintf("Undid: restored default model to %s", e.oldValue)
		}
	case "bulkSwap":
		if e.oldAgents != nil {
			m.agents = cloneAssignments(e.oldAgents)
		}
		if e.oldCategories != nil {
			m.categories = cloneAssignments(e.oldCategories)
		}
		m.defaultModel = e.oldDefault
		m.status = "Undid: provider swap"
	case "bulkAssign":
		if e.oldAgents != nil {
			m.agents = cloneAssignments(e.oldAgents)
		}
		if e.oldCategories != nil {
			m.categories = cloneAssignments(e.oldCategories)
		}
		m.status = fmt.Sprintf("Undid: bulk assign to %s", e.key)
	}

	return *m, nil
}

func (m *Model) pushUndoForClear() {
	switch m.currentSection() {
	case SectionAgents:
		keys := m.filteredAgents()
		if len(keys) > 0 {
			key := keys[m.currentSelection()]
			oldValue := m.agents[key].Model
			m.undo.push(edit{section: SectionAgents, key: key, oldValue: oldValue, newValue: "", editType: "clear"})
		}
	case SectionCategories:
		keys := m.filteredCategories()
		if len(keys) > 0 {
			key := keys[m.currentSelection()]
			oldValue := m.categories[key].Model
			m.undo.push(edit{section: SectionCategories, key: key, oldValue: oldValue, newValue: "", editType: "clear"})
		}
	case SectionDefaults:
		oldValue := m.defaultModel
		m.undo.push(edit{section: SectionDefaults, key: "default", oldValue: oldValue, newValue: "", editType: "clear"})
	}
}

func (m *Model) enterMode() (tea.Model, tea.Cmd) {
	switch m.currentSection() {
	case SectionProfiles:
		keys := m.filteredProfiles()
		if len(keys) == 0 {
			return *m, nil
		}
		selected := keys[m.currentSelection()]
		if err := m.loadProfile(selected); err != nil {
			m.err = err
			m.status = err.Error()
			return *m, nil
		}
		m.status = fmt.Sprintf("Switched to profile: %s", selected)
		return *m, nil
	case SectionAgents, SectionCategories:
		m.viewState = stateDetail
		return *m, nil
	case SectionProviders:
		items := m.filteredProviders()
		if len(items) == 0 {
			return *m, nil
		}
		selected := items[m.currentSelection()]
		m.providerForm = newProviderForm(selected.Template, selected.Provider)
		m.mode = modeProviderEditor
		return *m, textinput.Blink
	case SectionDefaults:
		m.openModelPicker(pickerTarget{kind: pickerDefault})
		return *m, textinput.Blink
	case SectionReview:
		m.undo.clear()
		return m.save()
	case SectionHelp:
		// Open the selected help link in browser
		helpLinks := []string{
			"https://github.com/anomalyco/opencode",
			"https://github.com/gavintomlins/oh-my-opencode",
			"https://agentskills.io/home",
			"https://github.com/awesome-opencode/awesome-opencode",
			"https://charm.sh",
		}
		if m.helpSelection >= 0 && m.helpSelection < len(helpLinks) {
			url := helpLinks[m.helpSelection]
			if err := openBrowser(url); err != nil {
				m.status = fmt.Sprintf("Error opening browser: %v", err)
			} else {
				m.status = fmt.Sprintf("Opening: %s", url)
			}
		}
		return *m, nil
	case SectionSkills:
		// These are display-only sections, Enter just acknowledges them
		m.status = fmt.Sprintf("Viewing %s section", sectionTitle(m.currentSection()))
		return *m, nil
	}
	return *m, nil
}

func (m *Model) activateSelection() (tea.Model, tea.Cmd) {
	switch m.currentSection() {
	case SectionAgents:
		keys := m.filteredAgents()
		if len(keys) == 0 {
			return *m, nil
		}
		m.openModelPicker(pickerTarget{kind: pickerAgent, key: keys[m.currentSelection()]})
		return *m, textinput.Blink
	case SectionCategories:
		keys := m.filteredCategories()
		if len(keys) == 0 {
			return *m, nil
		}
		m.openModelPicker(pickerTarget{kind: pickerCategory, key: keys[m.currentSelection()]})
		return *m, textinput.Blink
	}
	return *m, nil
}

func (m *Model) clearCurrent() (tea.Model, tea.Cmd) {
	switch m.currentSection() {
	case SectionAgents:
		keys := m.filteredAgents()
		if len(keys) > 0 {
			assignment := m.agents[keys[m.currentSelection()]]
			assignment.Model = ""
			m.agents[keys[m.currentSelection()]] = assignment
			m.status = fmt.Sprintf("Cleared model for agent %s", keys[m.currentSelection()])
		}
	case SectionCategories:
		keys := m.filteredCategories()
		if len(keys) > 0 {
			assignment := m.categories[keys[m.currentSelection()]]
			assignment.Model = ""
			m.categories[keys[m.currentSelection()]] = assignment
			m.status = fmt.Sprintf("Cleared model for category %s", keys[m.currentSelection()])
		}
	case SectionProviders:
		items := m.filteredProviders()
		if len(items) > 0 {
			selected := items[m.currentSelection()]
			delete(m.providers, selected.ID)
			m.status = fmt.Sprintf("Removed provider %s", selected.ID)
		}
	case SectionDefaults:
		m.defaultModel = ""
		m.status = "Cleared default model"
	}
	return *m, nil
}

func (m *Model) applyDefault() (tea.Model, tea.Cmd) {
	if m.defaultModel == "" {
		m.status = "Set a default model first"
		return *m, nil
	}
	count := 0
	for key, value := range m.agents {
		if strings.TrimSpace(value.Model) == "" {
			value.Model = m.defaultModel
			m.agents[key] = value
			count++
		}
	}
	for key, value := range m.categories {
		if strings.TrimSpace(value.Model) == "" {
			value.Model = m.defaultModel
			m.categories[key] = value
			count++
		}
	}
	m.status = fmt.Sprintf("Applied default model to %d unassigned entries", count)
	return *m, nil
}

func (m *Model) save() (tea.Model, tea.Cmd) {
	m.syncWorkingIntoSnapshotProfile()
	err := config.Save(config.SaveInput{
		Snapshot:      m.snapshot,
		ActiveProfile: m.activeProfile,
		Agents:        m.agents,
		Categories:    m.categories,
		Providers:     m.providers,
		DefaultModel:  m.defaultModel,
	})
	if err != nil {
		m.err = err
		m.status = "Save failed: " + err.Error()
		return *m, nil
	}
	m.updateFileInfos()
	refreshed, err := config.Load(m.paths)
	if err != nil {
		m.status = "Saved, but reload failed: " + err.Error()
		return *m, nil
	}
	m.original = refreshed
	m.snapshot = refreshed
	m.providers = cloneProviders(refreshed.Providers.Provider)
	m.defaultModel = refreshed.UIState.DefaultModel
	if m.defaultModel == "" {
		m.defaultModel = m.firstAvailableModel()
	}
	_ = m.loadProfile(refreshed.Profiles.ActiveProfile)
	m.status = "Saved configuration"
	return *m, nil
}

func (m *Model) loadProfile(name string) error {
	if name == "" {
		name = "default"
	}
	if m.activeProfile != "" {
		m.syncWorkingIntoSnapshotProfile()
	}
	resolved, err := m.snapshot.Profiles.ResolveProfile(name)
	if err != nil {
		return err
	}
	m.activeProfile = name
	m.agents = cloneAssignments(resolved.Agents)
	m.categories = cloneAssignments(resolved.Categories)
	for _, key := range m.snapshot.AgentKeys() {
		if _, ok := m.agents[key]; !ok {
			m.agents[key] = config.Assignment{}
		}
	}
	for _, key := range m.snapshot.CategoryKeys() {
		if _, ok := m.categories[key]; !ok {
			m.categories[key] = config.Assignment{}
		}
	}
	return nil
}

func (m *Model) syncWorkingIntoSnapshotProfile() {
	if m.activeProfile == "" {
		return
	}
	profile := m.snapshot.Profiles.Profiles[m.activeProfile]
	profile.Agents = cloneAssignments(m.agents)
	profile.Categories = cloneAssignments(m.categories)
	m.snapshot.Profiles.Profiles[m.activeProfile] = profile
	m.snapshot.Profiles.ActiveProfile = m.activeProfile
	if m.snapshot.UIState.DefaultModel != m.defaultModel {
		m.snapshot.UIState.DefaultModel = m.defaultModel
	}
	if m.snapshot.Providers.Provider == nil {
		m.snapshot.Providers.Provider = map[string]config.Provider{}
	}
	for key := range m.snapshot.Providers.Provider {
		delete(m.snapshot.Providers.Provider, key)
	}
	for key, value := range m.providers {
		m.snapshot.Providers.Provider[key] = value.Clone()
	}
}

func (m *Model) openModelPicker(target pickerTarget) {
	m.mode = modeModelPicker
	m.pickerTarget = target
	m.pickerSelection = 0
	m.resetSearch("Search models")
	m.searchFocused = true
	m.search.Focus()
}

func (m Model) currentSection() Section {
	return m.sections[m.sectionIndex]
}

func (m *Model) moveSelection(delta int) {
	section := m.currentSection()
	length := m.currentListLength()
	m.selection[section] = moveIndex(m.selection[section], length, delta)
}

func (m Model) currentSelection() int {
	section := m.currentSection()
	return clamp(m.selection[section], m.currentListLength())
}

func (m Model) currentListLength() int {
	switch m.currentSection() {
	case SectionProfiles:
		return len(m.filteredProfiles())
	case SectionAgents:
		return len(m.filteredAgents())
	case SectionCategories:
		return len(m.filteredCategories())
	case SectionProviders:
		return len(m.filteredProviders())
	default:
		return 1
	}
}

func (m Model) filteredProfiles() []string {
	keys := make([]string, 0, len(m.snapshot.Profiles.Profiles))
	for key := range m.snapshot.Profiles.Profiles {
		if containsFold(key, m.search.Value()) || containsFold(m.snapshot.Profiles.Profiles[key].Name, m.search.Value()) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (m Model) filteredAgents() []string {
	keys := m.snapshot.AgentKeys()
	return filterStrings(keys, m.search.Value())
}

func (m Model) filteredCategories() []string {
	keys := m.snapshot.CategoryKeys()
	return filterStrings(keys, m.search.Value())
}

func (m Model) filteredProviders() []providerItem {
	items := []providerItem{}
	seen := map[string]struct{}{}
	for _, tmpl := range m.builtinTemplates {
		provider, ok := m.providers[tmpl.ID]
		if !ok {
			provider = providercatalog.ProviderFromTemplate(tmpl)
		}
		item := providerItem{ID: tmpl.ID, Name: choose(provider.Name, tmpl.Name), Connected: ok, Template: tmpl, Provider: provider, ModelCount: len(provider.Models), Custom: tmpl.Custom}
		if containsFold(item.ID, m.search.Value()) || containsFold(item.Name, m.search.Value()) {
			items = append(items, item)
		}
		seen[tmpl.ID] = struct{}{}
	}
	for id, provider := range m.providers {
		if _, ok := seen[id]; ok {
			continue
		}
		item := providerItem{ID: id, Name: choose(provider.Name, id), Connected: true, Provider: provider, ModelCount: len(provider.Models), Custom: true}
		if containsFold(item.ID, m.search.Value()) || containsFold(item.Name, m.search.Value()) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Connected != items[j].Connected {
			return items[i].Connected
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func (m Model) filteredProviderCatalog() []providercatalog.Template {
	items := []providercatalog.Template{}
	for _, item := range m.builtinTemplates {
		if containsFold(item.ID, m.search.Value()) || containsFold(item.Name, m.search.Value()) {
			items = append(items, item)
		}
	}
	return items
}

func (m Model) allModelOptions() []modelOption {
	options := map[string]modelOption{}
	for id, def := range m.snapshot.Profiles.ModelDefinitions {
		options[id] = modelOption{ID: id, Name: choose(def.Name, id), ProviderID: def.Provider, ProviderName: providerDisplayName(def.Provider, m.providers, m.builtinMap), Source: "profile definitions"}
	}
	for providerID, provider := range m.providers {
		for modelID, model := range provider.Models {
			fullID := providerID + "/" + modelID
			options[fullID] = modelOption{ID: fullID, Name: choose(model.Name, modelID), ProviderID: providerID, ProviderName: providerDisplayName(providerID, m.providers, m.builtinMap), Source: "configured provider"}
		}
	}
	for _, tmpl := range m.builtinTemplates {
		for _, model := range tmpl.Models {
			fullID := tmpl.ID + "/" + model.ID
			if _, ok := options[fullID]; !ok {
				options[fullID] = modelOption{ID: fullID, Name: model.Name, ProviderID: tmpl.ID, ProviderName: tmpl.Name, Source: "builtin catalog"}
			}
		}
	}
	items := make([]modelOption, 0, len(options))
	for _, item := range options {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ProviderName != items[j].ProviderName {
			return items[i].ProviderName < items[j].ProviderName
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func (m Model) filteredModelOptions() []modelOption {
	items := []modelOption{}
	for _, item := range m.allModelOptions() {
		if containsFold(item.ID, m.search.Value()) || containsFold(item.Name, m.search.Value()) || containsFold(item.ProviderName, m.search.Value()) {
			items = append(items, item)
		}
	}
	return items
}

func (m Model) firstAvailableModel() string {
	items := m.allModelOptions()
	if len(items) == 0 {
		return ""
	}
	return items[0].ID
}

func (m *Model) resetSearch(placeholder string) {
	m.search.SetValue("")
	m.search.Placeholder = placeholder
}

func (m *Model) focusProviderForm() {
	for i := range m.providerForm.inputs {
		if i == m.providerForm.focus {
			m.providerForm.inputs[i].Focus()
		} else {
			m.providerForm.inputs[i].Blur()
		}
	}
}

func newProfileForm() profileForm {
	inputs := make([]textinput.Model, 3)
	placeholders := []string{"my-profile", "My Profile", "Description of this profile"}
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Prompt = ""
		inputs[i].Placeholder = placeholders[i]
		inputs[i].Width = 48
	}
	inputs[0].Focus()
	return profileForm{inputs: inputs, focus: 0}
}

func newProviderForm(tmpl providercatalog.Template, existing config.Provider) providerForm {
	inputs := make([]textinput.Model, 5)
	labels := []string{"Provider ID", "Display name", "Base URL", "API key", "ENV var (optional)"}
	values := []string{tmpl.ID, choose(existing.Name, tmpl.Name), choose(existing.Options["baseURL"], tmpl.DefaultBaseURL), "", ""}
	if existingKey := existing.Options["apiKey"]; strings.HasPrefix(existingKey, "${") && strings.HasSuffix(existingKey, "}") {
		values[4] = strings.TrimSuffix(strings.TrimPrefix(existingKey, "${"), "}")
	} else if existingKey != "" && existingKey != "local" {
		values[3] = existingKey
	}
	if tmpl.Custom {
		values[0] = choose(values[0], "custom-provider")
	}
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Prompt = ""
		inputs[i].Placeholder = labels[i]
		inputs[i].SetValue(values[i])
		inputs[i].Width = 48
	}
	form := providerForm{template: tmpl, existing: existing.Name != "" || len(existing.Models) > 0, inputs: inputs, focus: 0}
	for i := range form.inputs {
		if i == 0 {
			form.inputs[i].Focus()
		}
	}
	return form
}

func (m Model) renderContent() string {
	navWidth := 20
	contentWidth := m.width - navWidth

	leftNav := m.viewNav(navWidth)
	rightContent := m.viewMain(contentWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftNav, rightContent)
}

func (m Model) viewNav(width int) string {
	items := make([]string, 0, len(m.sections))
	for i, section := range m.sections {
		name := sectionTitle(section)
		prefix := "  "
		if i == m.sectionIndex {
			if m.viewState == stateList {
				prefix = "▶ "
				name = navActiveStyle.Render(prefix + name + " ")
			} else {
				prefix = "○ "
				name = navInactiveStyle.Render(prefix + name + " ")
			}
		} else {
			name = navItemStyle.Render(prefix + name)
		}
		items = append(items, name)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return navBoxStyle.Width(width).Height(m.height - 5).Render(content)
}

func (m Model) viewMain(width int) string {
	contentHeight := m.height - 5

	header := headerStyle.Render(sectionTitle(m.currentSection()))

	searchBox := searchBoxStyle.Render(m.search.View())

	var body string
	switch m.currentSection() {
	case SectionProfiles:
		body = m.viewProfiles(width - 4)
	case SectionAgents:
		body = m.viewAssignments(m.filteredAgents(), m.agents, "agent", width-4)
	case SectionCategories:
		body = m.viewAssignments(m.filteredCategories(), m.categories, "category", width-4)
	case SectionProviders:
		body = m.viewProviders(width - 4)
	case SectionDefaults:
		body = m.viewDefaults(width - 4)
	case SectionReview:
		body = m.viewReview(width - 4)
	case SectionHelp:
		body = m.viewHelp(width - 4)
	case SectionSkills:
		body = m.viewSkills(width - 4)
	}

	fullBody := lipgloss.JoinVertical(lipgloss.Left, header, searchBox, body)
	return contentStyle.Width(width).Height(contentHeight).Render(fullBody)
}

func (m Model) viewProfiles(width int) string {
	keys := m.filteredProfiles()
	listWidth := width / 2
	detailWidth := width - listWidth - 2

	var listContent string
	if len(keys) == 0 {
		listContent = mutedStyle.Render("No profiles")
	} else {
		items := make([]string, len(keys))
		for i, key := range keys {
			profile := m.snapshot.Profiles.Profiles[key]
			name := key
			if key == m.activeProfile {
				name += " [active]"
			}
			desc := profile.Description
			if desc == "" {
				desc = "No description"
			}

			line := fmt.Sprintf("%s\n%s", name, mutedStyle.Render(shorten(desc, 50)))
			if i == m.currentSelection() && m.viewState == stateList {
				line = selectedItemStyle.Render(" " + line)
			} else {
				line = listItemStyle.Render(" " + line)
			}
			items[i] = line
		}
		listContent = lipgloss.JoinVertical(lipgloss.Left, items...)
	}

	detailContent := lipgloss.JoinVertical(lipgloss.Left,
		mutedStyle.Render("Profile Management"),
		"",
		cmdStyle.Render(" enter ")+" switch to profile",
		cmdStyle.Render(" n ")+" create new profile",
		cmdStyle.Render(" b ")+" bulk assign to all agents",
		cmdStyle.Render(" c ")+" bulk assign to all categories",
		cmdStyle.Render(" s ")+" swap all models to new provider",
	)

	if len(keys) > 0 && m.currentSelection() < len(keys) {
		key := keys[m.currentSelection()]
		profile := m.snapshot.Profiles.Profiles[key]

		// Show working state counts if this is the active profile
		agentCount := len(profile.Agents)
		categoryCount := len(profile.Categories)
		if key == m.activeProfile {
			agentCount = len(m.agents)
			categoryCount = len(m.categories)
		}

		detailContent = lipgloss.JoinVertical(lipgloss.Left,
			detailTitleStyle.Render(choose(profile.Name, key)),
			"",
			fmt.Sprintf("Key: %s", key),
			fmt.Sprintf("Extends: %s", choose(profile.Extends, "-")),
			fmt.Sprintf("Agents: %d", agentCount),
			fmt.Sprintf("Categories: %d", categoryCount),
			"",
			mutedStyle.Render(profile.Description),
			"",
			mutedStyle.Render("Commands:"),
			cmdStyle.Render(" enter ")+" switch to profile ",
			cmdStyle.Render(" n ")+" create new profile ",
			cmdStyle.Render(" b ")+" bulk assign model to all agents ",
			cmdStyle.Render(" c ")+" bulk assign model to all categories ",
			cmdStyle.Render(" s ")+" swap all models to new provider ",
		)
	}

	listHeader := mutedStyle.Render("─ Profiles ─")
	detailHeader := mutedStyle.Render("─ Configuration ─")
	fullListContent := lipgloss.JoinVertical(lipgloss.Left, listHeader, "", listContent)
	fullDetailContent := lipgloss.JoinVertical(lipgloss.Left, detailHeader, "", detailContent)

	listPane := listPaneStyle.Width(listWidth).Height(m.height - 10).Render(fullListContent)
	detailPane := detailPaneStyle.Width(detailWidth).Height(m.height - 10).Render(fullDetailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
}

func (m Model) viewAssignments(keys []string, values map[string]config.Assignment, noun string, width int) string {
	listWidth := width / 2
	detailWidth := width - listWidth - 2

	var listContent string
	if len(keys) == 0 {
		listContent = mutedStyle.Render(fmt.Sprintf("No %ss", noun))
	} else {
		items := make([]string, len(keys))
		for i, key := range keys {
			assignment := values[key]
			model := assignment.Model
			if model == "" {
				model = "(unassigned)"
			}

			line := fmt.Sprintf("%s\n%s", key, mutedStyle.Render(shorten(model, 40)))
			if i == m.currentSelection() && m.viewState == stateList {
				line = selectedItemStyle.Render(" " + line)
			} else {
				line = listItemStyle.Render(" " + line)
			}
			items[i] = line
		}
		listContent = lipgloss.JoinVertical(lipgloss.Left, items...)
	}

	var bulkCmd string
	if noun == "agent" {
		bulkCmd = cmdStyle.Render(" b ") + " bulk assign to all agents "
	} else {
		bulkCmd = cmdStyle.Render(" c ") + " bulk assign to all categories "
	}

	detailContent := lipgloss.JoinVertical(lipgloss.Left,
		mutedStyle.Render(fmt.Sprintf("Select a %s from the list", noun)),
		"",
		cmdStyle.Render(" Enter ")+" configure ",
		bulkCmd,
	)
	if m.viewState == stateDetail && len(keys) > 0 && m.currentSelection() < len(keys) {
		key := keys[m.currentSelection()]
		assignment := values[key]
		detailContent = lipgloss.JoinVertical(lipgloss.Left,
			detailTitleStyle.Render(key),
			"",
			fmt.Sprintf("Model: %s", choose(assignment.Model, "(unassigned)")),
			fmt.Sprintf("Variant: %s", choose(assignment.Variant, "-")),
			"",
			cmdStyle.Render(" Enter ")+" select model ",
			cmdStyle.Render(" x ")+" clear ",
			cmdStyle.Render(" esc ")+" back ",
		)
		if assignment.Prompt != "" {
			detailContent = lipgloss.JoinVertical(lipgloss.Left, detailContent, "", mutedStyle.Render("Prompt:"), mutedStyle.Render(shorten(assignment.Prompt, 100)))
		}
	}

	listHeader := mutedStyle.Render("─ Items ─")
	detailHeader := mutedStyle.Render("─ Configuration ─")
	fullListContent := lipgloss.JoinVertical(lipgloss.Left, listHeader, "", listContent)
	fullDetailContent := lipgloss.JoinVertical(lipgloss.Left, detailHeader, "", detailContent)

	listPane := listPaneStyle.Width(listWidth).Height(m.height - 10).Render(fullListContent)
	detailPane := detailPaneStyle.Width(detailWidth).Height(m.height - 10).Render(fullDetailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
}

func (m Model) viewProviders(width int) string {
	items := m.filteredProviders()
	listWidth := width / 2
	detailWidth := width - listWidth - 2

	var listContent string
	if len(items) == 0 {
		listContent = mutedStyle.Render("No providers")
	} else {
		rows := make([]string, len(items))
		for i, item := range items {
			status := "available"
			if item.Connected {
				status = "connected"
			}
			line := fmt.Sprintf("%s\n%s · %d models", item.Name, mutedStyle.Render(status), item.ModelCount)
			if i == m.currentSelection() && m.viewState == stateList {
				line = selectedItemStyle.Render(" " + line)
			} else {
				line = listItemStyle.Render(" " + line)
			}
			rows[i] = line
		}
		listContent = lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	detailContent := "Select a provider"
	if len(items) > 0 && m.currentSelection() < len(items) {
		selected := items[m.currentSelection()]
		apiKey := selected.Provider.Options["apiKey"]
		apiState := "not set"
		if apiKey != "" {
			apiState = maskedKey(apiKey)
		}
		detailContent = lipgloss.JoinVertical(lipgloss.Left,
			detailTitleStyle.Render(selected.Name),
			"",
			fmt.Sprintf("ID: %s", selected.ID),
			fmt.Sprintf("Status: %s", choose(selected.Provider.Options["baseURL"], "-")),
			fmt.Sprintf("API: %s", apiState),
			"",
			cmdStyle.Render(" Enter ")+" edit provider ",
			cmdStyle.Render(" a ")+" add new ",
			cmdStyle.Render(" x ")+" remove ",
		)
	}

	listHeader := mutedStyle.Render("─ Providers ─")
	detailHeader := mutedStyle.Render("─ Configuration ─")
	fullListContent := lipgloss.JoinVertical(lipgloss.Left, listHeader, "", listContent)
	fullDetailContent := lipgloss.JoinVertical(lipgloss.Left, detailHeader, "", detailContent)

	listPane := listPaneStyle.Width(listWidth).Height(m.height - 10).Render(fullListContent)
	detailPane := detailPaneStyle.Width(detailWidth).Height(m.height - 10).Render(fullDetailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
}

func (m Model) viewDefaults(width int) string {
	content := lipgloss.JoinVertical(lipgloss.Left,
		detailTitleStyle.Render("Default Model"),
		"",
		fmt.Sprintf("Current: %s", choose(m.defaultModel, "(none set)")),
		"",
		mutedStyle.Render("This model is used as a fallback for unassigned agents and categories."),
		"",
		cmdStyle.Render(" Enter ")+" select default model ",
		cmdStyle.Render(" d ")+" apply to all unassigned ",
		cmdStyle.Render(" x ")+" clear default ",
	)
	return defaultsPaneStyle.Width(width).Height(m.height - 10).Render(content)
}

func (m Model) viewReview(width int) string {
	profileChanges := diffAssignments(m.original, m.activeProfile, m.agents, m.categories)
	providerChanges := diffProviders(m.original.Providers.Provider, m.providers)
	defaultChanged := m.defaultModel != m.original.UIState.DefaultModel

	content := lipgloss.JoinVertical(lipgloss.Left,
		detailTitleStyle.Render("Review Changes"),
		"",
		fmt.Sprintf("Profile: %s", m.activeProfile),
		fmt.Sprintf("Agent changes: %d", profileChanges.agentChanges),
		fmt.Sprintf("Category changes: %d", profileChanges.categoryChanges),
		fmt.Sprintf("Provider changes: %d", providerChanges),
		fmt.Sprintf("Default model changed: %t", defaultChanged),
		"",
		cmdStyle.Render(" Enter ")+" or "+cmdStyle.Render(" ctrl+s ")+" to save ",
		mutedStyle.Render("Backups are created before overwriting."),
	)

	return reviewPaneStyle.Width(width).Height(m.height - 10).Render(content)
}

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}

func (m Model) viewHelp(width int) string {
	helpLinks := []struct {
		name string
		url  string
	}{
		{"opencode repository", "https://github.com/anomalyco/opencode"},
		{"oh-my-opencode repository", "https://github.com/gavintomlins/oh-my-opencode"},
		{"Agent Skills", "https://agentskills.io/home"},
		{"Awesome Opencode", "https://github.com/awesome-opencode/awesome-opencode"},
		{"Charm.sh (TUI Framework)", "https://charm.sh"},
	}

	items := []string{
		detailTitleStyle.Render("Help & Resources"),
		"",
		"Useful Links:",
		"",
	}

	for i, link := range helpLinks {
		line := fmt.Sprintf("  • %s", link.name)
		if i == m.helpSelection {
			line = selectedItemStyle.Render("▶ " + link.name)
		}
		items = append(items, line)
		items = append(items, mutedStyle.Render("    "+link.url))
		items = append(items, "")
	}

	items = append(items, "")
	items = append(items, cmdStyle.Render(" ↑↓ ")+"navigate "+cmdStyle.Render(" enter ")+"open link "+cmdStyle.Render(" esc ")+"back")
	items = append(items, mutedStyle.Render("Press Enter to open link in browser"))

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return reviewPaneStyle.Width(width).Height(m.height - 10).Render(content)
}

func (m Model) viewSkills(width int) string {
	var items []string
	items = append(items, detailTitleStyle.Render("Skills & Sources"))
	items = append(items, "")
	items = append(items, "Built-in Skills:")
	items = append(items, "  • fs-read     - File reading operations")
	items = append(items, "  • fs-write    - File writing operations")
	items = append(items, "  • fs-edit     - File editing operations")
	items = append(items, "  • bash        - Shell command execution")
	items = append(items, "  • browser     - Web browser automation")
	items = append(items, "  • websearch   - Web search capabilities")
	items = append(items, "  • github      - GitHub integration")
	items = append(items, "")
	items = append(items, "External Skill Sources:")
	items = append(items, "")
	items = append(items, "  • Agent Skills:")
	items = append(items, "    https://agentskills.io/home")
	items = append(items, "")
	items = append(items, "  • Awesome Opencode:")
	items = append(items, "    https://github.com/awesome-opencode/awesome-opencode")
	items = append(items, "")
	items = append(items, "  • opencode Core:")
	items = append(items, "    https://github.com/anomalyco/opencode")
	items = append(items, "")
	items = append(items, mutedStyle.Render("Skills extend opencode functionality."))
	items = append(items, mutedStyle.Render("Install external skills via the opencode CLI."))

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return reviewPaneStyle.Width(width).Height(m.height - 10).Render(content)
}

func (m Model) viewModelPicker() string {
	options := m.filteredModelOptions()

	titleBar := titleBarStyle.Render(" Select Model ")

	var listContent string
	if len(options) == 0 {
		listContent = mutedStyle.Render("  No models match your search")
	} else {
		providerCounts := map[string]int{}
		for _, opt := range options {
			providerCounts[opt.ProviderName]++
		}

		items := make([]string, 0, len(options)+len(providerCounts))
		currentProvider := ""
		for i, opt := range options {
			if opt.ProviderName != currentProvider {
				currentProvider = opt.ProviderName
				header := pickerProviderHeaderStyle.Render("  " + currentProvider)
				count := pickerProviderMetaStyle.Render(fmt.Sprintf(" %d", providerCounts[currentProvider]))
				items = append(items, lipgloss.JoinHorizontal(lipgloss.Left, header, count))
			}

			label := "  " + opt.Name
			if i == m.pickerSelection {
				label = pickerSelectedStyle.Render("▶ " + opt.Name + " ")
			}

			metaParts := []string{}
			if opt.ID != "" {
				metaParts = append(metaParts, opt.ID)
			}
			if opt.Source != "" {
				metaParts = append(metaParts, opt.Source)
			}

			if len(metaParts) > 0 {
				label = lipgloss.JoinHorizontal(lipgloss.Left, label, pickerModelMetaStyle.Render("  "+strings.Join(metaParts, "  •  ")))
			}
			items = append(items, label)
		}

		listContent = lipgloss.JoinVertical(lipgloss.Left, items...)
	}

	searchLine := "  " + m.search.View()
	help := "  " + cmdStyle.Render(" ↑↓ ") + "navigate " + cmdStyle.Render(" enter ") + "select " + cmdStyle.Render(" esc ") + "back " + cmdStyle.Render(" type ") + "filter"

	body := lipgloss.JoinVertical(lipgloss.Left, titleBar, "", searchLine, "", listContent, "", help)
	return body
}

func (m Model) viewProviderCatalog() string {
	items := m.filteredProviderCatalog()

	titleBar := titleBarStyle.Render(" Connect a Provider ")

	var listContent string
	if len(items) == 0 {
		listContent = mutedStyle.Render("  No providers match your search")
	} else {
		rows := make([]string, len(items))
		for i, item := range items {
			line := "  " + item.Name
			if i == m.catalogSelection {
				line = pickerSelectedStyle.Render("▶ " + item.Name + " ")
			}
			rows[i] = line
		}
		listContent = lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	searchLine := "  " + m.search.View()
	help := "  " + cmdStyle.Render(" ↑↓ ") + "navigate " + cmdStyle.Render(" enter ") + "configure " + cmdStyle.Render(" esc/shift+enter ") + "back"

	body := lipgloss.JoinVertical(lipgloss.Left, titleBar, "", searchLine, "", listContent, "", help)
	return body
}

func (m Model) viewProviderEditor() string {
	titleBar := titleBarStyle.Render(" Provider Settings ")

	lines := []string{"", "  " + mutedStyle.Render(m.providerForm.template.Name), ""}
	labels := []string{"Provider ID", "Display Name", "Base URL", "API Key", "ENV Variable"}

	for i, input := range m.providerForm.inputs {
		label := labels[i]
		field := input.View()
		if i == m.providerForm.focus {
			field = pickerSelectedStyle.Render("▶ " + field + " ")
		} else {
			field = "  " + field
		}
		lines = append(lines, "  "+mutedStyle.Render(label), field, "")
	}

	help := "  " + cmdStyle.Render(" tab ") + "next " + cmdStyle.Render(" shift+tab ") + "prev " + cmdStyle.Render(" ctrl+s ") + "save " + cmdStyle.Render(" esc ") + "cancel"
	lines = append(lines, help)

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body)
}

func (m Model) viewProfileCreator() string {
	titleBar := titleBarStyle.Render(" Create New Profile ")

	lines := []string{"", "  " + mutedStyle.Render("Enter profile details:"), ""}
	labels := []string{"Profile Key (unique ID)", "Display Name", "Description"}

	for i, input := range m.profileForm.inputs {
		label := labels[i]
		field := input.View()
		if i == m.profileForm.focus {
			field = pickerSelectedStyle.Render("▶ " + field + " ")
		} else {
			field = "  " + field
		}
		lines = append(lines, "  "+mutedStyle.Render(label), field, "")
	}

	help := "  " + cmdStyle.Render(" tab ") + "next " + cmdStyle.Render(" shift+tab ") + "prev " + cmdStyle.Render(" ctrl+s ") + "save " + cmdStyle.Render(" esc ") + "cancel"
	lines = append(lines, help)

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body)
}

func (m Model) viewBulkAssign() string {
	options := m.filteredModelOptions()

	targetLabel := "Agents"
	title := " Bulk Assign to Agents "
	if m.bulkAssign.targetType == "categories" {
		targetLabel = "Categories"
		title = " Bulk Assign to Categories "
	}

	titleBar := titleBarStyle.Render(title)

	var listContent string
	if len(options) == 0 {
		listContent = mutedStyle.Render("  No models match your search")
	} else {
		providerCounts := map[string]int{}
		for _, opt := range options {
			providerCounts[opt.ProviderName]++
		}

		items := make([]string, 0, len(options)+len(providerCounts))
		currentProvider := ""
		for i, opt := range options {
			if opt.ProviderName != currentProvider {
				currentProvider = opt.ProviderName
				header := pickerProviderHeaderStyle.Render("  " + currentProvider)
				count := pickerProviderMetaStyle.Render(fmt.Sprintf(" %d", providerCounts[currentProvider]))
				items = append(items, lipgloss.JoinHorizontal(lipgloss.Left, header, count))
			}

			label := "  " + opt.Name
			if i == m.pickerSelection {
				label = pickerSelectedStyle.Render("▶ " + opt.Name + " ")
			}

			metaParts := []string{}
			if opt.ID != "" {
				metaParts = append(metaParts, opt.ID)
			}
			if opt.Source != "" {
				metaParts = append(metaParts, opt.Source)
			}

			if len(metaParts) > 0 {
				label = lipgloss.JoinHorizontal(lipgloss.Left, label, pickerModelMetaStyle.Render("  "+strings.Join(metaParts, "  •  ")))
			}
			items = append(items, label)
		}

		listContent = lipgloss.JoinVertical(lipgloss.Left, items...)
	}

	searchLine := "  " + m.search.View()

	profileLabel := m.bulkAssign.targetProfile
	if profileLabel == "" {
		profileLabel = m.activeProfile
	}
	info := "  " + mutedStyle.Render(fmt.Sprintf("Profile: %s | Target: %s (press Tab to switch)", profileLabel, targetLabel))
	help := "  " + cmdStyle.Render(" ↑↓ ") + "navigate " + cmdStyle.Render(" enter ") + "assign " + cmdStyle.Render(" tab ") + "switch target " + cmdStyle.Render(" esc ") + "back " + cmdStyle.Render(" type ") + "filter"

	body := lipgloss.JoinVertical(lipgloss.Left, titleBar, "", info, "", searchLine, "", listContent, "", help)
	return body
}

func (m Model) viewSwapProvider() string {
	titleBar := titleBarStyle.Render(" Swap Provider ")

	var content string
	if m.swapProvider.fromProvider == "" {
		providers := m.getProviderList()
		var listContent string
		if len(providers) == 0 {
			listContent = mutedStyle.Render("  No providers available")
		} else {
			items := make([]string, len(providers))
			for i, provider := range providers {
				line := "  " + provider
				if i == m.pickerSelection {
					line = pickerSelectedStyle.Render("▶ " + provider + " ")
				}
				items[i] = line
			}
			listContent = lipgloss.JoinVertical(lipgloss.Left, items...)
		}

		searchLine := "  " + m.search.View()
		info := "  " + mutedStyle.Render("Step 1: Select the provider to replace")
		help := "  " + cmdStyle.Render(" ↑↓ ") + "navigate " + cmdStyle.Render(" enter ") + "select " + cmdStyle.Render(" esc ") + "cancel " + cmdStyle.Render(" type ") + "filter"

		content = lipgloss.JoinVertical(lipgloss.Left, "", info, "", searchLine, "", listContent, "", help)
	} else {
		providers := m.getProviderList()
		var listContent string
		if len(providers) == 0 {
			listContent = mutedStyle.Render("  No providers available")
		} else {
			items := make([]string, 0, len(providers))
			for i, provider := range providers {
				if provider == m.swapProvider.fromProvider {
					continue
				}
				line := "  " + provider
				if i == m.pickerSelection {
					line = pickerSelectedStyle.Render("▶ " + provider + " ")
				}
				items = append(items, line)
			}
			listContent = lipgloss.JoinVertical(lipgloss.Left, items...)
		}

		searchLine := "  " + m.search.View()
		info := "  " + mutedStyle.Render(fmt.Sprintf("Step 2: Select target provider to replace '%s'", m.swapProvider.fromProvider))
		help := "  " + cmdStyle.Render(" ↑↓ ") + "navigate " + cmdStyle.Render(" enter ") + "select " + cmdStyle.Render(" esc ") + "cancel " + cmdStyle.Render(" type ") + "filter"

		content = lipgloss.JoinVertical(lipgloss.Left, "", info, "", searchLine, "", listContent, "", help)
	}

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, content)
}

type assignmentDiff struct {
	agentChanges    int
	categoryChanges int
}

func diffAssignments(original config.Snapshot, activeProfile string, agents, categories map[string]config.Assignment) assignmentDiff {
	resolved, err := original.Profiles.ResolveProfile(activeProfile)
	if err != nil {
		resolved = config.Profile{Agents: original.Active.Agents, Categories: original.Active.Categories}
	}
	diff := assignmentDiff{}
	keys := map[string]struct{}{}
	for key := range resolved.Agents {
		keys[key] = struct{}{}
	}
	for key := range agents {
		keys[key] = struct{}{}
	}
	for key := range keys {
		if resolved.Agents[key].Model != agents[key].Model {
			diff.agentChanges++
		}
	}
	keys = map[string]struct{}{}
	for key := range resolved.Categories {
		keys[key] = struct{}{}
	}
	for key := range categories {
		keys[key] = struct{}{}
	}
	for key := range keys {
		if resolved.Categories[key].Model != categories[key].Model {
			diff.categoryChanges++
		}
	}
	return diff
}

func diffProviders(original, current map[string]config.Provider) int {
	keys := map[string]struct{}{}
	for key := range original {
		keys[key] = struct{}{}
	}
	for key := range current {
		keys[key] = struct{}{}
	}
	changes := 0
	for key := range keys {
		if providerFingerprint(original[key]) != providerFingerprint(current[key]) {
			changes++
		}
	}
	return changes
}

func providerFingerprint(p config.Provider) string {
	modelKeys := make([]string, 0, len(p.Models))
	for key := range p.Models {
		modelKeys = append(modelKeys, key)
	}
	sort.Strings(modelKeys)
	optionKeys := make([]string, 0, len(p.Options))
	for key := range p.Options {
		optionKeys = append(optionKeys, key+"="+p.Options[key])
	}
	sort.Strings(optionKeys)
	return fmt.Sprintf("%s|%s|%v|%v", p.Name, p.NPM, optionKeys, modelKeys)
}

func cloneProviders(in map[string]config.Provider) map[string]config.Provider {
	out := map[string]config.Provider{}
	for key, value := range in {
		out[key] = value.Clone()
	}
	return out
}

func cloneAssignments(in map[string]config.Assignment) map[string]config.Assignment {
	out := map[string]config.Assignment{}
	for key, value := range in {
		out[key] = value.Clone()
	}
	return out
}

func filterStrings(values []string, query string) []string {
	if strings.TrimSpace(query) == "" {
		return values
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if containsFold(value, query) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func containsFold(value, query string) bool {
	if strings.TrimSpace(query) == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(strings.TrimSpace(query)))
}

func providerDisplayName(id string, configured map[string]config.Provider, builtins map[string]providercatalog.Template) string {
	if provider, ok := configured[id]; ok && provider.Name != "" {
		return provider.Name
	}
	if tmpl, ok := builtins[id]; ok {
		return tmpl.Name
	}
	if id == "" {
		return "Unknown"
	}
	return id
}

func choose(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func shorten(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen-3] + "..."
}

func maskedKey(value string) string {
	if strings.HasPrefix(value, "${") {
		return value
	}
	if len(value) <= 6 {
		return "***"
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

func sectionTitle(section Section) string {
	switch section {
	case SectionProfiles:
		return "Profiles"
	case SectionAgents:
		return "Agents"
	case SectionCategories:
		return "Categories"
	case SectionProviders:
		return "Providers"
	case SectionDefaults:
		return "Defaults"
	case SectionReview:
		return "Review"
	case SectionHelp:
		return "Help"
	case SectionSkills:
		return "Skills"
	default:
		return "Unknown"
	}
}

func keyMatches(msg tea.KeyMsg, target string) bool {
	return msg.String() == target
}

func moveIndex(current, length, delta int) int {
	if length <= 0 {
		return 0
	}
	current += delta
	if current < 0 {
		return 0
	}
	if current >= length {
		return length - 1
	}
	return current
}

func clamp(current, length int) int {
	if length <= 0 {
		return 0
	}
	if current < 0 {
		return 0
	}
	if current >= length {
		return length - 1
	}
	return current
}

var (
	background = lipgloss.Color("235")
	cyan       = lipgloss.Color("81")
	green      = lipgloss.Color("113")
	yellow     = lipgloss.Color("221")
	muted      = lipgloss.Color("245")
	red        = lipgloss.Color("204")
	cream      = lipgloss.Color("255")
	darkGray   = lipgloss.Color("240")
	black      = lipgloss.Color("0")

	titleBarStyle = lipgloss.NewStyle().
			Background(cyan).
			Foreground(black).
			Bold(true).
			Padding(0, 1).
			Width(100)

	statusBarStyle = lipgloss.NewStyle().
			Background(darkGray).
			Foreground(cream).
			Padding(0, 1)

	footerBarStyle = lipgloss.NewStyle().
			Background(background).
			Foreground(muted).
			Padding(0, 1)

	cmdStyle = lipgloss.NewStyle().
			Background(cyan).
			Foreground(black).
			Bold(true).
			Padding(0, 1)

	navBoxStyle = lipgloss.NewStyle().
			Background(background).
			BorderStyle(lipgloss.NormalBorder()).
			BorderRight(true).
			BorderForeground(darkGray)

	navActiveStyle = lipgloss.NewStyle().
			Background(cyan).
			Foreground(black).
			Bold(true)

	navInactiveStyle = lipgloss.NewStyle().
				Background(darkGray).
				Foreground(cream)

	navItemStyle = lipgloss.NewStyle().
			Foreground(cream).
			Bold(true)

	contentStyle = lipgloss.NewStyle().
			Background(background).
			PaddingLeft(1)

	headerStyle = lipgloss.NewStyle().
			Foreground(cream).
			Bold(true).
			MarginBottom(1)

	searchBoxStyle = lipgloss.NewStyle().
			Foreground(cream).
			MarginBottom(1)

	listPaneStyle = lipgloss.NewStyle().
			Background(background).
			BorderStyle(lipgloss.NormalBorder()).
			BorderRight(true).
			BorderForeground(darkGray).
			PaddingRight(1)

	detailPaneStyle = lipgloss.NewStyle().
			Background(background).
			PaddingLeft(1)

	listItemStyle = lipgloss.NewStyle().
			Foreground(cream).
			PaddingLeft(0)

	selectedItemStyle = lipgloss.NewStyle().
				Background(cyan).
				Foreground(black).
				Bold(true)

	detailTitleStyle = lipgloss.NewStyle().
				Foreground(cyan).
				Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(muted)

	okStyle = lipgloss.NewStyle().
		Foreground(green)

	errorStyle = lipgloss.NewStyle().
			Foreground(red)

	cursorStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Bold(true)

	selectedRowStyle = lipgloss.NewStyle().
				Background(cyan).
				Foreground(black).
				Bold(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(muted).
			Background(background).
			PaddingTop(1)

	defaultsPaneStyle = lipgloss.NewStyle().
				Background(background).
				PaddingLeft(1)

	reviewPaneStyle = lipgloss.NewStyle().
			Background(background).
			PaddingLeft(1)

	pickerHeaderStyle = lipgloss.NewStyle().
				Foreground(cream).
				Bold(true).
				MarginBottom(1)

	pickerSearchStyle = lipgloss.NewStyle().
				Foreground(cream).
				MarginBottom(1)

	pickerBodyStyle = lipgloss.NewStyle().
			Background(background).
			MarginBottom(1)

	pickerSelectedStyle = lipgloss.NewStyle().
				Background(cyan).
				Foreground(black).
				Bold(true)

	pickerProviderHeaderStyle = lipgloss.NewStyle().
					Foreground(green).
					Bold(true)

	pickerProviderMetaStyle = lipgloss.NewStyle().
				Foreground(yellow).
				Bold(true)

	pickerModelMetaStyle = lipgloss.NewStyle().
				Foreground(muted)

	pickerHelpStyle = lipgloss.NewStyle().
			Foreground(muted)
)
