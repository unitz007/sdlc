package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sdlc/lib"
	"strings"
	"gopkg.in/yaml.v3"
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

// LoadEnvConfig reads the .sdlc.conf file from the given directory.
// It parses lines starting with '$' as environment variables and '-' as flags.
func LoadEnvConfig(dir string) (*EnvSettings, error) {
	configPath := filepath.Join(dir, envConfigName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}

	file, err := os.Open(configPath)
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

		if strings.HasPrefix(line, "$") {
			// Environment variable: $KEY=VALUE
			parts := strings.SplitN(line[1:], "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove surrounding quotes if present
				if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
					value = value[1 : len(value)-1]
				}
				config.Env[key] = value
			}
		} else if strings.HasPrefix(line, "-") {
			// Flag: --flag=value or -f=value
			// Check for value assignment
			if idx := strings.Index(line, "="); idx != -1 {
				key := line[:idx]
				value := line[idx+1:]
				// Remove surrounding quotes from value
				if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
					value = value[1 : len(value)-1]
				}
				config.Args = append(config.Args, key+"="+value)
			} else {
				config.Args = append(config.Args, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading env config: %w", err)
	}

	return config, nil
}


// LoadYAML imports workflow definition from a YAML file.
func LoadYAML(filePath string) (map[string]lib.Task, error) {
    content, err := os.ReadFile(filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read YAML file: %w", err)
    }
    if len(content) == 0 {
        return make(map[string]lib.Task), nil
    }
    var tasks map[string]lib.Task
    if err := yaml.Unmarshal(content, &tasks); err != nil {
        return nil, fmt.Errorf("invalid YAML structure in %s: %w", filePath, err)
    }
    return tasks, nil
}

// ExportYAML writes the workflow definition to a YAML file.
func ExportYAML(filePath string, tasks map[string]lib.Task) error {
    data, err := yaml.Marshal(tasks)
    if err != nil {
        return fmt.Errorf("failed to marshal tasks to YAML: %w", err)
    }
    if err := os.WriteFile(filePath, data, 0644); err != nil {
        return fmt.Errorf("failed to write YAML file: %w", err)
    }
    return nil
}

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

// GetConfigFilePath returns the path to the configuration file (JSON) for the given directory.
func GetConfigFilePath(confDir string) (string, error) {
    return getConfigFile(confDir)
}

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
