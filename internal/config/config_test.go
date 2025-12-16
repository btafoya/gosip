package config

import (
	"os"
	"testing"
)

func TestGetEnvStringSlice(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue []string
		want         []string
	}{
		{
			name:         "empty environment variable uses default",
			envValue:     "",
			defaultValue: []string{"default1", "default2"},
			want:         []string{"default1", "default2"},
		},
		{
			name:         "single value",
			envValue:     "value1",
			defaultValue: []string{"default"},
			want:         []string{"value1"},
		},
		{
			name:         "multiple values",
			envValue:     "value1,value2,value3",
			defaultValue: []string{"default"},
			want:         []string{"value1", "value2", "value3"},
		},
		{
			name:         "values with whitespace",
			envValue:     "value1, value2 , value3",
			defaultValue: []string{"default"},
			want:         []string{"value1", "value2", "value3"},
		},
		{
			name:         "empty values filtered out",
			envValue:     "value1,,value2",
			defaultValue: []string{"default"},
			want:         []string{"value1", "value2"},
		},
		{
			name:         "only commas uses default",
			envValue:     ",,,",
			defaultValue: []string{"default"},
			want:         []string{"default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_STRING_SLICE"
			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
				defer os.Unsetenv(key)
			}

			got := getEnvStringSlice(key, tt.defaultValue)

			if len(got) != len(tt.want) {
				t.Errorf("getEnvStringSlice() length = %v, want %v", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("getEnvStringSlice()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLoadCORSOrigins(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     []string
	}{
		{
			name:     "default CORS origins",
			envValue: "",
			want: []string{
				"http://localhost:3000",
				"http://localhost:8080",
				"http://127.0.0.1:3000",
				"http://127.0.0.1:8080",
			},
		},
		{
			name:     "custom CORS origins",
			envValue: "https://example.com,https://www.example.com",
			want:     []string{"https://example.com", "https://www.example.com"},
		},
		{
			name:     "custom CORS origins with whitespace",
			envValue: "https://example.com, https://admin.example.com, https://api.example.com",
			want:     []string{"https://example.com", "https://admin.example.com", "https://api.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("GOSIP_CORS_ORIGINS", tt.envValue)
				defer os.Unsetenv("GOSIP_CORS_ORIGINS")
			} else {
				os.Unsetenv("GOSIP_CORS_ORIGINS")
			}

			cfg := Load()

			if len(cfg.CORSOrigins) != len(tt.want) {
				t.Errorf("Load().CORSOrigins length = %v, want %v", len(cfg.CORSOrigins), len(tt.want))
				return
			}

			for i := range cfg.CORSOrigins {
				if cfg.CORSOrigins[i] != tt.want[i] {
					t.Errorf("Load().CORSOrigins[%d] = %v, want %v", i, cfg.CORSOrigins[i], tt.want[i])
				}
			}
		})
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no whitespace", "hello", "hello"},
		{"leading space", " hello", "hello"},
		{"trailing space", "hello ", "hello"},
		{"both spaces", " hello ", "hello"},
		{"multiple spaces", "  hello  ", "hello"},
		{"tabs", "\thello\t", "hello"},
		{"newlines", "\nhello\n", "hello"},
		{"mixed whitespace", " \t\nhello\n\t ", "hello"},
		{"only whitespace", "   ", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimSpace(tt.input)
			if got != tt.want {
				t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter string
		want      []string
	}{
		{"simple split", "a,b,c", ",", []string{"a", "b", "c"}},
		{"empty string", "", ",", []string{}},
		{"no delimiter", "abc", ",", []string{"abc"}},
		{"trailing delimiter", "a,b,", ",", []string{"a", "b", ""}},
		{"leading delimiter", ",a,b", ",", []string{"", "a", "b"}},
		{"multi-char delimiter", "a::b::c", "::", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitString(tt.input, tt.delimiter)

			if len(got) != len(tt.want) {
				t.Errorf("splitString() length = %v, want %v", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitString()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
