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
			Models:         []TemplateModel{{"claude-opus-4-6", "Claude Opus 4.6"}, {"claude-haiku-4-5", "Claude Haiku 4.5"}, {"claude-sonnet-4-5", "Claude Sonnet 4.5"}, {"gemini-3-pro", "Gemini 3 Pro"}, {"gemini-3-flash", "Gemini 3 Flash"}, {"glm-4.7-free", "GLM 4.7 Free"}, {"glm-5", "GLM-5"}, {"kimi-k2.5-free", "Kimi K2.5 Free"}, {"kimi-k2.5-thinking", "Kimi K2.5 Thinking"}, {"minimax-m2.5", "MiniMax M2.5"}, {"gpt-5.4", "GPT-5.4"}, {"gpt-5.3-codex-spark", "GPT-5.3 Codex Spark"}},
		},
		{
			ID:             "openai",
			Name:           "OpenAI",
			NPM:            "@ai-sdk/openai",
			DefaultBaseURL: "https://api.openai.com/v1",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"codex-mini", "Codex Mini"}, {"gpt-5", "GPT-5"}, {"gpt-5-codex", "GPT-5 Codex"}, {"gpt-5.1", "GPT-5.1"}, {"gpt-5.1-codex", "GPT-5.1 Codex"}, {"gpt-5.2", "GPT-5.2"}, {"gpt-5.3-codex", "GPT-5.3 Codex"}, {"gpt-5.3-codex-spark", "GPT-5.3 Codex Spark"}, {"gpt-5.4", "GPT-5.4"}, {"gpt-4.1", "GPT-4.1"}, {"gpt-4.1-mini", "GPT-4.1 Mini"}, {"gpt-4o", "GPT-4o"}, {"gpt-4o-mini", "GPT-4o Mini"}, {"o4-mini", "o4 Mini"}},
		},
		{
			ID:             "anthropic",
			Name:           "Anthropic",
			NPM:            "@ai-sdk/anthropic",
			DefaultBaseURL: "https://api.anthropic.com",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"claude-haiku-3", "Claude Haiku 3"}, {"claude-haiku-3-5", "Claude Haiku 3.5"}, {"claude-haiku-3-5-latest", "Claude Haiku 3.5 (latest)"}, {"claude-haiku-4-5", "Claude Haiku 4.5"}, {"claude-haiku-4-5-latest", "Claude Haiku 4.5 (latest)"}, {"claude-opus-3", "Claude Opus 3"}, {"claude-opus-4", "Claude Opus 4"}, {"claude-opus-4-latest", "Claude Opus 4 (latest)"}, {"claude-opus-4-1", "Claude Opus 4.1"}, {"claude-opus-4-1-latest", "Claude Opus 4.1 (latest)"}, {"claude-opus-4-5", "Claude Opus 4.5"}, {"claude-opus-4-5-latest", "Claude Opus 4.5 (latest)"}, {"claude-opus-4-6", "Claude Opus 4.6"}, {"claude-sonnet-3", "Claude Sonnet 3"}, {"claude-sonnet-3-5", "Claude Sonnet 3.5"}, {"claude-sonnet-3-5-v2", "Claude Sonnet 3.5 v2"}, {"claude-sonnet-3-7", "Claude Sonnet 3.7"}, {"claude-sonnet-3-7-latest", "Claude Sonnet 3.7 (latest)"}, {"claude-sonnet-4", "Claude Sonnet 4"}, {"claude-sonnet-4-5", "Claude Sonnet 4.5"}, {"claude-sonnet-4-5-latest", "Claude Sonnet 4.5 (latest)"}, {"claude-sonnet-4-6", "Claude Sonnet 4.6"}},
		},
		{
			ID:             "google",
			Name:           "Google",
			NPM:            "@ai-sdk/google",
			DefaultBaseURL: "https://generativelanguage.googleapis.com",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"gemini-2.5-flash", "Gemini 2.5 Flash"}, {"gemini-2.5-pro", "Gemini 2.5 Pro"}, {"gemini-3-flash", "Gemini 3 Flash"}, {"gemini-3-pro", "Gemini 3 Pro"}},
		},
		{
			ID:             "chatbot",
			Name:           "Chatbot",
			NPM:            "@ai-sdk/openai-compatible",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"chatbot-coding", "Chatbot-Coding"}, {"glm4-chatbot", "GLM4-Chatbot"}, {"gpt-codex-chatbot", "GPT-Codex-Chatbot"}},
		},
		{
			ID:             "deepseek",
			Name:           "DeepSeek",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "https://api.deepseek.com/v1",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"deepseek-chat", "DeepSeek Chat"}, {"deepseek-coder", "DeepSeek Coder"}, {"deepseek-reasoner", "DeepSeek Reasoner"}},
		},
		{
			ID:             "kimi",
			Name:           "Kimi For Coding",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "https://api.moonshot.ai/v1",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"kimi-k2", "Kimi K2"}, {"kimi-k2-thinking", "Kimi K2 Thinking"}, {"kimi-k2.5", "Kimi K2.5"}, {"kimi-k2.5-coding", "Kimi K2.5 Coding"}, {"kimi-k2.5-thinking", "Kimi K2.5 Thinking"}},
		},
		{
			ID:             "litellm",
			Name:           "LiteLLM",
			NPM:            "@ai-sdk/openai-compatible",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"deepseek-chat", "DeepSeek Chat"}, {"gpt-5", "GPT-5"}, {"claude-sonnet-4", "Claude Sonnet 4"}, {"gemini-2.5-pro", "Gemini 2.5 Pro"}},
		},
		{
			ID:             "minimax",
			Name:           "MiniMax",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "https://api.minimax.io/v1",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"minimax-m2.5", "MiniMax M2.5"}, {"minimax-text-01", "MiniMax Text 01"}},
		},
		{
			ID:             "openrouter",
			Name:           "OpenRouter",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "https://openrouter.ai/api/v1",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"anthropic/claude-3.7-sonnet", "Claude 3.7 Sonnet"}, {"anthropic/claude-3.7-sonnet:thinking", "Claude 3.7 Sonnet Thinking"}, {"anthropic/claude-haiku-3.5", "Claude Haiku 3.5"}, {"anthropic/claude-opus-4", "Claude Opus 4"}, {"anthropic/claude-opus-4.1", "Claude Opus 4.1"}, {"anthropic/claude-sonnet-4", "Claude Sonnet 4"}, {"anthropic/claude-sonnet-4.5", "Claude Sonnet 4.5"}, {"deepseek/deepseek-chat", "DeepSeek Chat"}, {"google/gemini-2.5-pro", "Gemini 2.5 Pro"}, {"google/gemini-3-pro", "Gemini 3 Pro"}, {"minimax/minimax-m2.5", "MiniMax M2.5"}, {"moonshotai/kimi-k2.5", "Kimi K2.5"}, {"mistralai/codestral", "Codestral"}, {"openai/gpt-5", "GPT-5"}, {"openai/gpt-5.4", "GPT-5.4"}, {"openrouter/aurora-alpha", "Aurora Alpha"}, {"qwen/qwen-2.5-coder-32b-instruct", "Qwen 2.5 Coder 32B"}, {"zhipu/glm-5", "GLM-5"}},
		},
		{
			ID:             "ollama",
			Name:           "Ollama (local)",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "http://localhost:11434/v1",
			Local:          true,
			Models:         []TemplateModel{{"kimi-k2.5-cloud:latest", "Kimi K2.5 Cloud"}, {"codellama:13b", "Code Llama 13B"}, {"deepseek-r1:14b", "DeepSeek R1 14B"}, {"llama3.1:8b", "Llama 3.1 8B"}, {"llama3.1:70b", "Llama 3.1 70B"}, {"llama3.2:3b", "Llama 3.2 3B"}, {"llama3.2:11b", "Llama 3.2 11B"}, {"qwen2.5:14b", "Qwen 2.5 14B"}, {"qwen2.5-coder-14b-opt:latest", "Qwen 2.5 Coder 14B Optimized"}, {"qwen2.5-coder:14b", "Qwen 2.5 Coder 14B"}, {"qwen2.5-coder:32b", "Qwen 2.5 Coder 32B"}, {"qwen3.5-9b:latest", "Qwen 3.5 9B"}, {"qwen3.5:14b", "Qwen 3.5 14B"}},
		},
		{
			ID:             "zhipu",
			Name:           "Zhipu",
			NPM:            "@ai-sdk/openai-compatible",
			DefaultBaseURL: "https://open.bigmodel.cn/api/paas/v4",
			RequiresAPIKey: true,
			Models:         []TemplateModel{{"glm-4.7", "GLM 4.7"}, {"glm-4.7-air", "GLM 4.7 Air"}, {"glm-5", "GLM-5"}},
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
