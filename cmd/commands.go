package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"sdlc/config"
	"sdlc/engine"
	"sdlc/lib"

	"github.com/fsnotify/fsnotify"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const (
	colorReset    = "\\033[0m"
	colorRed      = "\\033[31m"
	colorGreen    = "\\033[32m"
	colorYellow   = "\\033[33m"
	colorBlue     = "\\033[34m"
	colorMagenta  = "\\033[35m"
	colorCyan     = "\\033[36m"
	colorWhite    = "\\033[37m"
	colorDarkGrey = "\\033[90m"
)

// watchDebounceInterval is the time to wait after the last file event
// before triggering a restart. This coalesces rapid successive saves
// (e.g., editor auto-save) into a single restart.
const watchDebounceInterval = 300 * time.Millisecond

var moduleColors = []string{
	colorCyan,
	colorGreen,
	colorMagenta,
	colorYellow,
	colorBlue,
}

func getModuleColor(index int) string {
	return moduleColors[index%len(moduleColors)]
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

func runTask(ctx context.Context, wd, action string) error {
	// Load configuration
	var tasks map[string]lib.Task
	var err error

	if cfgFile != "" {
		tasks, err = config.Load(cfgFile)
	} else {
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

	// Interactive selection if multiple projects found and no specific flags were set
	if len(selectedProjects) > 1 && !runAllMods && targetMod == "" && len(ignoreMods) == 0 {
		// This is the case where we want to prompt
		selectedProjects, err = promptModuleSelection(selectedProjects)
		if err != nil {
			return err
		}
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
				fmt.Printf(" %s✘ %s (%s) [IGNORED]%s\n", colorDarkGrey, p.Path, p.Name, colorReset)
			} else {
				color := getModuleColor(i)
				fmt.Printf(" %s✔%s %s%s%s (%s)\n", colorGreen, colorReset, color, p.Path, colorReset, p.Name)
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

			color := getModuleColor(i)
			prefix := fmt.Sprintf("[%s%s%s] ", color, p.Path, colorReset)
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

	// Execute for each selected project once
	var wg sync.WaitGroup
	for i, project := range selectedProjects {
		wg.Add(1)

		// Find the correct index in the original projects list for consistent coloring
		originalIdx := i
		for idx, p := range projects {
			if p.Path == project.Path {
				originalIdx = idx
				break
			}
		}

		go func(p engine.Project, index int) {
			defer wg.Done()

			env, args := prepareProjectEnv(p, rootEnvConfig)
			runProject(ctx, p, index, action, env, args, len(selectedProjects) > 1)
		}(project, originalIdx)
	}

	wg.Wait()
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
	fmt.Println("[SDLC] Starting smart watchAndRunLoop")
	defer fmt.Println("[SDLC] Exiting watchAndRunLoop")

	type projectState struct {
		cancel      context.CancelFunc
		wg          *sync.WaitGroup
		lastMod     time.Time
		debounce    *time.Timer
		changedFile string
	}

	states := make(map[string]*projectState)
	var mu sync.Mutex

	// Create a single fsnotify watcher for all projects
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// Build a map from watched directory to the project that owns it
	// (a directory may belong to only one project since projects have distinct paths)
	dirToProject := make(map[string]*engine.Project)

	for i := range projects {
		p := &projects[i]
		if err := addWatchedDir(watcher, p.AbsPath); err != nil {
			fmt.Printf("[SDLC] Warning: error watching %s: %v\n", p.AbsPath, err)
		}
		// Walk again to populate dirToProject map
		_ = filepath.Walk(p.AbsPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return nil
			}
			if shouldSkipPath(path, true) {
				return filepath.SkipDir
			}
			dirToProject[path] = p
			return nil
		})
	}

	// Helper to start (or restart) a project
	startProject := func(p engine.Project) {
		mu.Lock()
		defer mu.Unlock()

		state, exists := states[p.Path]
		if exists {
			state.cancel()
			state.wg.Wait()
			time.Sleep(500 * time.Millisecond)
		}

		runCtx, cancel := context.WithCancel(ctx)
		wg := &sync.WaitGroup{}
		wg.Add(1)

		newState := &projectState{
			cancel:  cancel,
			wg:      wg,
			lastMod: time.Now(),
		}
		states[p.Path] = newState

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

	// Helper to restart a module and all modules that depend on it (cascade).
	restartWithCascade := func(p engine.Project, reason string) {
		// Collect the module itself plus all reverse dependencies
		restartSet := make(map[string]bool)
		restartSet[p.Path] = true

		// BFS to find all transitive dependents
		queue := []string{p.Path}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, dependent := range reverseDeps[current] {
				if !restartSet[dependent] {
					restartSet[dependent] = true
					queue = append(queue, dependent)
				}
			}
		}

		// Restart the source module
		restartModule(p, reason)

		// Restart all dependents
		for depPath := range restartSet {
			if depPath == p.Path {
				continue
			}
			if depProject, ok := resolveProject(depPath); ok {
				restartModule(depProject, fmt.Sprintf("dependency %s changed", p.Path))
			}
		}
	}

	// isChildOfAnyModule returns true if the given file path is inside any of the
	// selected project directories. This prevents root-level checks from triggering
	// restarts for changes that are already handled by per-module checks.
	isChildOfAnyModule := func(filePath string) bool {
		for _, p := range projects {
			if strings.HasPrefix(filePath, p.AbsPath+string(filepath.Separator)) {
				return true
			}
		}
		return false
	}
	_ = isChildOfAnyModule
	_ = restartWithCascade // referenced by future cascade restart logic

	// Initial start of all selected projects
	for _, p := range projects {
		mu.Lock()
		runCtx, cancel := context.WithCancel(ctx)
		wg := &sync.WaitGroup{}
		wg.Add(1)

		newState := &projectState{
			cancel:  cancel,
			wg:      wg,
			lastMod: time.Now(),
		}
		states[p.Path] = newState
		mu.Unlock()

		idx := 0
		for i, original := range allProjects {
			if original.Path == p.Path {
				idx = i
				break
			}
		}

		go func(proj engine.Project, index int) {
			defer wg.Done()
			env, args := prepareProjectEnv(proj, rootEnvConfig)
			err := runProject(runCtx, proj, index, action, env, args, len(projects) > 1)
			if err != nil && err != context.Canceled {
				fmt.Printf("[SDLC] Module %s exited with error: %v\n", proj.Name, err)
			}
		}(p, idx)
	}

	// Helper to find which project owns a given file path.
	// It walks up the directory tree to find the longest matching watched directory.
	findProject := func(filePath string) *engine.Project {
		dir := filepath.Dir(filePath)
		for {
			if p, ok := dirToProject[dir]; ok {
				return p
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
		return nil
	}

	// Helper to handle a file event for a project with debouncing
	handleEvent := func(filePath string, p *engine.Project) {
		mu.Lock()
		state, ok := states[p.Path]
		if !ok {
			mu.Unlock()
			return
		}

		// Store the latest changed file name
		state.changedFile = filePath

		// Reset debounce timer
		if state.debounce != nil {
			state.debounce.Stop()
		}
		state.debounce = time.AfterFunc(watchDebounceInterval, func() {
			mu.Lock()
			changedFile := state.changedFile
			if state.debounce != nil {
				state.debounce.Stop()
				state.debounce = nil
			}
			mu.Unlock()

			fmt.Printf("\n[SDLC] File change detected: %s in %s. Restarting module...\n", filepath.Base(changedFile), p.Path)
			startProject(*p)
		})
		mu.Unlock()
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Println("[SDLC] Context cancelled, exiting watch loop")

			// Stop all debounce timers
			mu.Lock()
			for _, s := range states {
				if s.debounce != nil {
					s.debounce.Stop()
				}
				s.cancel()
			}
			mu.Unlock()

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

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only react to Write and Create events
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Check if the file should be skipped
			info, err := os.Stat(event.Name)
			if err != nil {
				continue
			}

			if info.IsDir() {
				// If a new directory was created, add it to the watcher
				if event.Op&fsnotify.Create != 0 && !shouldSkipPath(event.Name, true) {
					if err := watcher.Add(event.Name); err == nil {
						// Find which project this directory belongs to
						// by walking up from the parent of the new directory
						p := findProject(event.Name)
						if p != nil {
							dirToProject[event.Name] = p
						}
					}
				}
				continue
			}

			// Skip ignored files
			if shouldSkipPath(event.Name, false) {
				continue
			}

			// Find the project this file belongs to
			p := findProject(event.Name)
			if p == nil {
				continue
			}

			handleEvent(event.Name, p)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("[SDLC] Watcher error: %v\n", err)
		}
	}
}

func prepareProjectEnv(p engine.Project, rootEnvConfig *config.EnvSettings) (map[string]string, []string) {
	finalEnv := make(map[string]string)
	finalArgs := []string{}

	// Apply root config
	if rootEnvConfig != nil {
		for k, v := range rootEnvConfig.Env {
			finalEnv[k] = v
		}
		finalArgs = append(finalArgs, rootEnvConfig.Args...)
	}

	// Apply module config
	modEnvConfig, err := config.LoadEnvConfig(p.AbsPath)
	if err == nil && modEnvConfig != nil {
		for k, v := range modEnvConfig.Env {
			finalEnv[k] = v
		}
		finalArgs = append(finalArgs, modEnvConfig.Args...)
	}

	// Append extra args from CLI
	if extraArgs != "" {
		finalArgs = append(finalArgs, strings.Split(extraArgs, " ")...)
	}
	return finalEnv, finalArgs
}

// substituteEnv replaces $KEY and ${KEY} patterns in a string with values
// from the environment map. Keys are sorted by length (longest first) to
// avoid partial replacements.
func substituteEnv(s string, env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	for _, k := range keys {
		v := env[k]
		s = strings.ReplaceAll(s, fmt.Sprintf("${%s}", k), v)
		s = strings.ReplaceAll(s, fmt.Sprintf("$%s", k), v)
	}
	return s
}

// runHook executes a pre/post hook command for the given project and action.
// It returns nil if the hook succeeded, or an error if the hook failed.
func runHook(ctx context.Context, phase, action string, p engine.Project, env map[string]string, out, errOut io.Writer, multi bool) error {
	hookCmd := p.Task.Hooks.Hook(phase, action)
	if hookCmd == "" {
		return nil // No hook defined for this phase/action
	}

	colorIdx := 0
	color := getModuleColor(colorIdx)
	prefix := fmt.Sprintf("[%s%s] ", color, p.Path)

	hookCmd = substituteEnv(hookCmd, env)

	if multi {
		fmt.Fprintf(out, "%sExecuting %s_%s hook...\n", prefix, phase, action)
	} else {
		fmt.Fprintf(out, "[SDLC] Executing %s_%s hook for module: %s\n", phase, action, p.Path)
	}

	if err := runCommand(ctx, hookCmd, p.AbsPath, out, errOut, env); err != nil {
		if multi {
			fmt.Fprintf(errOut, "%s%s_%s hook failed: %v%s\n", prefix, phase, action, err, colorReset)
		} else {
			fmt.Fprintf(errOut, "[SDLC] %s_%s hook failed: %v\n", phase, action, err)
		}
		return err
	}

	if multi {
		fmt.Fprintf(out, "%s%s_%s hook completed.\n", prefix, phase, action)
	} else {
		fmt.Fprintf(out, "[SDLC] %s_%s hook completed.\n", phase, action)
	}
	return nil
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

	color := getModuleColor(index)
	prefix := fmt.Sprintf("[%s%s%s] ", color, p.Path, colorReset)
	var out, errOut io.Writer

	if multi {
		out = NewPrefixWriter(os.Stdout, prefix)
		errOut = NewPrefixWriter(os.Stderr, prefix)
	} else {
		out = os.Stdout
		errOut = os.Stderr
		fmt.Printf("[SDLC] Executing %s for module: %s%s%s\n", action, color, p.Path, colorReset)
	}

	// Execute pre-hook (if defined)
	// If the pre-hook fails, skip the main command
	if err := runHook(ctx, "pre", action, p, env, out, errOut, multi); err != nil {
		return fmt.Errorf("pre-%s hook failed, skipping %s: %w", action, action, err)
	}

	// Construct command arguments string
	cmdArgsStr := strings.Join(args, " ")

	// Execute command
	cmdStr, err := p.Task.Command(action)
	if err != nil {
		fmt.Fprintf(errOut, "Error getting command: %v\n", err)
		return err
	}

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
}

// hasChanges checks if any file in root has been modified since sinceTime
func hasChanges(root string, sinceTime time.Time) (bool, string, error) {
	var changed bool
	var changedFile string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip .git, .idea, etc.
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			// Skip common build/dependency directories
			if info.Name() == "node_modules" || info.Name() == "dist" || info.Name() == "build" || info.Name() == "target" || info.Name() == "bin" || info.Name() == "pkg" {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip log files and other temporary artifacts
		if strings.HasSuffix(info.Name(), ".log") || strings.HasSuffix(info.Name(), ".tmp") || strings.HasSuffix(info.Name(), ".lock") || strings.HasSuffix(info.Name(), ".pid") || strings.HasSuffix(info.Name(), ".swp") {
			return nil
		}

		if info.ModTime().After(sinceTime) {
			changed = true
			changedFile = path
			return io.EOF // Stop walking
		}
		return nil
	})

	// Return the main command error (if any)
	return mainErr
}

func filterProjects(projects []engine.Project) ([]engine.Project, error) {
	// Handle ignore flags
	if len(ignoreMods) > 0 {
		if len(projects) <= 1 {
			return nil, fmt.Errorf("--ignore flag is only supported in multi-module projects")
		}

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

	if runAllMods {
		return projects, nil
	}

	if targetMod != "" {
		for _, p := range projects {
			if p.Path == targetMod {
				return []engine.Project{p}, nil
			}
		}
		return []engine.Project{}, nil
	}

	// If only one project exists, default to it
	if len(projects) == 1 {
		return projects, nil
	}

	// Otherwise return all projects (caller will handle multi-module case)
	return projects, nil
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
	fmt.Println(colorCyan + banner + colorReset)
}
