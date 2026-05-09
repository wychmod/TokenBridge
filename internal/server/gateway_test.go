package server

import "testing"

func TestBuildProviderEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		operationPath string
		want          string
	}{
		{
			name:          "openai root base appends v1 chat completions",
			baseURL:       "https://api.openai.com",
			operationPath: "/v1/chat/completions",
			want:          "https://api.openai.com/v1/chat/completions",
		},
		{
			name:          "openai v1 base does not duplicate v1",
			baseURL:       "https://api.minimaxi.com/v1",
			operationPath: "/v1/chat/completions",
			want:          "https://api.minimaxi.com/v1/chat/completions",
		},
		{
			name:          "openai v1 base trailing slash does not duplicate v1",
			baseURL:       "https://api.minimaxi.com/v1/",
			operationPath: "/v1/chat/completions",
			want:          "https://api.minimaxi.com/v1/chat/completions",
		},
		{
			name:          "anthropic prefix base appends v1 messages",
			baseURL:       "https://api.minimaxi.com/anthropic",
			operationPath: "/v1/messages",
			want:          "https://api.minimaxi.com/anthropic/v1/messages",
		},
		{
			name:          "anthropic v1 prefix base does not duplicate v1",
			baseURL:       "https://api.minimaxi.com/anthropic/v1",
			operationPath: "/v1/messages",
			want:          "https://api.minimaxi.com/anthropic/v1/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildProviderEndpoint(tt.baseURL, tt.operationPath)
			if err != nil {
				t.Fatalf("buildProviderEndpoint() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildProviderEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildProviderEndpointRejectsEmptyBaseURL(t *testing.T) {
	_, err := buildProviderEndpoint("   ", "/v1/chat/completions")
	if err == nil {
		t.Fatal("buildProviderEndpoint() expected error for empty baseURL")
	}
}
