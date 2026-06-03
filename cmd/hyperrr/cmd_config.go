package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var defaultConfigPaths = []string{
	"hyperrr.yml",
	"hyperrr.yaml",
	"hyperrr.json",
	"configs/hyperrr.yml",
	"configs/hyperrr.yaml",
	"configs/hyperrr.json",
}

var configCmd = &cobra.Command{
	Use:     "config",
	Short:   "Configuration management",
	Long:    `View, modify, locate, and initialize configuration options stored in hyperrr.yml.`,
	GroupID: "config",
}

type configEntry struct {
	Key    string `json:"key"`
	Value  any    `json:"value"`
	Source string `json:"source"`
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all resolved configuration values",
	RunE: func(cmd *cobra.Command, args []string) error {
		deps, err := buildDeps(false)
		if err != nil {
			return err
		}

		cfg := deps.Config
		cfgPath := findConfigFile()
		exists := false
		if _, err := os.Stat(cfgPath); err == nil {
			exists = true
		}

		// Helper to check source
		getSource := func(key string) string {
			envKey := strings.ToUpper(key)
			if os.Getenv(envKey) != "" {
				return "env:" + envKey
			}
			if exists && isKeyInConfigFile(cfgPath, key) {
				return filepath.Base(cfgPath)
			}
			return "default"
		}

		entries := []configEntry{
			{"APP_NAME", cfg.AppName, getSource("APP_NAME")},
			{"APP_ENV", cfg.AppEnv, getSource("APP_ENV")},
			{"LOG_LEVEL", cfg.LogLevel, getSource("LOG_LEVEL")},
			{"LOG_FORMAT", cfg.LogFormat, getSource("LOG_FORMAT")},
			{"SERVER_PORT", cfg.ServerPort, getSource("SERVER_PORT")},
			{"EVENT_BUS_PROVIDER", cfg.EventBusProvider, getSource("EVENT_BUS_PROVIDER")},
			{"WORKFLOW_STORE_TYPE", cfg.WorkflowStoreType, getSource("WORKFLOW_STORE_TYPE")},
			{"DB_DRIVER", cfg.DBDriver, getSource("DB_DRIVER")},
			{"DB_DSN", cfg.DBDSN, getSource("DB_DSN")},
			{"STORAGE_PROVIDER", cfg.StorageProvider, getSource("STORAGE_PROVIDER")},
			{"STORAGE_BUCKET_URL", cfg.StorageBucketURL, getSource("STORAGE_BUCKET_URL")},
			{"NATS_URL", cfg.NATSURL, getSource("NATS_URL")},
			{"MCP_AUTH_PROVIDERS", cfg.MCPAuthProviders, getSource("MCP_AUTH_PROVIDERS")},
		}

		if jsonOutput {
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("\n  %-22s %-18s %s\n", "KEY", "VALUE", "SOURCE")
		fmt.Println("  " + strings.Repeat("─", 60))
		for _, entry := range entries {
			valStr := fmt.Sprintf("%v", entry.Value)
			if len(valStr) > 18 {
				valStr = valStr[:15] + "..."
			}
			fmt.Printf("  %-22s %-18s %s\n", entry.Key, valStr, entry.Source)
		}
		fmt.Println()
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a specific config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deps, err := buildDeps(false)
		if err != nil {
			return err
		}

		key := strings.ToUpper(args[0])
		cfg := deps.Config

		var val any
		switch key {
		case "APP_NAME":
			val = cfg.AppName
		case "APP_ENV":
			val = cfg.AppEnv
		case "LOG_LEVEL":
			val = cfg.LogLevel
		case "LOG_FORMAT":
			val = cfg.LogFormat
		case "SERVER_PORT":
			val = cfg.ServerPort
		case "EVENT_BUS_PROVIDER":
			val = cfg.EventBusProvider
		case "WORKFLOW_STORE_TYPE":
			val = cfg.WorkflowStoreType
		case "DB_DRIVER":
			val = cfg.DBDriver
		case "DB_DSN":
			val = cfg.DBDSN
		case "STORAGE_PROVIDER":
			val = cfg.StorageProvider
		case "STORAGE_BUCKET_URL":
			val = cfg.StorageBucketURL
		case "NATS_URL":
			val = cfg.NATSURL
		case "MCP_AUTH_PROVIDERS":
			val = cfg.MCPAuthProviders
		default:
			return fmt.Errorf("unknown configuration key: %s", args[0])
		}

		if jsonOutput {
			data, err := json.Marshal(map[string]any{key: val})
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Println(val)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value in hyperrr.yml",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToUpper(args[0])
		value := args[1]

		cfgPath := findConfigFile()

		// Validate key and convert type
		var typedVal any
		switch key {
		case "APP_NAME", "APP_ENV", "LOG_LEVEL", "LOG_FORMAT", "EVENT_BUS_PROVIDER", "WORKFLOW_STORE_TYPE", "DB_DRIVER", "DB_DSN", "STORAGE_PROVIDER", "STORAGE_BUCKET_URL", "NATS_URL":
			typedVal = value
		case "SERVER_PORT":
			var port int
			if _, err := fmt.Sscanf(value, "%d", &port); err != nil {
				return fmt.Errorf("invalid SERVER_PORT (must be integer): %s", value)
			}
			typedVal = port
		case "MCP_AUTH_PROVIDERS":
			providers := strings.Split(value, ",")
			for i, p := range providers {
				providers[i] = strings.TrimSpace(p)
			}
			typedVal = providers
		default:
			return fmt.Errorf("unknown configuration key: %s", args[0])
		}

		// Choose update strategy: comment-preserving YAML update vs Viper fallback
		isYAML := strings.HasSuffix(cfgPath, ".yml") || strings.HasSuffix(cfgPath, ".yaml")
		if isYAML {
			if err := updateYAMLFileCommentPreserving(cfgPath, key, typedVal); err != nil {
				return fmt.Errorf("failed to update config file: %w", err)
			}
		} else {
			// Fallback for JSON/other formats
			v := viper.New()
			v.SetConfigFile(cfgPath)
			_ = v.ReadInConfig()
			v.Set(key, typedVal)
			if err := v.WriteConfig(); err != nil {
				if err = v.WriteConfigAs(cfgPath); err != nil {
					return fmt.Errorf("failed to write config file: %w", err)
				}
			}
		}

		fmt.Printf("✓ Set %s = %s in %s\n", key, value, cfgPath)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path to the active config file",
	Run: func(cmd *cobra.Command, args []string) {
		path := findConfigFile()
		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Println(path)
			return
		}
		fmt.Println(absPath)
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a default hyperrr.yml",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "hyperrr.yml"
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists at %s", path)
		}

		defaultContent := `# Engine Configuration
APP_NAME: hyperrr
APP_ENV: local
LOG_LEVEL: info
LOG_FORMAT: text
SERVER_PORT: 8080

# Database
DB_DRIVER: sqlite
DB_DSN: hyperrr.db

# MCP Authentication
MCP_AUTH_PROVIDERS:
  - apikey

# Event Bus
EVENT_BUS_PROVIDER: inmem
`
		err := os.WriteFile(path, []byte(defaultContent), 0644)
		if err != nil {
			return fmt.Errorf("failed to write default config: %w", err)
		}

		fmt.Println("✓ Created hyperrr.yml with default configuration")
		return nil
	},
}

func findConfigFile() string {
	if cfgFile != "" {
		return cfgFile
	}
	for _, path := range defaultConfigPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "hyperrr.yml"
}

func isKeyInConfigFile(path string, key string) bool {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return false
	}
	return v.IsSet(strings.ToLower(key))
}

