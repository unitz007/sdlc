package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"sdlc/config"
	"sdlc/engine"
	"sdlc/lib"
	"sdlc/watcher"

	"github.com/fsnotify/fsnotify"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// ExitCodeError wraps an error with a specific process exit code so that
// the top-level Execute() function can exit with the child process's code.
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	return e.Err.Error()
}

func (e *ExitCodeError) Unwrap() error {
	return e.Err
}

// flusher is an interface for writers that need to flush buffered content.
// It is satisfied by *PrefixWriter (from prefix_writer.go).
type flusher interface {
	Flush()
}

func init() {
	RootCmd.AddCommand(runCmd)
	RootCmd.AddCommand(testCmd)
	RootCmd.AddCommand(buildCmd)
	RootCmd.AddCommand(installCmd)
	RootCmd.AddCommand(cleanCmd)
	RootCmd.AddCommand(initCmd)
	RootCmd.AddCommand(listCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs your code",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeTask(cmd, "run")
	},
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Tests your code",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeTask(cmd, "test")
	},
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds your project",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeTask(cmd, "build")
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Installs project dependencies",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeTask(cmd, "install")
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Cleans build artifacts",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeTask(cmd, "clean")
	},
}

// RegisterDynamicCommands scans loaded config for custom actions and registers
// them as Cobra sub-commands. These appear in help output and execute the same
// pipeline as built-in commands (config load → project detect → filter → execute
// with hooks).
func RegisterDynamicCommands() {
	tasks := loadConfigForDiscovery()
	if tasks == nil {
		return
	}

	// Collect all custom action names across all tasks
	customActions := make(map[string]string) // action name -> description
	for _, task := range tasks {
		for name, cmd := range task.Custom {
			if _, exists := customActions[name]; !exists {
				customActions[name] = cmd
			}
		}
	}

	if len(customActions) == 0 {
		return
	}

	// Sort for deterministic ordering
	names := make([]string, 0, len(customActions))
	for name := range customActions {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		actionName := name
		cmdStr := customActions[name]
		dynamicCmd := &cobra.Command{
			Use:   actionName,
			Short: fmt.Sprintf("Custom command: %s", cmdStr),
			Long:  fmt.Sprintf("Runs the custom action '%s' defined in .sdlc.json.\nCommand: %s", actionName, cmdStr),
			RunE: func(cmd *cobra.Command, args []string) error {
				return executeTask(cmd, actionName)
			},
			GroupID: "custom",
		}
		RootCmd.AddCommand(dynamicCmd)
	}

	// Add a custom commands group to the help output
	RootCmd.AddGroup(&cobra.Group{
		ID:    "custom",
		Title: "Custom Commands:",
	})
}

// loadConfigForDiscovery loads config to discover custom actions for dynamic
// sub-command registration. It tries local config first, then global.
func loadConfigForDiscovery() map[string]lib.Task {
	wd, err := resolveWorkDir(workDir)
	if err != nil {
		return nil
	}

	var tasks map[string]lib.Task
	if cfgFile != "" {
		tasks, _ = config.Load(cfgFile)
	} else {
		tasks, _ = config.LoadLocal(wd)
		if tasks == nil {
			tasks, _ = config.Load("")
		}
	}
	return tasks
}

func executeTask(cmd *cobra.Command, action string) error {
	printBanner()
	// Resolve working directory
	wd, err := resolveWorkDir(workDir)
	if err != nil {
		return fmt.Errorf("directory error: %w", err)
	}

	// Create a context that cancels on SIGINT or SIGTERM
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runTask(ctx, wd, action)
}

