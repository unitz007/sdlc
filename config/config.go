package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sdlc/lib"
	"strings"
)

const (
	configFileName = ".sdlc.json"
	envConfigName  = ".sdlc.conf"
)

// EnvSettings represents the configuration from .sdlc.conf
type EnvSettings struct {
	Env  map[string]string
	Args []string
}

// ParseEnvConfig reads and parses a .sdlc.conf file at the given explicit path.
// Plain KEY=VALUE lines become environment variables.
// FLAG=value lines become extra flags (the value after FLAG= is appended to args).
// Lines with = but empty value (e.g., KEY=) store an empty string.
// Lines with no = at all are silently skipped.
// Lines starting with # and blank lines are ignored.
// Surrounding quotes on values are stripped.
func ParseEnvConfig(filePath string) (*EnvSettings, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open env config: %w", err)
	}
	defer file.Close()

	config := &EnvSettings{
		Env:  make(map[string]string),
		Args: make([]string, 0),
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Lines without '=' are silently skipped
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])

		// Remove surrounding quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		// FLAG=value lines become extra flags
		if strings.HasPrefix(key, "-") {
			config.Args = append(config.Args, key+"="+value)
		} else {
			// Plain KEY=VALUE lines become environment variables
			config.Env[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading env config: %w", err)
	}

	return config, nil
}

// LoadEnvConfig reads the .sdlc.conf file from the given directory.
// It returns nil if the file does not exist.
func LoadEnvConfig(dir string) (*EnvSettings, error) {
	configPath := filepath.Join(dir, envConfigName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}
	return ParseEnvConfig(configPath)
}

// MergeEnvSettings returns a new EnvSettings where env vars from override replace
// those from base (map merge), and args from override are appended after base args
// (slice concat). Nil inputs are treated as empty.
func MergeEnvSettings(base, override *EnvSettings) *EnvSettings {
	result := &EnvSettings{
		Env:  make(map[string]string),
		Args: make([]string, 0),
	}

	if base != nil {
		for k, v := range base.Env {
			result.Env[k] = v
		}
		result.Args = append(result.Args, base.Args...)
	}

	if override != nil {
		for k, v := range override.Env {
			result.Env[k] = v
		}
		result.Args = append(result.Args, override.Args...)
	}

	return result
}

// Load reads the .sdlc.json configuration file from the given directory path.
// If conf is empty, it defaults to the user's home directory.
// If the file does not exist, an empty file is created.
func Load(confDir string) (map[string]lib.Task, error) {
	configFile, err := getConfigFile(confDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get config file: %w", err)
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		// Should not happen as getConfigFile ensures file exists, but good to check
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(content) == 0 {
		return make(map[string]lib.Task), nil
	}

	var tasks map[string]lib.Task
	if err := json.Unmarshal(content, &tasks); err != nil {
		return nil, fmt.Errorf("invalid configuration structure in %s: %w", configFile, err)
	}

	return tasks, nil
}

// LoadLocal reads the .sdlc.json configuration file from the given directory path.
// It returns nil if the file does not exist, without creating it.
func LoadLocal(confDir string) (map[string]lib.Task, error) {
	configPath := filepath.Join(confDir, configFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(content) == 0 {
		return make(map[string]lib.Task), nil
	}

	var tasks map[string]lib.Task
	if err := json.Unmarshal(content, &tasks); err != nil {
		return nil, fmt.Errorf("invalid configuration structure in %s: %w", configPath, err)
	}

	return tasks, nil
}

func getConfigFile(confDir string) (string, error) {
	var configPath string
	if confDir != "" {
		configPath = filepath.Join(confDir, configFileName)
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, configFileName)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if file, err := os.Create(configPath); err != nil {
			return "", fmt.Errorf("could not create config file at %s: %w", configPath, err)
		} else {
			file.Close()
		}
	}

	return configPath, nil
}
