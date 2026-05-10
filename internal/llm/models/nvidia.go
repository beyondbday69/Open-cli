package models

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/spf13/viper"
)

const (
	ProviderNvidia ModelProvider = "nvidia"

	nvidiaBaseURL   = "https://integrate.api.nvidia.com/v1"
	nvidiaModelsURL = nvidiaBaseURL + "/models"

	// Static fallback model IDs
	NvidiaDeepSeekR1_0528  ModelID = "nvidia.deepseek-ai/deepseek-r1-0528"
	NvidiaDeepSeekV3_0324  ModelID = "nvidia.deepseek-ai/deepseek-v3-0324"
	NvidiaDeepSeekR1       ModelID = "nvidia.deepseek-ai/deepseek-r1"
	NvidiaLlama33_70B      ModelID = "nvidia.meta/llama-3.3-70b-instruct"
	NvidiaLlama31_8B       ModelID = "nvidia.meta/llama-3.1-8b-instruct"
	NvidiaLlama31_70B      ModelID = "nvidia.meta/llama-3.1-70b-instruct"
	NvidiaMistral7B        ModelID = "nvidia.mistralai/mistral-7b-instruct-v0.3"
	NvidiaMixtral8x7B      ModelID = "nvidia.mistralai/mixtral-8x7b-instruct-v0.1"
	NvidiaQwen2_5_72B      ModelID = "nvidia.qwen/qwen2.5-72b-instruct"
	NvidiaQwen2_5Coder_32B ModelID = "nvidia.qwen/qwen2.5-coder-32b-instruct"
)

// NvidiaModels is the static fallback set. init() merges live-fetched models
// on top so you always have something usable even without connectivity.
var NvidiaModels = map[ModelID]Model{
	NvidiaDeepSeekR1_0528: {
		ID: NvidiaDeepSeekR1_0528, Name: "DeepSeek R1 0528 (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "deepseek-ai/deepseek-r1-0528",
		CostPer1MIn: 4.0, CostPer1MOut: 8.0,
		ContextWindow: 128_000, DefaultMaxTokens: 8192, CanReason: true,
	},
	NvidiaDeepSeekV3_0324: {
		ID: NvidiaDeepSeekV3_0324, Name: "DeepSeek V3 0324 (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "deepseek-ai/deepseek-v3-0324",
		CostPer1MIn: 0.27, CostPer1MOut: 1.1,
		ContextWindow: 128_000, DefaultMaxTokens: 8192,
	},
	NvidiaDeepSeekR1: {
		ID: NvidiaDeepSeekR1, Name: "DeepSeek R1 (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "deepseek-ai/deepseek-r1",
		CostPer1MIn: 4.0, CostPer1MOut: 8.0,
		ContextWindow: 128_000, DefaultMaxTokens: 8192, CanReason: true,
	},
	NvidiaLlama33_70B: {
		ID: NvidiaLlama33_70B, Name: "Llama 3.3 70B (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "meta/llama-3.3-70b-instruct",
		CostPer1MIn: 0.12, CostPer1MOut: 0.12,
		ContextWindow: 128_000, DefaultMaxTokens: 8192,
	},
	NvidiaLlama31_8B: {
		ID: NvidiaLlama31_8B, Name: "Llama 3.1 8B (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "meta/llama-3.1-8b-instruct",
		CostPer1MIn: 0.04, CostPer1MOut: 0.04,
		ContextWindow: 128_000, DefaultMaxTokens: 8192,
	},
	NvidiaLlama31_70B: {
		ID: NvidiaLlama31_70B, Name: "Llama 3.1 70B (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "meta/llama-3.1-70b-instruct",
		CostPer1MIn: 0.12, CostPer1MOut: 0.12,
		ContextWindow: 128_000, DefaultMaxTokens: 8192,
	},
	NvidiaMistral7B: {
		ID: NvidiaMistral7B, Name: "Mistral 7B v0.3 (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "mistralai/mistral-7b-instruct-v0.3",
		CostPer1MIn: 0.04, CostPer1MOut: 0.04,
		ContextWindow: 32_768, DefaultMaxTokens: 4096,
	},
	NvidiaMixtral8x7B: {
		ID: NvidiaMixtral8x7B, Name: "Mixtral 8x7B (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "mistralai/mixtral-8x7b-instruct-v0.1",
		CostPer1MIn: 0.24, CostPer1MOut: 0.24,
		ContextWindow: 32_768, DefaultMaxTokens: 4096,
	},
	NvidiaQwen2_5_72B: {
		ID: NvidiaQwen2_5_72B, Name: "Qwen 2.5 72B (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "qwen/qwen2.5-72b-instruct",
		CostPer1MIn: 0.35, CostPer1MOut: 0.4,
		ContextWindow: 128_000, DefaultMaxTokens: 8192,
	},
	NvidiaQwen2_5Coder_32B: {
		ID: NvidiaQwen2_5Coder_32B, Name: "Qwen 2.5 Coder 32B (NVIDIA NIM)",
		Provider: ProviderNvidia, APIModel: "qwen/qwen2.5-coder-32b-instruct",
		CostPer1MIn: 0.25, CostPer1MOut: 0.3,
		ContextWindow: 128_000, DefaultMaxTokens: 8192,
	},
}

