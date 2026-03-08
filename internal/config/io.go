package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tailscale/hujson"
)

func Load(paths Paths) (Snapshot, error) {
	active, err := readActiveConfig(paths.ActiveConfig)
	if err != nil {
		return Snapshot{}, err
	}
	profiles, err := readProfiles(paths.Profiles)
	if err != nil {
		return Snapshot{}, err
	}
	providers, err := readProviders(paths.Providers)
	if err != nil {
		return Snapshot{}, err
	}
	uiState, err := readUIState(paths.UIState)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{Paths: paths, Active: active, Profiles: profiles, Providers: providers, UIState: uiState}, nil
}

func ParseJSONC(data []byte, dest any) error {
	standard, err := hujson.Standardize(data)
	if err != nil {
		return fmt.Errorf("standardize jsonc: %w", err)
	}
	if err := json.Unmarshal(standard, dest); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func readActiveConfig(path string) (ActiveConfig, error) {
	var cfg ActiveConfig
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg.Agents = map[string]Assignment{}
		cfg.Categories = map[string]Assignment{}
		cfg.Extra = map[string]json.RawMessage{}
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ActiveConfig{}, fmt.Errorf("read active config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ActiveConfig{}, fmt.Errorf("parse active config: %w", err)
	}
	return cfg, nil
}

func readProfiles(path string) (ProfilesDocument, error) {
	var doc ProfilesDocument
	data, err := os.ReadFile(path)
	if err != nil {
		return ProfilesDocument{}, fmt.Errorf("read profiles: %w", err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return ProfilesDocument{}, fmt.Errorf("parse profiles: %w", err)
	}
	return doc, nil
}

func readProviders(path string) (ProviderDocument, error) {
	var doc ProviderDocument
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderDocument{}, fmt.Errorf("read providers: %w", err)
	}
	if err := ParseJSONC(data, &doc); err != nil {
		return ProviderDocument{}, fmt.Errorf("parse providers: %w", err)
	}
	return doc, nil
}

func readUIState(path string) (UIState, error) {
	var state UIState
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return state, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return UIState{}, fmt.Errorf("read ui state: %w", err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return UIState{}, fmt.Errorf("parse ui state: %w", err)
	}
	return state, nil
}

type SaveInput struct {
	Snapshot      Snapshot
	ActiveProfile string
	Agents        map[string]Assignment
	Categories    map[string]Assignment
	Providers     map[string]Provider
	DefaultModel  string
}

func Save(input SaveInput) error {
	paths := input.Snapshot.Paths
	if err := os.MkdirAll(filepath.Dir(paths.UIState), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.MkdirAll(paths.BackupDir, 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	profiles := input.Snapshot.Profiles
	activeProfile := input.ActiveProfile
	if activeProfile == "" {
		activeProfile = profiles.ActiveProfile
	}
	if activeProfile == "" {
		activeProfile = "default"
	}
	profile := profiles.Profiles[activeProfile]
	if profile.Name == "" {
		profile.Name = activeProfile
	}
	if profile.Description == "" {
		profile.Description = "Managed by oh-my-opencode-tui"
	}
	profile.Agents = cloneAssignments(input.Agents)
	profile.Categories = cloneAssignments(input.Categories)
	profiles.Profiles[activeProfile] = profile
	profiles.ActiveProfile = activeProfile

	if err := backupFile(paths.Profiles, paths.BackupDir); err != nil {
		return err
	}
	if err := writeJSON(paths.Profiles, profiles); err != nil {
		return fmt.Errorf("write profiles: %w", err)
	}

	resolved, err := profiles.ResolveProfile(activeProfile)
	if err != nil {
		return fmt.Errorf("resolve profile %q: %w", activeProfile, err)
	}
	active := input.Snapshot.Active
	if active.Schema == "" {
		active.Schema = paths.DefaultSchemaURL
	}
	active.Agents = cloneAssignments(resolved.Agents)
	active.Categories = cloneAssignments(resolved.Categories)

	if err := backupFile(paths.ActiveConfig, paths.BackupDir); err != nil {
		return err
	}
	if err := writeJSON(paths.ActiveConfig, active); err != nil {
		return fmt.Errorf("write active config: %w", err)
	}

	providerDoc := input.Snapshot.Providers
	providerDoc.Provider = map[string]Provider{}
	for key, value := range input.Providers {
		providerDoc.Provider[key] = value.Clone()
	}
	if err := backupFile(paths.Providers, paths.BackupDir); err != nil {
		return err
	}
	if err := writeJSON(paths.Providers, providerDoc); err != nil {
		return fmt.Errorf("write providers: %w", err)
	}

	if err := writeJSON(paths.UIState, UIState{DefaultModel: input.DefaultModel}); err != nil {
		return fmt.Errorf("write ui state: %w", err)
	}

	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func backupFile(path, backupDir string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("backup read %s: %w", path, err)
	}
	name := filepath.Base(path)
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s.%s.bak", name, timestamp))
	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		return fmt.Errorf("write backup %s: %w", backupPath, err)
	}
	return nil
}
