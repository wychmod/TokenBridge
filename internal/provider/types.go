package provider

import (
	"fmt"
	"strings"
)

// Provider API format types.
// These determine how the gateway authenticates and forwards requests.
const (
	// TypeOpenAI is the official OpenAI API format.
	// Uses "Authorization: Bearer <key>" header.
	// Endpoint: /v1/chat/completions
	TypeOpenAI = "openai"

	// TypeOpenAICompatible is OpenAI-compatible format used by third-party providers
	// such as DeepSeek, SiliconFlow, OpenRouter, Moonshot, etc.
	// Uses "Authorization: Bearer <key>" header.
	// Endpoint: /v1/chat/completions
	TypeOpenAICompatible = "openai-compatible"

	// TypeAnthropic is the official Anthropic Claude API format.
	// Uses "x-api-key: <key>" header.
	// Endpoint: /v1/messages
	TypeAnthropic = "anthropic"
)

// KnownProviderTypes lists all valid provider type values.
var KnownProviderTypes = []string{
	TypeOpenAI,
	TypeOpenAICompatible,
	TypeAnthropic,
}

// ProviderDefaults holds recommended default configuration per provider type.
type ProviderDefaults struct {
	BaseURL string
}

var providerDefaultsMap = map[string]ProviderDefaults{
	TypeOpenAI: {
		BaseURL: "https://api.openai.com",
	},
	TypeOpenAICompatible: {
		BaseURL: "",
	},
	TypeAnthropic: {
		BaseURL: "https://api.anthropic.com",
	},
}

// GetDefaults returns the recommended defaults for a provider type.
func GetDefaults(providerType string) (ProviderDefaults, bool) {
	d, ok := providerDefaultsMap[NormalizeType(providerType)]
	return d, ok
}

// NormalizeType maps legacy UI labels and provider-specific aliases to canonical provider type values.
func NormalizeType(t string) string {
	normalized := strings.ToLower(strings.TrimSpace(t))
	switch normalized {
	case TypeOpenAI, "openai official", "openai官方", "openai 官方":
		return TypeOpenAI
	case TypeOpenAICompatible, "openai compatible", "openai_compatible", "openai 兼容", "openai兼容", "deepseek", "deepseek compatible", "deepseek 兼容", "deepseek兼容":
		return TypeOpenAICompatible
	case TypeAnthropic, "claude", "anthropic compatible", "anthropic 兼容", "anthropic兼容", "anthropic official", "anthropic 官方", "anthropic官方":
		return TypeAnthropic
	default:
		return strings.TrimSpace(t)
	}
}

// ValidateType checks if a provider type string is valid after normalization.
func ValidateType(t string) error {
	normalized := NormalizeType(t)
	if normalized == "" {
		return fmt.Errorf("供应商类型(type)不能为空，可选值：%v", KnownProviderTypes)
	}
	for _, known := range KnownProviderTypes {
		if normalized == known {
			return nil
		}
	}
	return fmt.Errorf("不支持的供应商类型 %q，可选值：%v", t, KnownProviderTypes)
}

// IsAnthropic returns true if the provider type should use Anthropic-style authentication.
func IsAnthropic(providerType string) bool {
	return NormalizeType(providerType) == TypeAnthropic
}

// IsOpenAIFormat returns true if the provider type uses OpenAI chat completions format.
func IsOpenAIFormat(providerType string) bool {
	normalized := NormalizeType(providerType)
	return normalized == TypeOpenAI || normalized == TypeOpenAICompatible
}

// APIFormat represents the request format being sent by the client.
const (
	APIFormatOpenAI = "openai" // /v1/chat/completions
	APIFormatClaude = "claude" // /v1/messages
)

// ValidateFormatCompatibility checks if a provider type is compatible with the given API format.
// Returns nil if compatible, an error describing the mismatch if not.
func ValidateFormatCompatibility(providerType string, apiFormat string) error {
	switch apiFormat {
	case APIFormatOpenAI:
		if IsOpenAIFormat(providerType) {
			return nil
		}
		return fmt.Errorf("供应商类型 %q 不兼容 OpenAI 格式请求（/v1/chat/completions），请使用 Claude 格式请求（/v1/messages）", providerType)
	case APIFormatClaude:
		if IsAnthropic(providerType) {
			return nil
		}
		return fmt.Errorf("供应商类型 %q 不兼容 Claude 格式请求（/v1/messages），请使用 OpenAI 格式请求（/v1/chat/completions）", providerType)
	default:
		return nil // Unknown format, allow passthrough
	}
}
