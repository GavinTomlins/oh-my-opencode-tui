package config

import (
	"encoding/json"
	"fmt"
	"sort"
)

type Assignment struct {
	Model   string                     `json:"model,omitempty"`
	Variant string                     `json:"variant,omitempty"`
	Prompt  string                     `json:"prompt,omitempty"`
	Extra   map[string]json.RawMessage `json:"-"`
}

func (a *Assignment) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	a.Extra = map[string]json.RawMessage{}
	for key, value := range raw {
		switch key {
		case "model":
			if err := json.Unmarshal(value, &a.Model); err != nil {
				return fmt.Errorf("decode assignment model: %w", err)
			}
		case "variant":
			if err := json.Unmarshal(value, &a.Variant); err != nil {
				return fmt.Errorf("decode assignment variant: %w", err)
			}
		case "prompt":
			if err := json.Unmarshal(value, &a.Prompt); err != nil {
				return fmt.Errorf("decode assignment prompt: %w", err)
			}
		default:
			a.Extra[key] = value
		}
	}

	return nil
}

func (a Assignment) MarshalJSON() ([]byte, error) {
	raw := map[string]json.RawMessage{}
	for key, value := range a.Extra {
		raw[key] = value
	}

	if a.Model != "" {
		b, _ := json.Marshal(a.Model)
		raw["model"] = b
	}
	if a.Variant != "" {
		b, _ := json.Marshal(a.Variant)
		raw["variant"] = b
	}
	if a.Prompt != "" {
		b, _ := json.Marshal(a.Prompt)
		raw["prompt"] = b
	}

	return json.Marshal(raw)
}

func (a Assignment) Clone() Assignment {
	clone := Assignment{
		Model:   a.Model,
		Variant: a.Variant,
		Prompt:  a.Prompt,
		Extra:   cloneRawMap(a.Extra),
	}
	return clone
}

type ActiveConfig struct {
	Schema     string                     `json:"$schema,omitempty"`
	Agents     map[string]Assignment      `json:"agents,omitempty"`
	Categories map[string]Assignment      `json:"categories,omitempty"`
	Extra      map[string]json.RawMessage `json:"-"`
}

func (c *ActiveConfig) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.Extra = map[string]json.RawMessage{}
	for key, value := range raw {
		switch key {
		case "$schema":
			if err := json.Unmarshal(value, &c.Schema); err != nil {
				return err
			}
		case "agents":
			if err := json.Unmarshal(value, &c.Agents); err != nil {
				return err
			}
		case "categories":
			if err := json.Unmarshal(value, &c.Categories); err != nil {
				return err
			}
		default:
			c.Extra[key] = value
		}
	}

	if c.Agents == nil {
		c.Agents = map[string]Assignment{}
	}
	if c.Categories == nil {
		c.Categories = map[string]Assignment{}
	}

	return nil
}

func (c ActiveConfig) MarshalJSON() ([]byte, error) {
	raw := map[string]json.RawMessage{}
	for key, value := range c.Extra {
		raw[key] = value
	}
	if c.Schema != "" {
		b, _ := json.Marshal(c.Schema)
		raw["$schema"] = b
	}
	if c.Agents != nil {
		b, err := json.Marshal(c.Agents)
		if err != nil {
			return nil, err
		}
		raw["agents"] = b
	}
	if c.Categories != nil {
		b, err := json.Marshal(c.Categories)
		if err != nil {
			return nil, err
		}
		raw["categories"] = b
	}
	return json.Marshal(raw)
}