// parseParallelFlag converts the --parallel string flag to a concurrency limit.
// Returns: -1 = flag not specified (caller should use sequential execution),
// 0 = unbounded concurrency (bare --parallel or --parallel=true),
// N = max N goroutines (e.g. 1 = sequential).
func parseParallelFlag(raw string) int {
	if raw == "" {
		return -1 // flag not specified: caller should execute sequentially
	}
	// When used as a boolean flag (--parallel with no value), NoOptDefValue sets it to "true"
	if strings.EqualFold(raw, "true") {
		return 0 // unbounded concurrency
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "Warning: invalid --parallel value %q, falling back to unbounded concurrency\n", raw)
		return 0 // invalid value, fall back to unbounded concurrency
	}
	return n
}

func runTask(ctx context.Context, wd, action string) error {
	// Load configuration
	var tasks map[string]lib.Task
	var err error

	if cfgFile != "" {
		tasks, err = config.LoadFromDir(cfgFile)
		if err != nil {
			return fmt.Errorf("config error: %w", err)
		}
	}

	if tasks == nil {
		// Try loading from working directory first
		tasks, err = config.LoadLocal(wd)
		if err != nil {
			return fmt.Errorf("local config error: %w", err)
		}
		if tasks == nil {
			// Fallback to global/home config
			tasks, err = config.Load("")
		}
	}

	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Detect projects with the configured detection depth
	projects, err := engine.DetectProjects(wd, tasks, detectionDepth)
	if err != nil {
		return fmt.Errorf("detection error: %w", err)
	}

	if len(projects) == 0 {
		return fmt.Errorf("no project configured or detected in %s", wd)
	}

	// Load root .sdlc.conf if available
	rootEnvConfig, err := config.LoadEnvConfig(wd)
	if err != nil {
		fmt.Printf("Warning: failed to load root .sdlc.conf: %v\n", err)
	}

	// Filter projects based on flags
	selectedProjects, err := filterProjects(projects)
	if err != nil {
		return err
	}

	if len(selectedProjects) == 0 {
		return fmt.Errorf("no projects matched the criteria")
	}

	if len(projects) > 1 {
		fmt.Printf("[SDLC] Multi-module project detected (%d modules):\n", len(projects))
		for i, p := range projects {
			// Check if project is in selectedProjects
			isSelected := false
			for _, sp := range selectedProjects {
				if sp.Path == p.Path {
					isSelected = true
					break
				}
			}

			if !isSelected {
				fmt.Printf(" %s✘ %s (%s) [IGNORED]%s\n", lib.Colorize("", lib.DarkGrey), p.Path, p.Name, lib.Colorize("", lib.Reset))
			} else {
				color := lib.ModuleColor(i)
				fmt.Printf(" %s✔%s %s (%s)\n", lib.Colorize("", lib.Green), lib.Colorize("", lib.Reset), lib.Colorize(p.Path, color), p.Name)
			}
		}
		fmt.Println()
	}

	if len(selectedProjects) > 1 && !runAllMods {
		fmt.Printf("[SDLC] Multiple projects detected. Running all modules by default.\n")
	}

	// Dry-run mode: simulate what would happen without executing commands
	if dryRun {
		fmt.Printf("[SDLC] DRY-RUN: would %s on %d module(s)\n", action, len(selectedProjects))
		for i, p := range selectedProjects {
			env, args := prepareProjectEnv(p, rootEnvConfig)
			cmdStr, err := p.Task.Command(action)
			if err != nil {
				fmt.Printf("[DRY-RUN] %s: invalid command for action %s: %v\n", p.Path, action, err)
				continue
			}
			if len(args) > 0 {
				cmdStr = cmdStr + " " + strings.Join(args, " ")
			}

			// Substitute environment variables in the command string
			keys := make([]string, 0, len(env))
			for k := range env {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				return len(keys[i]) > len(keys[j])
			})
			for _, k := range keys {
				v := env[k]
				cmdStr = strings.ReplaceAll(cmdStr, fmt.Sprintf("${%s}", k), v)
				cmdStr = strings.ReplaceAll(cmdStr, fmt.Sprintf("$%s", k), v)
			}

			color := lib.ModuleColor(i)
			prefix := fmt.Sprintf("[%s] ", lib.Colorize(p.Path, color))
			fmt.Printf(" - %s%s\n", prefix, cmdStr)

			// Show hooks in dry-run
			if preHook := p.Task.PreHook(action); preHook != "" {
				fmt.Printf(" - %s[pre-hook] %s\n", prefix, preHook)
			}
			if postHook := p.Task.PostHook(action); postHook != "" {
				fmt.Printf(" - %s[post-hook] %s\n", prefix, postHook)
			}
		}
		// Do not perform any actions in dry-run mode
		return nil
	}

	if watchMode {
		fmt.Printf("[SDLC] Watch mode enabled. Watching for changes in detected projects...\n")
		return watchAndRunLoop(ctx, selectedProjects, projects, action, rootEnvConfig)
	}

	// Parse the --parallel flag to determine execution mode
	parallelLimit := parseParallelFlag(parallelFlag.raw)
	multi := len(selectedProjects) > 1

	// Execute for each selected project
	summaryResults := make([]ModuleResult, len(selectedProjects))

	if parallelLimit < 0 {
		// --parallel not specified: execute modules sequentially (original behavior)
		for i, project := range selectedProjects {
			originalIdx := i
			for idx, p := range projects {
				if p.Path == project.Path {
					originalIdx = idx
					break
				}
			}

			env, args := prepareProjectEnv(project, rootEnvConfig)
			cmdStr, _ := resolveCommandString(project, action, env, args)
			err := runProject(ctx, project, originalIdx, action, env, args, multi)
			summaryResults[i] = ModuleResult{
				Path:       project.Path,
				Command:    cmdStr,
				Err:        err,
				ColorIndex: originalIdx,
			}
		}
	} else {
		// --parallel specified: execute modules concurrently with optional limit
		var wg sync.WaitGroup
		var sem chan struct{}
		if parallelLimit > 0 {
			sem = make(chan struct{}, parallelLimit)
		}

		for i, project := range selectedProjects {
			originalIdx := i
			for idx, p := range projects {
				if p.Path == project.Path {
					originalIdx = idx
					break
				}
			}

			wg.Add(1)
			go func(p engine.Project, index int, slot int) {
				defer wg.Done()
				if sem != nil {
					select {
					case sem <- struct{}{}:
						defer func() { <-sem }()
					case <-ctx.Done():
						return
					}
				}
				env, args := prepareProjectEnv(p, rootEnvConfig)
				cmdStr, _ := resolveCommandString(p, action, env, args)
				err := runProject(ctx, p, index, action, env, args, multi)
				summaryResults[slot] = ModuleResult{
					Path:       p.Path,
					Command:    cmdStr,
					Err:        err,
					ColorIndex: index,
				}
			}(project, originalIdx, i)
		}

		wg.Wait()
	}

	// Print summary table when two or more modules were executed
	if len(summaryResults) >= 2 {
		hasFailure := false
		for _, r := range summaryResults {
			if r.Err != nil {
				hasFailure = true
				break
			}
		}
		w := io.Writer(os.Stdout)
		if hasFailure {
			w = os.Stderr
		}
		printSummaryTable(summaryResults, w)
	}

	// Identify failed modules and determine overall exit code
	var failedModules []string
	var firstFailureCode int
	for _, r := range summaryResults {
		if r.Err != nil {
			failedModules = append(failedModules, r.Path)
			if firstFailureCode == 0 {
				var exitErr *ExitCodeError
				if errors.As(r.Err, &exitErr) {
					firstFailureCode = exitErr.Code
				} else {
					firstFailureCode = 1
				}
			}
		}
	}

	if len(failedModules) > 0 {
		fmt.Fprintf(os.Stderr, "[SDLC] %d module(s) failed: %s\n", len(failedModules), strings.Join(failedModules, ", "))
		code := 1
		if len(failedModules) == 1 {
			// Preserve the specific exit code from the single failed module
			// so CI/CD pipelines see the real failure code (e.g., 42 not 1).
			code = firstFailureCode
		}
		return &ExitCodeError{Code: code, Err: fmt.Errorf("%d module(s) failed", len(failedModules))}
	}

	return nil
}