func updateYAMLFileCommentPreserving(filename string, key string, val any) error {
	data, err := os.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var root yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &root); err != nil {
			return err
		}
	} else {
		root = yaml.Node{
			Kind: yaml.DocumentNode,
		}
	}

	if len(root.Content) == 0 {
		mapNode := &yaml.Node{
			Kind: yaml.MappingNode,
			Tag:  "!!map",
		}
		root.Content = append(root.Content, mapNode)
	}

	mapNode := root.Content[0]
	if mapNode.Kind != yaml.MappingNode {
		return fmt.Errorf("root of YAML is not a map")
	}

	found := false
	for i := 0; i < len(mapNode.Content); i += 2 {
		kNode := mapNode.Content[i]
		if strings.EqualFold(kNode.Value, key) {
			vNode := mapNode.Content[i+1]
			if stringSlice, ok := val.([]string); ok {
				vNode.Kind = yaml.SequenceNode
				vNode.Tag = "!!seq"
				vNode.Value = ""
				vNode.Content = nil
				for _, str := range stringSlice {
					vNode.Content = append(vNode.Content, &yaml.Node{
						Kind:  yaml.ScalarNode,
						Tag:   "!!str",
						Value: str,
					})
				}
			} else {
				vNode.Kind = yaml.ScalarNode
				vNode.Value = fmt.Sprintf("%v", val)
				if _, ok := val.(int); ok {
					vNode.Tag = "!!int"
				} else {
					vNode.Tag = "!!str"
				}
			}
			found = true
			break
		}
	}

	if !found {
		kNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: key,
		}
		var vNode *yaml.Node
		if stringSlice, ok := val.([]string); ok {
			vNode = &yaml.Node{
				Kind: yaml.SequenceNode,
				Tag:  "!!seq",
			}
			for _, str := range stringSlice {
				vNode.Content = append(vNode.Content, &yaml.Node{
					Kind:  yaml.ScalarNode,
					Tag:   "!!str",
					Value: str,
				})
			}
		} else {
			vNode = &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: fmt.Sprintf("%v", val),
			}
			if _, ok := val.(int); ok {
				vNode.Tag = "!!int"
			} else {
				vNode.Tag = "!!str"
			}
		}
		mapNode.Content = append(mapNode.Content, kNode, vNode)
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return err
	}
	_ = encoder.Close()

	return os.WriteFile(filename, buf.Bytes(), 0644)
}

func init() {
	configCmd.AddCommand(configListCmd, configGetCmd, configSetCmd, configPathCmd, configInitCmd)
	rootCmd.AddCommand(configCmd)
}