type Profile struct {
	Name        string                     `json:"name,omitempty"`
	Description string                     `json:"description,omitempty"`
	Extends     string                     `json:"extends,omitempty"`
	Agents      map[string]Assignment      `json:"agents,omitempty"`
	Categories  map[string]Assignment      `json:"categories,omitempty"`
	Settings    map[string]any             `json:"settings,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

func (p *Profile) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.Extra = map[string]json.RawMessage{}
	for key, value := range raw {
		switch key {
		case "name":
			if err := json.Unmarshal(value, &p.Name); err != nil {
				return err
			}
		case "description":
			if err := json.Unmarshal(value, &p.Description); err != nil {
				return err
			}
		case "extends":
			if err := json.Unmarshal(value, &p.Extends); err != nil {
				return err
			}
		case "agents":
			if err := json.Unmarshal(value, &p.Agents); err != nil {
				return err
			}
		case "categories":
			if err := json.Unmarshal(value, &p.Categories); err != nil {
				return err
			}
		case "settings":
			if err := json.Unmarshal(value, &p.Settings); err != nil {
				return err
			}
		default:
			p.Extra[key] = value
		}
	}

	if p.Agents == nil {
		p.Agents = map[string]Assignment{}
	}
	if p.Categories == nil {
		p.Categories = map[string]Assignment{}
	}
	if p.Settings == nil {
		p.Settings = map[string]any{}
	}

	return nil
}

func (p Profile) MarshalJSON() ([]byte, error) {
	raw := map[string]json.RawMessage{}
	for key, value := range p.Extra {
		raw[key] = value
	}
	marshalInto(raw, "name", p.Name)
	marshalInto(raw, "description", p.Description)
	marshalInto(raw, "extends", p.Extends)
	marshalInto(raw, "agents", p.Agents)
	marshalInto(raw, "categories", p.Categories)
	marshalInto(raw, "settings", p.Settings)
	return json.Marshal(raw)
}

func (p Profile) Clone() Profile {
	return Profile{
		Name:        p.Name,
		Description: p.Description,
		Extends:     p.Extends,
		Agents:      cloneAssignments(p.Agents),
		Categories:  cloneAssignments(p.Categories),
		Settings:    cloneAnyMap(p.Settings),
		Extra:       cloneRawMap(p.Extra),
	}
}

type ModelDefinition struct {
	Provider      string   `json:"provider,omitempty"`
	Name          string   `json:"name,omitempty"`
	MaxTokens     int      `json:"maxTokens,omitempty"`
	ContextWindow int      `json:"contextWindow,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	Speed         string   `json:"speed,omitempty"`
	Local         bool     `json:"local,omitempty"`
}

type ProfilesDocument struct {
	Version          string                     `json:"version,omitempty"`
	ActiveProfile    string                     `json:"activeProfile,omitempty"`
	Profiles         map[string]Profile         `json:"profiles,omitempty"`
	ModelDefinitions map[string]ModelDefinition `json:"modelDefinitions,omitempty"`
	Extra            map[string]json.RawMessage `json:"-"`
}

func (d *ProfilesDocument) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	d.Extra = map[string]json.RawMessage{}
	for key, value := range raw {
		switch key {
		case "version":
			if err := json.Unmarshal(value, &d.Version); err != nil {
				return err
			}
		case "activeProfile":
			if err := json.Unmarshal(value, &d.ActiveProfile); err != nil {
				return err
			}
		case "profiles":
			if err := json.Unmarshal(value, &d.Profiles); err != nil {
				return err
			}
		case "modelDefinitions":
			if err := json.Unmarshal(value, &d.ModelDefinitions); err != nil {
				return err
			}
		default:
			d.Extra[key] = value
		}
	}

	if d.Profiles == nil {
		d.Profiles = map[string]Profile{}
	}
	if d.ModelDefinitions == nil {
		d.ModelDefinitions = map[string]ModelDefinition{}
	}

	return nil
}

func (d ProfilesDocument) MarshalJSON() ([]byte, error) {
	raw := map[string]json.RawMessage{}
	for key, value := range d.Extra {
		raw[key] = value
	}
	marshalInto(raw, "version", d.Version)
	marshalInto(raw, "activeProfile", d.ActiveProfile)
	marshalInto(raw, "profiles", d.Profiles)
	marshalInto(raw, "modelDefinitions", d.ModelDefinitions)
	return json.Marshal(raw)
}

