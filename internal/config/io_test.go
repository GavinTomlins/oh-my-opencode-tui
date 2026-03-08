package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseJSONCProviderDocument(t *testing.T) {
	data := []byte(`{
	  // provider config
	  "provider": {
	    "openai": {
	      "name": "OpenAI",
	      "options": {
	        "apiKey": "${OPENAI_API_KEY}"
	      },
	      "models": {
	        "gpt-5.2": {"name": "GPT-5.2"}
	      }
	    }
	  }
	}`)

	var doc ProviderDocument
	if err := ParseJSONC(data, &doc); err != nil {
		t.Fatalf("ParseJSONC returned error: %v", err)
	}

	provider, ok := doc.Provider["openai"]
	if !ok {
		t.Fatalf("expected openai provider to exist")
	}
	if provider.Options["apiKey"] != "${OPENAI_API_KEY}" {
		t.Fatalf("unexpected api key placeholder: %q", provider.Options["apiKey"])
	}
}

func TestResolveProfileInheritance(t *testing.T) {
	doc := ProfilesDocument{
		ActiveProfile: "custom",
		Profiles: map[string]Profile{
			"default": {
				Agents: map[string]Assignment{
					"atlas": {Model: "opencode/base-model"},
				},
				Categories: map[string]Assignment{
					"quick": {Model: "opencode/quick-model"},
				},
			},
			"custom": {
				Extends: "default",
				Agents: map[string]Assignment{
					"atlas":  {Model: "openai/override-model"},
					"oracle": {Model: "opencode/oracle-model"},
				},
			},
		},
	}

	resolved, err := doc.ResolveProfile("custom")
	if err != nil {
		t.Fatalf("ResolveProfile returned error: %v", err)
	}
	if resolved.Agents["atlas"].Model != "openai/override-model" {
		t.Fatalf("expected child override, got %q", resolved.Agents["atlas"].Model)
	}
	if resolved.Agents["oracle"].Model != "opencode/oracle-model" {
		t.Fatalf("expected child-added agent, got %q", resolved.Agents["oracle"].Model)
	}
	if resolved.Categories["quick"].Model != "opencode/quick-model" {
		t.Fatalf("expected inherited category, got %q", resolved.Categories["quick"].Model)
	}
}

func TestSavePreservesUnmanagedTopLevelKeys(t *testing.T) {
	tempDir := t.TempDir()
	paths := Paths{
		ActiveConfig:     filepath.Join(tempDir, "oh-my-opencode.json"),
		Profiles:         filepath.Join(tempDir, "config", "oh-my-opencode-profiles.json"),
		Providers:        filepath.Join(tempDir, "opencode.json"),
		UIState:          filepath.Join(tempDir, "config", "oh-my-opencode-tui.json"),
		BackupDir:        filepath.Join(tempDir, "config", "backups"),
		DefaultSchemaURL: "https://example.invalid/schema.json",
	}

	active := ActiveConfig{
		Schema:     "$schema",
		Agents:     map[string]Assignment{"atlas": {Model: "old/model"}},
		Categories: map[string]Assignment{"quick": {Model: "old/category"}},
		Extra: map[string]json.RawMessage{
			"terminal_multiplexer": json.RawMessage(`{"enabled":true,"type":"auto"}`),
		},
	}
	profiles := ProfilesDocument{
		ActiveProfile: "default",
		Profiles: map[string]Profile{
			"default": {
				Agents:     map[string]Assignment{"atlas": {Model: "old/model"}},
				Categories: map[string]Assignment{"quick": {Model: "old/category"}},
			},
		},
	}
	providers := ProviderDocument{Provider: map[string]Provider{}}

	if err := os.MkdirAll(filepath.Dir(paths.ActiveConfig), 0o755); err != nil {
		t.Fatalf("mkdir active dir: %v", err)
	}
	if err := writeJSON(paths.ActiveConfig, active); err != nil {
		t.Fatalf("write active fixture: %v", err)
	}
	if err := writeJSON(paths.Profiles, profiles); err != nil {
		t.Fatalf("write profile fixture: %v", err)
	}
	if err := writeJSON(paths.Providers, providers); err != nil {
		t.Fatalf("write providers fixture: %v", err)
	}

	snapshot := Snapshot{Paths: paths, Active: active, Profiles: profiles, Providers: providers}
	err := Save(SaveInput{
		Snapshot:      snapshot,
		ActiveProfile: "default",
		Agents:        map[string]Assignment{"atlas": {Model: "new/model"}},
		Categories:    map[string]Assignment{"quick": {Model: "new/category"}},
		Providers:     map[string]Provider{},
		DefaultModel:  "new/model",
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	data, err := os.ReadFile(paths.ActiveConfig)
	if err != nil {
		t.Fatalf("read saved active config: %v", err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal saved active config: %v", err)
	}
	if _, ok := saved["terminal_multiplexer"]; !ok {
		t.Fatalf("expected unmanaged top-level key to be preserved")
	}

	var savedActive ActiveConfig
	if err := json.Unmarshal(data, &savedActive); err != nil {
		t.Fatalf("unmarshal saved active struct: %v", err)
	}
	if savedActive.Agents["atlas"].Model != "new/model" {
		t.Fatalf("expected saved agent model, got %q", savedActive.Agents["atlas"].Model)
	}
	if savedActive.Categories["quick"].Model != "new/category" {
		t.Fatalf("expected saved category model, got %q", savedActive.Categories["quick"].Model)
	}
}
