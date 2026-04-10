package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sdlc/lib"
	"sort"
	"strings"
)

const (
	configFileName = ".sdlc.json"
	envConfigName  = ".sdlc.conf"
)

// builtInActions are the reserved action names that cannot be used in the custom map.
var builtInActions = map[string]bool{
	"run":     true,
	"test":    true,
	"build":   true,
	"install": true,
	"clean":   true,
}

// EnvSettings represents the configuration from .sdlc.conf
type EnvSettings struct {
	Env     map[string]string
	Args    []string
	Depends []string // Module paths this module depends on (relative paths)
}

// LoadEnvConfig reads the .sdlc.conf file from the given directory.
// It parses lines starting with '$' as environment variables, '-' as flags,
// and 'depends=' as inter-module dependency declarations.
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
		Env:     make(map[string]string),
		Args:    make([]string, 0),
		Depends: make([]string, 0),
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
		} else if strings.HasPrefix(line, "depends=") {
			// Dependency declaration: depends=path1,path2,...
			depList := strings.TrimSpace(line[len("depends="):])
			if depList != "" {
				deps := strings.Split(depList, ",")
				for _, d := range deps {
					d = strings.TrimSpace(d)
					if d != "" {
						config.Depends = append(config.Depends, d)
					}
				}
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

// rawTask is an intermediate representation used for JSON unmarshaling that
// supports the extended schema with custom actions and hooks alongside the
// built-in lifecycle fields. This ensures backward compatibility with existing
// .sdlc.json files that only have the 5 built-in fields.
type rawTask struct {
	Run     string            `json:"run"`
	Test    string            `json:"test"`
	Build   string            `json:"build"`
	Install string            `json:"install"`
	Clean   string            `json:"clean"`
	Custom  map[string]string `json:"custom,omitempty"`
	Hooks   lib.TaskHooks     `json:"hooks,omitempty"`
}

// toTask converts a rawTask to a lib.Task.
func (r *rawTask) toTask() lib.Task {
	return lib.Task{
		Run:     r.Run,
		Test:    r.Test,
		Build:   r.Build,
		Install: r.Install,
		Clean:   r.Clean,
		Custom:  r.Custom,
		Hooks:   r.Hooks,
	}
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

	var rawTasks map[string]rawTask
	if err := json.Unmarshal(content, &rawTasks); err != nil {
		return nil, fmt.Errorf("invalid configuration structure in %s: %w", configFile, err)
	}

	tasks := make(map[string]lib.Task, len(rawTasks))
	for k, v := range rawTasks {
		tasks[k] = v.toTask()
	}

	if err := Validate(tasks, configFile); err != nil {
		return nil, err
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

	var rawTasks map[string]rawTask
	if err := json.Unmarshal(content, &rawTasks); err != nil {
		return nil, fmt.Errorf("invalid configuration structure in %s: %w", configPath, err)
	}

	tasks := make(map[string]lib.Task, len(rawTasks))
	for k, v := range rawTasks {
		tasks[k] = v.toTask()
	}

	if err := Validate(tasks, configPath); err != nil {
		return nil, err
	}

	return tasks, nil
}

// validationErrors collects multiple validation errors and returns them as a single error.
type validationErrors []string

func (ve validationErrors) Error() string {
	sort.Strings(ve)
	return strings.Join(ve, "\n")
}

// isEmptyTask returns true if the task has no commands defined (no built-in
// lifecycle commands, no custom actions, and no hooks).
func isEmptyTask(t lib.Task) bool {
	if t.Run != "" || t.Test != "" || t.Build != "" || t.Install != "" || t.Clean != "" {
		return false
	}
	if len(t.Custom) > 0 {
		return false
	}
	if t.Hooks.Pre != nil && len(t.Hooks.Pre) > 0 {
		return false
	}
	if t.Hooks.Post != nil && len(t.Hooks.Post) > 0 {
		return false
	}
	return true
}

// allActions returns the set of all valid action names for a task (built-in + custom).
func allActions(t lib.Task) map[string]bool {
	actions := make(map[string]bool)
	for name := range builtInActions {
		actions[name] = true
	}
	for name := range t.Custom {
		actions[name] = true
	}
	return actions
}

// Validate checks a parsed task map for configuration errors.
// It collects all validation errors and returns them as a single error.
// filePath is used in error messages to identify the source config file.
func Validate(tasks map[string]lib.Task, filePath string) error {
	var errs validationErrors

	for taskKey, task := range tasks {
		// 1. No empty task entries
		if isEmptyTask(task) {
			errs = append(errs, fmt.Sprintf("%s: task %q has no commands defined", filePath, taskKey))
			continue // skip further checks on this empty task
		}

		// 2. Custom action names must not collide with built-in names
		for customName := range task.Custom {
			if builtInActions[customName] {
				errs = append(errs, fmt.Sprintf("%s: task %q has custom action %q that conflicts with a built-in action", filePath, taskKey, customName))
			}

			// 3. Custom map values must be non-empty strings
			if strings.TrimSpace(task.Custom[customName]) == "" {
				errs = append(errs, fmt.Sprintf("%s: task %q has custom action %q with an empty command", filePath, taskKey, customName))
			}
		}

		// 4. Hook command values must be non-empty strings
		if task.Hooks.Pre != nil {
			for action, cmd := range task.Hooks.Pre {
				if strings.TrimSpace(cmd) == "" {
					errs = append(errs, fmt.Sprintf("%s: task %q has pre-hook for action %q with an empty command", filePath, taskKey, action))
				}
			}
		}
		if task.Hooks.Post != nil {
			for action, cmd := range task.Hooks.Post {
				if strings.TrimSpace(cmd) == "" {
					errs = append(errs, fmt.Sprintf("%s: task %q has post-hook for action %q with an empty command", filePath, taskKey, action))
				}
			}
		}

		// 5. Hook action names must reference valid actions (built-in or custom)
		validActions := allActions(task)
		if task.Hooks.Pre != nil {
			for action := range task.Hooks.Pre {
				if !validActions[action] {
					errs = append(errs, fmt.Sprintf("%s: task %q has pre-hook referencing undefined action %q", filePath, taskKey, action))
				}
			}
		}
		if task.Hooks.Post != nil {
			for action := range task.Hooks.Post {
				if !validActions[action] {
					errs = append(errs, fmt.Sprintf("%s: task %q has post-hook referencing undefined action %q", filePath, taskKey, action))
				}
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
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
