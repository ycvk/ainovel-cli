package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const configDirName = ".ainovel"

// DefaultConfigPath 返回全局配置文件路径 ~/.ainovel/config.json。
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, configDirName, "config.json")
}

// DefaultConfigDir 返回 ~/.ainovel 目录路径；取不到家目录时返回空字符串。
// 仅用于读/写不强制存在的文件（如模型缓存），不会自动创建目录。
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, configDirName)
}

// configDir 返回 ~/.ainovel 目录路径，不存在时创建。
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, configDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

// LoadConfig 按优先级加载并合并配置：
//  1. ~/.ainovel/config.json（全局）
//  2. ./ainovel.json（项目级覆盖）
//  3. flagPath 指定的路径（最高优先级）
func LoadConfig(flagPath string) (Config, error) {
	var cfg Config

	// 1. 全局配置
	if p := DefaultConfigPath(); p != "" {
		global, err := loadConfigPatchFile(p)
		switch {
		case err == nil:
			cfg = mergeConfig(cfg, global)
		case !os.IsNotExist(err):
			return cfg, fmt.Errorf("load config %s: %w", p, err)
		}
	}

	// 2. 项目级覆盖
	if project, err := loadConfigPatchFile("ainovel.json"); err == nil {
		cfg = mergeConfig(cfg, project)
	} else if !os.IsNotExist(err) {
		return cfg, fmt.Errorf("load config ainovel.json: %w", err)
	}

	// 3. CLI flag 覆盖
	if flagPath != "" {
		override, err := loadConfigPatchFile(flagPath)
		if err != nil {
			return cfg, fmt.Errorf("load config %s: %w", flagPath, err)
		}
		cfg = mergeConfig(cfg, override)
	}

	return cfg, nil
}

// LoadConfigFile 读取单个 JSON 配置文件，支持 // 行注释。
// 不做任何合并，仅返回该文件自身的配置。文件不存在时返回错误。
func LoadConfigFile(path string) (Config, error) {
	patch, err := loadConfigPatchFile(path)
	if err != nil {
		return Config{}, err
	}
	return mergeConfig(Config{}, patch), nil
}

// configPatch tracks field presence so overlays can explicitly set zero values
// such as false, 0, "", and [].
type configPatch struct {
	Provider            *string                        `json:"provider,omitempty"`
	ModelName           *string                        `json:"model,omitempty"`
	Providers           map[string]providerConfigPatch `json:"providers,omitempty"`
	Roles               map[string]roleConfigPatch     `json:"roles,omitempty"`
	Style               *string                        `json:"style,omitempty"`
	CompactWindow       *int                           `json:"compact_window,omitempty"`
	DebugStreamThinking *bool                          `json:"debug_stream_thinking,omitempty"`
}

type providerConfigPatch struct {
	Type    *string   `json:"type,omitempty"`
	APIKey  *string   `json:"api_key,omitempty"`
	BaseURL *string   `json:"base_url,omitempty"`
	Models  *[]string `json:"models,omitempty"`
}

type roleConfigPatch struct {
	Provider  *string     `json:"provider,omitempty"`
	Model     *string     `json:"model,omitempty"`
	Fallbacks *[]ModelRef `json:"fallbacks,omitempty"`
}

// loadConfigPatchFile 读取 JSON 配置文件，支持 // 行注释。
// 文件不存在时返回错误（由调用方决定是否忽略）。
func loadConfigPatchFile(path string) (configPatch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return configPatch{}, err
	}
	return parseConfigPatch(path, data)
}

func parseConfigPatch(path string, data []byte) (configPatch, error) {
	cleaned := stripJSONComments(data)
	dec := json.NewDecoder(bytes.NewReader(cleaned))
	dec.DisallowUnknownFields()

	var patch configPatch
	if err := dec.Decode(&patch); err != nil {
		return configPatch{}, fmt.Errorf("parse %s: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return configPatch{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return patch, nil
}

// mergeConfig 将 overlay 合并到 base 上。出现过的标量字段覆盖，map 按 key 合并。
func mergeConfig(base Config, overlay configPatch) Config {
	if overlay.Provider != nil {
		base.Provider = *overlay.Provider
	}
	if overlay.ModelName != nil {
		base.ModelName = *overlay.ModelName
	}
	if overlay.Style != nil {
		base.Style = *overlay.Style
	}
	if overlay.CompactWindow != nil {
		base.CompactWindow = *overlay.CompactWindow
	}
	if overlay.DebugStreamThinking != nil {
		base.DebugStreamThinking = *overlay.DebugStreamThinking
	}

	// Providers: overlay 的 key 覆盖 base 同名 key
	if len(overlay.Providers) > 0 {
		if base.Providers == nil {
			base.Providers = make(map[string]ProviderConfig)
		}
		for k, v := range overlay.Providers {
			existing := base.Providers[k]
			if v.Type != nil {
				existing.Type = *v.Type
			}
			if v.APIKey != nil {
				existing.APIKey = *v.APIKey
			}
			if v.BaseURL != nil {
				existing.BaseURL = *v.BaseURL
			}
			if v.Models != nil {
				existing.Models = append([]string(nil), (*v.Models)...)
			}
			base.Providers[k] = existing
		}
	}

	// Roles: overlay 的 key 覆盖 base 同名 key
	if len(overlay.Roles) > 0 {
		if base.Roles == nil {
			base.Roles = make(map[string]RoleConfig)
		}
		for k, v := range overlay.Roles {
			existing := base.Roles[k]
			if v.Provider != nil {
				existing.Provider = *v.Provider
			}
			if v.Model != nil {
				existing.Model = *v.Model
			}
			if v.Fallbacks != nil {
				existing.Fallbacks = append([]ModelRef(nil), (*v.Fallbacks)...)
			}
			base.Roles[k] = existing
		}
	}

	return base
}

// stripJSONComments 去除 JSON 中的 // 行注释，跟踪引号状态避免误删字符串内容。
func stripJSONComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	escaped := false

	for i := 0; i < len(data); i++ {
		b := data[i]

		if escaped {
			out = append(out, b)
			escaped = false
			continue
		}

		if inString {
			out = append(out, b)
			if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}

		// 不在字符串内
		if b == '"' {
			inString = true
			out = append(out, b)
			continue
		}

		// 检测 // 注释
		if b == '/' && i+1 < len(data) && data[i+1] == '/' {
			// 跳到行尾
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
			continue
		}

		out = append(out, b)
	}

	return out
}

// SaveConfig 将配置写入指定路径（JSON 格式，缩进美化）。
func SaveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
