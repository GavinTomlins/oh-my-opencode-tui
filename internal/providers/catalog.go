package providers

import (
	"sort"

	"github.com/gavintomlins/oh-my-opencode-tui/internal/config"
)

type Template struct {
	ID             string
	Name           string
	NPM            string
	DefaultBaseURL string
	RequiresAPIKey bool
	Local          bool
	Custom         bool
	Models         []TemplateModel
}

type TemplateModel struct {
	ID   string
	Name string
}

func Builtins() []Template {
	templates := []Template{
		{
			ID:             "opencode",
			Name:           "OpenCode AI",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "https://api.opencode.ai/v1",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"claude-opus-4-6", "Claude Opus 4.6"}, {"claude-haiku-4-5", "Claude Haiku 4.5"}, {"claude-sonnet-4-5", "Claude Sonnet 4.5"}, {"gemini-3-pro", "Gemini 3 Pro"}, {"gemini-3-flash", "Gemini 3 Flash"}, {"glm-4.7-free", "GLM 4.7 Free"}, {"kimi-k2.5-free", "Kimi K2.5 Free"}},
		},
		{
			ID:             "openai",
			Name:           "OpenAI",
			NPM:            "@ai-sdk/openai",
			DefaultBaseURL: "https://api.openai.com/v1",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"gpt-5.2", "GPT-5.2"}, {"gpt-5.3-codex", "GPT-5.3 Codex"}, {"gpt-4o", "GPT-4o"}},
		},
		{
			ID:             "anthropic",
			Name:           "Anthropic",
			NPM:            "@ai-sdk/anthropic",
			DefaultBaseURL: "https://api.anthropic.com",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"claude-haiku-3", "Claude Haiku 3"}, {"claude-haiku-3-5", "Claude Haiku 3.5"}, {"claude-haiku-3-5-latest", "Claude Haiku 3.5 (latest)"}, {"claude-haiku-4-5", "Claude Haiku 4.5"}, {"claude-haiku-4-5-latest", "Claude Haiku 4.5 (latest)"}, {"claude-opus-3", "Claude Opus 3"}, {"claude-opus-4", "Claude Opus 4"}, {"claude-opus-4-latest", "Claude Opus 4 (latest)"}, {"claude-opus-4-1", "Claude Opus 4.1"}, {"claude-opus-4-1-latest", "Claude Opus 4.1 (latest)"}, {"claude-opus-4-5", "Claude Opus 4.5"}, {"claude-opus-4-5-latest", "Claude Opus 4.5 (latest)"}, {"claude-opus-4-6", "Claude Opus 4.6"}, {"claude-sonnet-3", "Claude Sonnet 3"}, {"claude-sonnet-3-5", "Claude Sonnet 3.5"}, {"claude-sonnet-3-5-v2", "Claude Sonnet 3.5 v2"}, {"claude-sonnet-3-7", "Claude Sonnet 3.7"}, {"claude-sonnet-3-7-latest", "Claude Sonnet 3.7 (latest)"}, {"claude-sonnet-4", "Claude Sonnet 4"}},
		},
		{
			ID:             "google",
			Name:           "Google",
			NPM:            "@ai-sdk/google",
			DefaultBaseURL: "https://generativelanguage.googleapis.com",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"gemini-3-flash", "Gemini 3 Flash"}, {"gemini-3-pro", "Gemini 3 Pro"}},
		},
		{
			ID:             "openrouter",
			Name:           "OpenRouter",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "https://openrouter.ai/api/v1",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"anthropic/claude-3.7-sonnet", "Claude 3.7 Sonnet"}, {"anthropic/claude-opus-4", "Claude Opus 4"}, {"openai/gpt-5", "GPT-5"}, {"google/gemini-2.5-pro", "Gemini 2.5 Pro"}},
		},
		{
			ID:             "ollama",
			Name:           "Ollama (local)",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "http://localhost:11434/v1",
			Local:          true,
			Models:         []TemplateModel{{"qwen2.5-coder-14b-opt:latest", "Qwen 2.5 Coder 14B Optimized"}, {"qwen2.5-coder:14b", "Qwen 2.5 Coder 14B"}, {"qwen3.5-9b:latest", "Qwen 3.5 9B"}, {"llama3.1:8b", "Llama 3.1 8B"}},
		},
		{
			ID:             "lmstudio",
			Name:           "LM Studio",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "http://localhost:1234/v1",
			Local:          true,
			Models:         []TemplateModel{{"local-model", "Local Model"}},
		},
		{
			ID:             "custom-openai-compatible",
			Name:           "Custom OpenAI-compatible",
			NPM:            "@ai-sdk/openai-compatible",
			RequiresAPIKey: true,
			Custom:         true,
		},
	}

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})
	return templates
}

func BuiltinMap() map[string]Template {
	out := map[string]Template{}
	for _, item := range Builtins() {
		out[item.ID] = item
	}
	return out
}

func ProviderFromTemplate(t Template) config.Provider {
	provider := config.Provider{
		NPM:     t.NPM,
		Name:    t.Name,
		Options: map[string]string{},
		Models:  map[string]config.ProviderModel{},
	}
	if t.DefaultBaseURL != "" {
		provider.Options["baseURL"] = t.DefaultBaseURL
	}
	if t.Local && provider.Options["apiKey"] == "" {
		provider.Options["apiKey"] = "local"
	}
	for _, model := range t.Models {
		provider.Models[model.ID] = config.ProviderModel{Name: model.Name}
	}
	return provider
}