// addWatchedDir recursively adds a directory tree to the fsnotify watcher,
// skipping directories and files that match the exclusion rules.
func addWatchedDir(w *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		if shouldSkipPath(path, true) {
			return filepath.SkipDir
		}

		if err := w.Add(path); err != nil {
			// Log but don't fail — some directories may not be watchable
			fmt.Printf("[SDLC] Warning: could not watch %s: %v\n", path, err)
			return nil
		}

		return nil
	})
}

// reverseDeps tracks which modules depend on each other (populated from .sdlc.conf).
var reverseDeps = make(map[string][]string)

// resolveProject finds a project by its path in the detected projects list.
var resolveProject = func(path string) (engine.Project, bool) {
	return engine.Project{}, false
}

// restartModule restarts a single module (stub — implemented by dependency tracking).
var restartModule = func(p engine.Project, reason string) {
	fmt.Printf("[SDLC] Restarting module %s: %s\n", p.Path, reason)
}

// watchAndRunLoop uses fsnotify to watch project directories for file changes
// and restarts projects when relevant files are modified. It debounces rapid
// successive file events into a single restart per 300ms window.
func watchAndRunLoop(ctx context.Context, projects []engine.Project, allProjects []engine.Project, action string, rootEnvConfig *config.EnvSettings) error {
	// Parse debounce duration, falling back to 500ms on error.
	parsedDebounce, err := time.ParseDuration(debounceDuration)
	if err != nil {
		fmt.Printf("[SDLC] Invalid debounce duration %q, defaulting to 500ms\n", debounceDuration)
		parsedDebounce = 500 * time.Millisecond
	}

	type projectState struct {
		cancel context.CancelFunc
		wg     *sync.WaitGroup
	}

	states := make(map[string]*projectState)
	var mu sync.Mutex

	// Helper to start (or restart) a project.
	startProject := func(p engine.Project) {
		mu.Lock()
		defer mu.Unlock()

		state, exists := states[p.Path]
		if exists {
			state.cancel()
			state.wg.Wait()
		}

		runCtx, cancel := context.WithCancel(ctx)
		wg := &sync.WaitGroup{}
		wg.Add(1)

		states[p.Path] = &projectState{
			cancel: cancel,
			wg:     wg,
		}

		idx := 0
		for i, original := range allProjects {
			if original.Path == p.Path {
				idx = i
				break
			}
		}

		go func() {
			defer wg.Done()
			env, args := prepareProjectEnv(p, rootEnvConfig)
			err := runProject(runCtx, p, idx, action, env, args, len(projects) > 1)
			if err != nil && err != context.Canceled {
				fmt.Printf("[SDLC] Module %s exited with error: %v\n", p.Name, err)
			}
		}()
	}

	// Build a lookup from project relative path → engine.Project for the callback.
	projectMap := make(map[string]engine.Project, len(projects))
	for _, p := range projects {
		projectMap[p.Path] = p
	}

	// Create the fsnotify-based watcher with per-project debouncing.
	w, err := watcher.NewWatcher(parsedDebounce, func(event watcher.ChangeEvent) {
		mu.Lock()
		defer mu.Unlock()

		p, ok := projectMap[event.ProjectPath]
		if !ok {
			return
		}

		// Cancel the existing run for this project and wait for it to finish.
		if state, exists := states[p.Path]; exists {
			state.cancel()
			state.wg.Wait()
		}

		color := lib.ModuleColor(0)
		for i, original := range allProjects {
			if original.Path == p.Path {
				color = lib.ModuleColor(i)
				break
			}
		}

		fmt.Printf("\n[SDLC] %sFile change detected: %s — restarting %s%s%s...\n",
			lib.Colorize("", lib.Yellow),
			filepath.Base(event.FilePath),
			lib.Colorize(p.Path, color),
			lib.Colorize("", lib.Reset),
			lib.Colorize("", lib.Reset),
		)

		// Start a fresh run for this project.
		runCtx, cancel := context.WithCancel(ctx)
		wg := &sync.WaitGroup{}
		wg.Add(1)

		states[p.Path] = &projectState{
			cancel: cancel,
			wg:     wg,
		}

		idx := 0
		for i, original := range allProjects {
			if original.Path == p.Path {
				idx = i
				break
			}

			handleEvent(event.Name, p)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("[SDLC] Watcher error: %v\n", err)
		}

		go func() {
			defer wg.Done()
			env, args := prepareProjectEnv(p, rootEnvConfig)
			err := runProject(runCtx, p, idx, action, env, args, len(projects) > 1)
			if err != nil && err != context.Canceled {
				fmt.Printf("[SDLC] Module %s exited with error: %v\n", p.Name, err)
			}
		}()
	})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer w.Close()

	// Add all selected projects to the watcher.
	for _, p := range projects {
		if err := w.AddProject(p.Path, p.AbsPath); err != nil {
			fmt.Printf("[SDLC] Warning: failed to watch %s: %v\n", p.AbsPath, err)
		}
	}

	// Initial start of all projects.
	for _, p := range projects {
		startProject(p)
	}

	// Block until the parent context is cancelled (e.g. Ctrl+C).
	if err := w.Watch(ctx); err != nil {
		return err
	}

	// Context cancelled — shut down all running projects.
	fmt.Println("[SDLC] Context cancelled, exiting watch loop")
	mu.Lock()
	for _, s := range states {
		s.cancel()
	}
	mu.Unlock()

	// Wait for all goroutines to finish with a timeout.
	done := make(chan struct{})
	go func() {
		mu.Lock()
		defer mu.Unlock()
		for _, s := range states {
			s.wg.Wait()
		}
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("[SDLC] All modules stopped gracefully")
	case <-time.After(5 * time.Second):
		fmt.Println("[SDLC] Timeout waiting for modules to stop")
	}

	return nil
}

