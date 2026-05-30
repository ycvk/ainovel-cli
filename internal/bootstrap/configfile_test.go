package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configtemplate "github.com/voocel/ainovel-cli"
)

func TestConfigExampleIsCanonicalEmbeddedTemplate(t *testing.T) {
	rootTemplate, err := os.ReadFile(filepath.Join("..", "..", "config.example.jsonc"))
	if err != nil {
		t.Fatalf("read root config example: %v", err)
	}
	if got, want := configtemplate.ConfigExampleJSONC, string(rootTemplate); got != want {
		t.Fatalf("embedded config example differs from root config.example.jsonc")
	}

	cfg := loadConfigExampleText(t, "root", string(rootTemplate))
	if cfg.Provider != "openrouter" {
		t.Fatalf("provider = %q, want openrouter", cfg.Provider)
	}
	if cfg.ModelName == "" {
		t.Fatalf("model must be present")
	}
	if _, ok := cfg.Providers[cfg.Provider]; !ok {
		t.Fatalf("default provider %q not configured", cfg.Provider)
	}

	compactEnabled := uncommentRequiredLine(t, string(rootTemplate), `// "compact_window": 300000,`, `"compact_window": 300000,`)
	loadConfigExampleText(t, "compact_window uncommented", compactEnabled)

	debugEnabled := uncommentRequiredLine(t, string(rootTemplate), `// "debug_stream_thinking": true,`, `"debug_stream_thinking": true,`)
	debugCfg := loadConfigExampleText(t, "debug_stream_thinking uncommented", debugEnabled)
	if !debugCfg.DebugStreamThinking {
		t.Fatalf("debug_stream_thinking should parse as true when uncommented")
	}
}

func TestLoadConfigMergesExplicitZeroValues(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(project)

	writeConfigFile(t, filepath.Join(home, configDirName, "config.json"), `{
  "provider": "my-proxy",
  "model": "gpt-5.5",
  "providers": {
    "my-proxy": {
      "type": "openai",
      "api_key": "secret",
      "base_url": "https://proxy.example.com/v1",
      "models": ["gpt-5.5"]
    }
  },
  "compact_window": 300000,
  "debug_stream_thinking": true
}`)
	writeConfigFile(t, "ainovel.json", `{
  "providers": {
    "my-proxy": {
      "api_key": "",
      "models": []
    }
  },
  "compact_window": 0,
  "debug_stream_thinking": false
}`)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.CompactWindow != 0 {
		t.Fatalf("CompactWindow = %d, want explicit project override 0", cfg.CompactWindow)
	}
	if cfg.DebugStreamThinking {
		t.Fatalf("DebugStreamThinking = true, want explicit project override false")
	}
	provider := cfg.Providers["my-proxy"]
	if provider.APIKey != "" {
		t.Fatalf("provider api_key = %q, want explicit empty override", provider.APIKey)
	}
	if len(provider.Models) != 0 {
		t.Fatalf("provider models = %#v, want explicit empty override", provider.Models)
	}
}

func TestLoadConfigFailsOnProjectConfigErrors(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(project)

	writeMinimalGlobalConfig(t, home)
	writeConfigFile(t, "ainovel.json", `{`)

	_, err := LoadConfig("")
	if err == nil {
		t.Fatalf("LoadConfig should fail on malformed project config")
	}
	if !strings.Contains(err.Error(), "ainovel.json") {
		t.Fatalf("error = %v, want path context", err)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(project)

	writeMinimalGlobalConfig(t, home)
	writeConfigFile(t, "ainovel.json", `{"output_dir":"custom"}`)

	_, err := LoadConfig("")
	if err == nil {
		t.Fatalf("LoadConfig should reject unknown config fields")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v, want unknown field error", err)
	}
}

func loadConfigExampleText(t *testing.T, name, text string) Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), strings.NewReplacer(" ", "-", "/", "-").Replace(name)+".jsonc")
	writeConfigFile(t, path, text)
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("%s should parse: %v", name, err)
	}
	return cfg
}

func uncommentRequiredLine(t *testing.T, text, old, new string) string {
	t.Helper()
	if !strings.Contains(text, old) {
		t.Fatalf("config example missing optional line %q", old)
	}
	return strings.Replace(text, old, new, 1)
}

func writeMinimalGlobalConfig(t *testing.T, home string) {
	t.Helper()
	writeConfigFile(t, filepath.Join(home, configDirName, "config.json"), `{
  "provider": "my-proxy",
  "model": "gpt-5.5",
  "providers": {
    "my-proxy": {
      "type": "openai",
      "base_url": "https://proxy.example.com/v1"
    }
  }
}`)
}

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