func (d ProfilesDocument) ResolveProfile(name string) (Profile, error) {
	visited := map[string]bool{}
	var resolve func(string) (Profile, error)

	resolve = func(profileName string) (Profile, error) {
		profile, ok := d.Profiles[profileName]
		if !ok {
			return Profile{}, fmt.Errorf("profile %q not found", profileName)
		}
		if visited[profileName] {
			return Profile{}, fmt.Errorf("cyclic profile inheritance detected at %q", profileName)
		}
		visited[profileName] = true
		defer delete(visited, profileName)

		resolved := profile.Clone()
		if profile.Extends == "" {
			return resolved, nil
		}

		parent, err := resolve(profile.Extends)
		if err != nil {
			return Profile{}, err
		}
		return mergeProfiles(parent, resolved), nil
	}

	return resolve(name)
}

type ProviderModel struct {
	Name  string                     `json:"name,omitempty"`
	Extra map[string]json.RawMessage `json:"-"`
}

func (m *ProviderModel) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Extra = map[string]json.RawMessage{}
	for key, value := range raw {
		switch key {
		case "name":
			if err := json.Unmarshal(value, &m.Name); err != nil {
				return err
			}
		default:
			m.Extra[key] = value
		}
	}
	return nil
}

func (m ProviderModel) MarshalJSON() ([]byte, error) {
	raw := map[string]json.RawMessage{}
	for key, value := range m.Extra {
		raw[key] = value
	}
	marshalInto(raw, "name", m.Name)
	return json.Marshal(raw)
}

func (m ProviderModel) Clone() ProviderModel {
	return ProviderModel{Name: m.Name, Extra: cloneRawMap(m.Extra)}
}

type Provider struct {
	NPM     string                     `json:"npm,omitempty"`
	Name    string                     `json:"name,omitempty"`
	Options map[string]string          `json:"options,omitempty"`
	Models  map[string]ProviderModel   `json:"models,omitempty"`
	Extra   map[string]json.RawMessage `json:"-"`
}

func (p *Provider) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Extra = map[string]json.RawMessage{}
	for key, value := range raw {
		switch key {
		case "npm":
			if err := json.Unmarshal(value, &p.NPM); err != nil {
				return err
			}
		case "name":
			if err := json.Unmarshal(value, &p.Name); err != nil {
				return err
			}
		case "options":
			if err := json.Unmarshal(value, &p.Options); err != nil {
				return err
			}
		case "models":
			if err := json.Unmarshal(value, &p.Models); err != nil {
				return err
			}
		default:
			p.Extra[key] = value
		}
	}
	if p.Options == nil {
		p.Options = map[string]string{}
	}
	if p.Models == nil {
		p.Models = map[string]ProviderModel{}
	}
	return nil
}

func (p Provider) MarshalJSON() ([]byte, error) {
	raw := map[string]json.RawMessage{}
	for key, value := range p.Extra {
		raw[key] = value
	}
	marshalInto(raw, "npm", p.NPM)
	marshalInto(raw, "name", p.Name)
	marshalInto(raw, "options", p.Options)
	marshalInto(raw, "models", p.Models)
	return json.Marshal(raw)
}

func (p Provider) Clone() Provider {
	cloneModels := map[string]ProviderModel{}
	for key, value := range p.Models {
		cloneModels[key] = value.Clone()
	}
	cloneOptions := map[string]string{}
	for key, value := range p.Options {
		cloneOptions[key] = value
	}
	return Provider{
		NPM:     p.NPM,
		Name:    p.Name,
		Options: cloneOptions,
		Models:  cloneModels,
		Extra:   cloneRawMap(p.Extra),
	}
}

type ProviderDocument struct {
	Provider map[string]Provider        `json:"provider,omitempty"`
	Extra    map[string]json.RawMessage `json:"-"`
}