func prepareProjectEnv(p engine.Project, rootEnvConfig *config.EnvSettings) (map[string]string, []string) {
	// Load module-level config
	modEnvConfig, _ := config.LoadEnvConfig(p.AbsPath)

	// Merge: module overrides root for env vars, module args appended after root args
	merged := config.MergeEnvSettings(rootEnvConfig, modEnvConfig)

	finalArgs := merged.Args

	// Append extra args from CLI (repeatable flag, each value split on whitespace)
	for _, ea := range extraArgs {
		finalArgs = append(finalArgs, strings.Fields(ea)...)
	}
	return merged.Env, finalArgs
}

// resolveCommandString builds the full command string for a project/action by
// resolving the base command, appending extra args, and substituting
// environment variables (longest keys first to avoid partial matches).
func resolveCommandString(p engine.Project, action string, env map[string]string, args []string) (string, error) {
	cmdStr, err := p.Task.Command(action)
	if err != nil {
		return "", err
	}

	cmdArgsStr := strings.Join(args, " ")
	if cmdArgsStr != "" {
		cmdStr += " " + cmdArgsStr
	}

	// Substitute environment variables in the command string
	substituteEnvVars(&cmdStr, env)

	// AC2: Run pre-hook if defined
	if preHookCmd := p.Task.PreHook(action); preHookCmd != "" {
		substituteEnvVars(&preHookCmd, env)
		if !multi {
			fmt.Printf("[SDLC] Running pre-hook for %s: %s\n", action, preHookCmd)
		}
		if err := runCommand(ctx, preHookCmd, p.AbsPath, out, errOut, env); err != nil {
			fmt.Fprintf(errOut, "Pre-hook failed (skipping main command): %v\n", err)
			// Run post-hook even on pre-hook failure
			runPostHookIfNeeded(ctx, p, action, env, out, errOut, multi)
			return fmt.Errorf("pre-hook for action %s failed: %w", action, err)
		}
	}

	// Run the main command
	mainErr := runCommand(ctx, cmdStr, p.AbsPath, out, errOut, env)
	if mainErr != nil {
		fmt.Fprintf(errOut, "Command failed: %v\n", mainErr)
	}

	// AC2: Run post-hook if defined (runs regardless of main command success/failure)
	runPostHookIfNeeded(ctx, p, action, env, out, errOut, multi)

	return mainErr
}