// ── Live model fetching ───────────────────────────────────────────────────────

type nvidiaModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type nvidiaModelList struct {
	Object string             `json:"object"`
	Data   []nvidiaModelEntry `json:"data"`
}

func fetchNvidiaModels(apiKey string) []nvidiaModelEntry {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET", nvidiaModelsURL, nil)
	if err != nil {
		logging.Debug("nvidia: failed to build models request", "error", err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logging.Debug("nvidia: models fetch failed", "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Debug("nvidia: models endpoint non-200", "status", resp.StatusCode)
		return nil
	}

	var list nvidiaModelList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		logging.Debug("nvidia: decode models failed", "error", err)
		return nil
	}

	logging.Debug("nvidia: fetched live models", "count", len(list.Data))
	return list.Data
}

// nvidiaModelID builds the internal ModelID from an API model string.
func nvidiaModelID(apiModel string) ModelID {
	return ModelID("nvidia." + apiModel)
}

// isReasoningModel returns true for known chain-of-thought / reasoning models.
func isReasoningModel(id string) bool {
	lower := strings.ToLower(id)
	for _, p := range []string{"-r1", "deepseek-r1", "qwq", "-thinking", "-reason"} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// nvidiaFriendlyName turns "deepseek-ai/deepseek-r1-0528" into a readable name.
func nvidiaFriendlyName(apiModel string) string {
	cap := func(s string) string {
		words := strings.Split(s, "-")
		for i, w := range words {
			if len(w) > 0 {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
		return strings.Join(words, "-")
	}
	parts := strings.SplitN(apiModel, "/", 2)
	if len(parts) == 2 {
		return cap(parts[0]) + " / " + cap(parts[1]) + " (NVIDIA NIM)"
	}
	return cap(apiModel) + " (NVIDIA NIM)"
}

// convertNvidiaModel maps a live API entry to a Model struct, reusing any
// cost/context metadata from the static NvidiaModels table if available.
func convertNvidiaModel(entry nvidiaModelEntry) Model {
	id := nvidiaModelID(entry.ID)
	if existing, ok := NvidiaModels[id]; ok {
		existing.Name = nvidiaFriendlyName(entry.ID)
		return existing
	}
	return Model{
		ID:               id,
		Name:             nvidiaFriendlyName(entry.ID),
		Provider:         ProviderNvidia,
		APIModel:         entry.ID,
		ContextWindow:    131_072,
		DefaultMaxTokens: 4096,
		CanReason:        isReasoningModel(entry.ID),
	}
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// Prefer the environment variable; fall back to viper config file value.
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		apiKey = viper.GetString("providers.nvidia.apiKey")
	}
	if apiKey == "" {
		// No key — static NvidiaModels still merge in via models.go init().
		return
	}

	entries := fetchNvidiaModels(apiKey)
	if len(entries) == 0 {
		logging.Debug("nvidia: no dynamic models returned, using static set")
		return
	}

	for _, entry := range entries {
		m := convertNvidiaModel(entry)
		// Update both NvidiaModels (source of truth) and SupportedModels
		// (global registry). models.go's maps.Copy will have already run for
		// the static set; we patch SupportedModels directly here for new ones.
		NvidiaModels[m.ID] = m
		SupportedModels[m.ID] = m
	}

	// Raise provider priority when a key is present so NVIDIA shows near top.
	ProviderPopularity[ProviderNvidia] = 3

	logging.Debug("nvidia: dynamic model load complete", "total", len(NvidiaModels))
}