func (d *ProviderDocument) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	d.Extra = map[string]json.RawMessage{}
	for key, value := range raw {
		switch key {
		case "provider":
			if err := json.Unmarshal(value, &d.Provider); err != nil {
				return err
			}
		default:
			d.Extra[key] = value
		}
	}
	if d.Provider == nil {
		d.Provider = map[string]Provider{}
	}
	return nil
}

func (d ProviderDocument) MarshalJSON() ([]byte, error) {
	raw := map[string]json.RawMessage{}
	for key, value := range d.Extra {
		raw[key] = value
	}
	marshalInto(raw, "provider", d.Provider)
	return json.Marshal(raw)
}

type UIState struct {
	DefaultModel string `json:"defaultModel,omitempty"`
}

type Snapshot struct {
	Paths     Paths
	Active    ActiveConfig
	Profiles  ProfilesDocument
	Providers ProviderDocument
	UIState   UIState
}

func (s Snapshot) AgentKeys() []string {
	keys := map[string]struct{}{}
	for key := range s.Active.Agents {
		keys[key] = struct{}{}
	}
	for _, profile := range s.Profiles.Profiles {
		for key := range profile.Agents {
			keys[key] = struct{}{}
		}
	}
	return sortKeys(keys)
}

func (s Snapshot) CategoryKeys() []string {
	keys := map[string]struct{}{}
	for key := range s.Active.Categories {
		keys[key] = struct{}{}
	}
	for _, profile := range s.Profiles.Profiles {
		for key := range profile.Categories {
			keys[key] = struct{}{}
		}
	}
	return sortKeys(keys)
}

func mergeProfiles(parent, child Profile) Profile {
	merged := parent.Clone()
	if child.Name != "" {
		merged.Name = child.Name
	}
	if child.Description != "" {
		merged.Description = child.Description
	}
	if child.Extends != "" {
		merged.Extends = child.Extends
	}
	if merged.Agents == nil {
		merged.Agents = map[string]Assignment{}
	}
	for key, value := range child.Agents {
		merged.Agents[key] = value.Clone()
	}
	if merged.Categories == nil {
		merged.Categories = map[string]Assignment{}
	}
	for key, value := range child.Categories {
		merged.Categories[key] = value.Clone()
	}
	if merged.Settings == nil {
		merged.Settings = map[string]any{}
	}
	for key, value := range child.Settings {
		merged.Settings[key] = value
	}
	for key, value := range child.Extra {
		merged.Extra[key] = value
	}
	return merged
}

func cloneAssignments(in map[string]Assignment) map[string]Assignment {
	if in == nil {
		return map[string]Assignment{}
	}
	out := make(map[string]Assignment, len(in))
	for key, value := range in {
		out[key] = value.Clone()
	}
	return out
}

func cloneRawMap(in map[string]json.RawMessage) map[string]json.RawMessage {
	if in == nil {
		return map[string]json.RawMessage{}
	}
	out := make(map[string]json.RawMessage, len(in))
	for key, value := range in {
		clone := make([]byte, len(value))
		copy(clone, value)
		out[key] = clone
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func marshalInto(raw map[string]json.RawMessage, key string, value any) {
	if value == nil {
		return
	}
	switch v := value.(type) {
	case string:
		if v == "" {
			return
		}
	case map[string]Assignment:
		if len(v) == 0 {
			return
		}
	case map[string]Profile:
		if len(v) == 0 {
			return
		}
	case map[string]ModelDefinition:
		if len(v) == 0 {
			return
		}
	case map[string]string:
		if len(v) == 0 {
			return
		}
	case map[string]ProviderModel:
		if len(v) == 0 {
			return
		}
	case map[string]Provider:
		if len(v) == 0 {
			return
		}
	case map[string]any:
		if len(v) == 0 {
			return
		}
	case []string:
		if len(v) == 0 {
			return
		}
	}
	b, err := json.Marshal(value)
	if err == nil {
		raw[key] = b
	}
}

func sortKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