// runPostHookIfNeeded executes the post-hook for the given action if one is defined.
func runPostHookIfNeeded(ctx context.Context, p engine.Project, action string, env map[string]string, out, errOut io.Writer, multi bool) {
	postHookCmd := p.Task.PostHook(action)
	if postHookCmd == "" {
		return
	}
	substituteEnvVars(&postHookCmd, env)
	if !multi {
		fmt.Printf("[SDLC] Running post-hook for %s: %s\n", action, postHookCmd)
	}
	if err := runCommand(ctx, postHookCmd, p.AbsPath, out, errOut, env); err != nil {
		fmt.Fprintf(errOut, "Post-hook failed: %v\n", err)
	}
}

// substituteEnvVars replaces $KEY and ${KEY} patterns in cmdStr with values from env.
// Longer keys are replaced first to avoid partial matches.
func substituteEnvVars(cmdStr *string, env map[string]string) {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	for _, k := range keys {
		v := env[k]
		*cmdStr = strings.ReplaceAll(*cmdStr, fmt.Sprintf("${%s}", k), v)
		*cmdStr = strings.ReplaceAll(*cmdStr, fmt.Sprintf("$%s", k), v)
	}

	return cmdStr, nil
}

func runProject(ctx context.Context, p engine.Project, index int, action string, env map[string]string, args []string, multi bool) error {
	// Clean up .vite-temp if it exists, to prevent EPERM errors on restart
	viteTemp := filepath.Join(p.AbsPath, "node_modules", ".vite-temp")
	if _, err := os.Stat(viteTemp); err == nil {
		fmt.Printf("[SDLC] Cleaning up %s\n", viteTemp)
		if err := os.RemoveAll(viteTemp); err != nil {
			fmt.Printf("[SDLC] Warning: failed to clean up %s: %v\n", viteTemp, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	color := lib.ModuleColor(index)
	prefix := fmt.Sprintf("[%s] ", lib.Colorize(p.Path, color))
	var out, errOut io.Writer

	if multi {
		out = NewPrefixWriter(os.Stdout, prefix)
		errOut = NewPrefixWriter(os.Stderr, prefix)
	} else {
		out = os.Stdout
		errOut = os.Stderr
		fmt.Printf("[SDLC] Executing %s for module: %s\n", action, p.Path)
	}

	// Resolve the full command string (base command + args + env substitution)
	cmdStr, err := resolveCommandString(p, action, env, args)
	if err != nil {
		fmt.Fprintf(errOut, "Error getting command: %v\n", err)
		return err
	}

	if verbose {
		var target io.Writer
		if multi {
			target = errOut
		} else {
			target = os.Stderr
		}
		if len(env) > 0 {
			envKeys := make([]string, 0, len(env))
			for k := range env {
				envKeys = append(envKeys, k)
			}
			sort.Strings(envKeys)
			for _, k := range envKeys {
				if multi {
					fmt.Fprintf(target, "ENV: %s=%s\n", k, env[k])
				} else {
					fmt.Fprintf(target, "%sENV: %s=%s\n", prefix, k, env[k])
				}
			}
		}
		if multi {
			fmt.Fprintf(target, "$ %s\n", cmdStr)
		} else {
			fmt.Fprintf(target, "%s$ %s\n", prefix, cmdStr)
		}
	}

	// Run the command
	if err := runCommand(ctx, cmdStr, p.AbsPath, out, errOut, env); err != nil {
		fmt.Fprintf(errOut, "Command failed: %v\n", err)
		var exitErr *ExitCodeError
		if errors.As(err, &exitErr) {
			return exitErr
		}
		return &ExitCodeError{Code: 1, Err: err}
	}
	return nil
}

// PrefixWriter wraps an io.Writer and prefixes each line with a given prefix
type PrefixWriter struct {
	w       io.Writer
	prefix  []byte
	midLine bool
}

func NewPrefixWriter(w io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{
		w:      w,
		prefix: []byte(prefix),
	}
}

func (pw *PrefixWriter) Write(p []byte) (n int, err error) {
	lines := bytes.SplitAfter(p, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var buf []byte
		if !pw.midLine {
			buf = append(buf, pw.prefix...)
			buf = append(buf, line...)
		} else {
			buf = line
		}

		if _, err := pw.w.Write(buf); err != nil {
			return 0, err
		}

		pw.midLine = !bytes.HasSuffix(line, []byte("\n"))
	}
	return len(p), nil
}

func filterProjects(projects []engine.Project) ([]engine.Project, error) {
	// Handle --module flag: select exactly one module by path or name
	if targetMod != "" {
		for _, p := range projects {
			if p.Path == targetMod || p.Name == targetMod {
				return []engine.Project{p}, nil
			}
		}
		return nil, fmt.Errorf("module %q not found. Available modules:\n  %s", targetMod, strings.Join(projectPaths(projects), "\n  "))
	}

	// Handle --ignore flags: exclude matching modules
	if len(ignoreMods) > 0 {
		var filtered []engine.Project
		for _, p := range projects {
			ignored := false
			for _, ignore := range ignoreMods {
				if p.Path == ignore || p.Name == ignore {
					ignored = true
					break
				}
			}
			if !ignored {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}

	// --all flag or default: return all remaining projects
	return projects, nil
}

// projectPaths returns a slice of relative paths for display in error messages.
func projectPaths(projects []engine.Project) []string {
	paths := make([]string, len(projects))
	for i, p := range projects {
		paths[i] = p.Path
	}
	return paths
}

func promptModuleSelection(projects []engine.Project) ([]engine.Project, error) {
	// If interactive mode is not possible (e.g. non-terminal), default to all
	// For now, we assume terminal is available if we are here.

	selected := make(map[int]bool)
	// Default to all selected initially.
	for i := range projects {
		selected[i] = true
	}

	for {
		items := []string{"[Done] Run selected modules"}
		for i, p := range projects {
			prefix := "[ ]"
			if selected[i] {
				prefix = "[x]"
			}
			items = append(items, fmt.Sprintf("%s %s (%s)", prefix, p.Name, p.Path))
		}

		prompt := promptui.Select{
			Label:       "Select modules to run (Select to toggle)",
			Items:       items,
			Size:        len(items) + 1,
			HideSelected: true,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return nil, fmt.Errorf("prompt failed: %w", err)
		}

		if idx == 0 {
			break
		}

		// Toggle selection
		projectIdx := idx - 1
		selected[projectIdx] = !selected[projectIdx]
	}

	var result []engine.Project
	for i, p := range projects {
		if selected[i] {
			result = append(result, p)
		} else {
			ignoreMods = append(ignoreMods, p.Path)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no modules selected")
	}

	return result, nil
}

func printBanner() {
	banner := `
   _____ ____  __    ______
  / ___// __ \\/ /   / ____/
  \\__ \\/ / / / /   / /     
 ___/ / /_/ / /___/ /___   
/____/_____/_____/\\____/   
`
	fmt.Println(lib.Colorize(banner, lib.Cyan))
}
