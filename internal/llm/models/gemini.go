package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/spf13/viper"
)

const (
	ProviderGemini ModelProvider = "gemini"

	geminiBaseURL   = "https://generativelanguage.googleapis.com"
	geminiModelsURL = geminiBaseURL + "/v1beta/models"

	// Static / well-known model IDs (used as fallback and for cost metadata)
	Gemini25Flash     ModelID = "gemini-2.5-flash"
	Gemini25          ModelID = "gemini-2.5"
	Gemini20Flash     ModelID = "gemini-2.0-flash"
	Gemini20FlashLite ModelID = "gemini-2.0-flash-lite"
)

// GeminiModels is the static fallback set.
// The init() function merges live-fetched models on top so you always have
// something usable even without connectivity.
var GeminiModels = map[ModelID]Model{
	Gemini25Flash: {
		ID:                  Gemini25Flash,
		Name:                "Gemini 2.5 Flash",
		Provider:            ProviderGemini,
		APIModel:            "gemini-2.5-flash-preview-04-17",
		CostPer1MIn:         0.15,
		CostPer1MOut:        0.60,
		ContextWindow:       1_000_000,
		DefaultMaxTokens:    50_000,
		SupportsAttachments: true,
	},
	Gemini25: {
		ID:                  Gemini25,
		Name:                "Gemini 2.5 Pro",
		Provider:            ProviderGemini,
		APIModel:            "gemini-2.5-pro-preview-05-06",
		CostPer1MIn:         1.25,
		CostPer1MOut:        10,
		ContextWindow:       1_000_000,
		DefaultMaxTokens:    50_000,
		SupportsAttachments: true,
	},
	Gemini20Flash: {
		ID:                  Gemini20Flash,
		Name:                "Gemini 2.0 Flash",
		Provider:            ProviderGemini,
		APIModel:            "gemini-2.0-flash",
		CostPer1MIn:         0.10,
		CostPer1MOut:        0.40,
		ContextWindow:       1_000_000,
		DefaultMaxTokens:    6_000,
		SupportsAttachments: true,
	},
	Gemini20FlashLite: {
		ID:                  Gemini20FlashLite,
		Name:                "Gemini 2.0 Flash Lite",
		Provider:            ProviderGemini,
		APIModel:            "gemini-2.0-flash-lite",
		CostPer1MIn:         0.05,
		CostPer1MOut:        0.30,
		ContextWindow:       1_000_000,
		DefaultMaxTokens:    6_000,
		SupportsAttachments: true,
	},
}

// ── Live model fetching via Google AI REST API ────────────────────────────────

type geminiModelEntry struct {
	Name                       string   `json:"name"`        // e.g. "models/gemini-2.5-flash"
	DisplayName                string   `json:"displayName"` // e.g. "Gemini 2.5 Flash"
	Description                string   `json:"description"`
	InputTokenLimit            int64    `json:"inputTokenLimit"`
	OutputTokenLimit           int64    `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

type geminiModelList struct {
	Models []geminiModelEntry `json:"models"`
}

func fetchGeminiModels(apiKey string) []geminiModelEntry {
	client := &http.Client{Timeout: 15 * time.Second}

	url := fmt.Sprintf("%s?key=%s", geminiModelsURL, apiKey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logging.Debug("gemini: failed to build models request", "error", err)
		return nil
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logging.Debug("gemini: models fetch failed", "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Debug("gemini: models endpoint non-200", "status", resp.StatusCode)
		return nil
	}

	var list geminiModelList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		logging.Debug("gemini: decode models failed", "error", err)
		return nil
	}

	// Only return models that support generateContent
	var out []geminiModelEntry
	for _, m := range list.Models {
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				out = append(out, m)
				break
			}
		}
	}

	logging.Debug("gemini: fetched live models", "count", len(out))
	return out
}

// geminiAPIModel strips the "models/" prefix from the name field.
// e.g. "models/gemini-2.5-flash" → "gemini-2.5-flash"
func geminiAPIModel(name string) string {
	return strings.TrimPrefix(name, "models/")
}

// geminiModelID builds a stable internal ModelID from the API model string.
func geminiModelID(apiModel string) ModelID {
	return ModelID(apiModel)
}

// geminiIsVision returns true for models that support image input.
func geminiIsVision(name string) bool {
	lower := strings.ToLower(name)
	return !strings.Contains(lower, "text") && !strings.Contains(lower, "embedding")
}

// convertGeminiModel maps a live API entry → Model, reusing static metadata
// (costs, context window) when available.
func convertGeminiModel(entry geminiModelEntry) Model {
	apiModel := geminiAPIModel(entry.Name)
	id := geminiModelID(apiModel)

	if existing, ok := GeminiModels[id]; ok {
		// Keep cost/window metadata; refresh the display name.
		if entry.DisplayName != "" {
			existing.Name = entry.DisplayName
		}
		return existing
	}

	contextWindow := entry.InputTokenLimit
	if contextWindow == 0 {
		contextWindow = 32_768
	}
	maxOut := entry.OutputTokenLimit
	if maxOut == 0 {
		maxOut = 8_192
	}

	return Model{
		ID:                  id,
		Name:                entry.DisplayName,
		Provider:            ProviderGemini,
		APIModel:            apiModel,
		ContextWindow:       contextWindow,
		DefaultMaxTokens:    maxOut,
		SupportsAttachments: geminiIsVision(entry.Name),
	}
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// Prefer env var; fall back to viper config file value.
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = viper.GetString("providers.gemini.apiKey")
	}
	if apiKey == "" {
		// No key — static GeminiModels still merge in via models.go init().
		return
	}

	entries := fetchGeminiModels(apiKey)
	if len(entries) == 0 {
		logging.Debug("gemini: no dynamic models returned, using static set")
		return
	}

	for _, entry := range entries {
		m := convertGeminiModel(entry)
		// Patch both GeminiModels (source of truth for this provider) and
		// SupportedModels (global registry). models.go maps.Copy has already
		// run for the static set; we patch SupportedModels directly here for
		// any new models returned by the live endpoint.
		GeminiModels[m.ID] = m
		SupportedModels[m.ID] = m
	}

	// Raise provider priority when a key is present so Gemini shows near top.
	ProviderPopularity[ProviderGemini] = 2

	logging.Debug("gemini: dynamic model load complete", "total", len(GeminiModels))
}