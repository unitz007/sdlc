package cmd

import (
	"bytes"
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

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const (
	colorReset    = "\033[0m"
	colorRed      = "\033[31m"
	colorGreen    = "\033[32m"
	colorYellow   = "\033[33m"
	colorBlue     = "\033[34m"
	colorMagenta  = "\033[35m"
	colorCyan     = "\033[36m"
	colorWhite    = "\033[37m"
	colorDarkGrey = "\033[90m"

	// debounceInterval is the time window to coalesce rapid file changes
	// into a single restart per module.
	debounceInterval = 1 * time.Second
)

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

	// Detect projects
	projects, err := engine.DetectProjects(wd, tasks)
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
		}
		// Do not perform any actions in dry-run mode
		return nil
	}

	if watchMode {
		fmt.Printf("[SDLC] Watch mode enabled. Watching for changes in detected projects...\n")
		return watchAndRunLoop(ctx, selectedProjects, projects, action, rootEnvConfig, wd)
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

// projectState tracks the running state of a module in watch mode.
type projectState struct {
	cancel  context.CancelFunc
	wg      *sync.WaitGroup
	lastMod time.Time
}

// watchAndRunLoop implements smart partial restarts with dependency cascading,
// root directory change propagation, config file monitoring, and debounce.
func watchAndRunLoop(ctx context.Context, projects []engine.Project, allProjects []engine.Project, action string, rootEnvConfig *config.EnvSettings, workDir string) error {
	fmt.Println("[SDLC] Starting smart watchAndRunLoop")
	defer fmt.Println("[SDLC] Exiting watchAndRunLoop")

	states := make(map[string]*projectState)
	var mu sync.Mutex

	// --- Build dependency graph ---
	// moduleDeps maps each selected project path to its declared dependencies (from .sdlc.conf)
	moduleDeps := make(map[string][]string)
	// Load per-module env config to discover dependencies
	moduleEnvConfigs := make(map[string]*config.EnvSettings)
	for _, p := range projects {
		modCfg, err := config.LoadEnvConfig(p.AbsPath)
		if err != nil {
			fmt.Printf("[SDLC] Warning: failed to load config for %s: %v\n", p.Path, err)
			modCfg = &config.EnvSettings{}
		}
		if modCfg == nil {
			modCfg = &config.EnvSettings{}
		}
		moduleEnvConfigs[p.Path] = modCfg
		if len(modCfg.Depends) > 0 {
			moduleDeps[p.Path] = modCfg.Depends
		}
	}

	// Build reverse-dependency map: reverseDeps[depPath] = list of paths that depend on it
	reverseDeps := make(map[string][]string)
	for _, p := range projects {
		if deps, ok := moduleDeps[p.Path]; ok {
			for _, dep := range deps {
				reverseDeps[dep] = append(reverseDeps[dep], p.Path)
			}
		}
	}

	// Log dependency information
	for _, p := range projects {
		if deps, ok := moduleDeps[p.Path]; ok {
			fmt.Printf("[SDLC] %s depends on: %s\n", p.Path, strings.Join(deps, ", "))
		}
	}

	// --- Debounce state ---
	// pendingRestart tracks modules that have changes detected but not yet acted on.
	// The value is the time of the first detected change; restart is scheduled after debounceInterval.
	pendingRestart := make(map[string]time.Time)

	// Helper to resolve a module path to the full Project, checking selected projects.
	resolveProject := func(path string) (engine.Project, bool) {
		for _, p := range projects {
			if p.Path == path {
				return p, true
			}
		}
		return engine.Project{}, false
	}

	// Helper to restart a single module and log the reason.
	restartModule := func(p engine.Project, reason string) {
		fmt.Printf("[SDLC] Restarting %s: %s\n", p.Path, reason)
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

	// statesRootLastMod returns the minimum lastMod time across all module states,
	// used for checking if the root directory has changes not already covered by
	// per-module checks.
	statesRootLastMod := func() time.Time {
		earliest := time.Now()
		for _, s := range states {
			if s.lastMod.Before(earliest) {
				earliest = s.lastMod
			}
		}
		return earliest
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

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("[SDLC] Context cancelled, exiting watch loop")
			mu.Lock()
			for _, s := range states {
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

		case <-ticker.C:
			now := time.Now()

			// --- Check root directory for changes (propagate to all modules) ---
			changed, changedFile, err := hasChanges(workDir, statesRootLastMod())
			if err != nil {
				fmt.Printf("[SDLC] Watch error in root directory: %v\n", err)
			} else if changed && changedFile != "" && !isChildOfAnyModule(changedFile) {
				// Root change not belonging to any specific module — restart all
				fmt.Printf("\n[SDLC] File change detected in root: %s. Restarting all modules...\n", filepath.Base(changedFile))
				for _, p := range projects {
					restartModule(p, fmt.Sprintf("root file %s changed", filepath.Base(changedFile)))
				}
				// Clear all pending restarts since we just restarted everything
				for k := range pendingRestart {
					delete(pendingRestart, k)
				}
				continue
			}

			// --- Check each module for changes ---
			for _, p := range projects {
				mu.Lock()
				s, ok := states[p.Path]
				mu.Unlock()

				if !ok {
					continue
				}

				changed, changedFile, err := hasChanges(p.AbsPath, s.lastMod)
				if err != nil {
					fmt.Printf("[SDLC] Watch error in %s: %v\n", p.Path, err)
					continue
				}

				if changed {
					// Record pending restart with debounce
					if _, alreadyPending := pendingRestart[p.Path]; !alreadyPending {
						pendingRestart[p.Path] = now
						if changedFile != "" {
							fmt.Printf("\n[SDLC] File change detected: %s in %s (debouncing)...\n", filepath.Base(changedFile), p.Path)
						} else {
							fmt.Printf("\n[SDLC] File change detected in %s (debouncing)...\n", p.Path)
						}
					}

					// Update the lastMod time immediately so we don't re-detect
					// the same file on the next tick
					mu.Lock()
					if state, exists := states[p.Path]; exists {
						state.lastMod = time.Now()
					}
					mu.Unlock()
				}
			}

			// --- Process debounced restarts ---
			var modulesToRestart []string
			for modPath, firstChange := range pendingRestart {
				if now.Sub(firstChange) >= debounceInterval {
					modulesToRestart = append(modulesToRestart, modPath)
				}
			}

			// Restart modules that have passed the debounce window
			for _, modPath := range modulesToRestart {
				delete(pendingRestart, modPath)

				if p, ok := resolveProject(modPath); ok {
					restartWithCascade(p, "file change detected")
				}
			}
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

	// Run the command
	if err := runCommand(ctx, cmdStr, p.AbsPath, out, errOut, env); err != nil {
		fmt.Fprintf(errOut, "Command failed: %v\n", err)
		return err
	}
	return nil
}

// hasChanges checks if any file in root has been modified since sinceTime.
// It watches SDLC config files (.sdlc.conf, .sdlc.json) in addition to
// regular source files, while still skipping hidden directories and build artifacts.
func hasChanges(root string, sinceTime time.Time) (bool, string, error) {
	var changed bool
	var changedFile string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip known hidden/special directories
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				// Skip all dot-directories (including .git, .idea, .planner, etc.)
				return filepath.SkipDir
			}
			// Skip common build/dependency directories
			if info.Name() == "node_modules" || info.Name() == "dist" || info.Name() == "build" || info.Name() == "target" || info.Name() == "bin" || info.Name() == "pkg" {
				return filepath.SkipDir
			}
			return nil
		}

		// Allow SDLC config files through (they are dot-prefixed files, not directories)
		if info.Name() == ".sdlc.conf" || info.Name() == ".sdlc.json" {
			if info.ModTime().After(sinceTime) {
				changed = true
				changedFile = path
				return io.EOF // Stop walking
			}
			return nil
		}

		// Skip other hidden files
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

	if err == io.EOF {
		return true, changedFile, nil
	}
	return changed, "", err
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
	selected := make(map[int]bool)
	// Default to all selected initially
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
			Label:        "Select modules to run (Select to toggle)",
			Items:        items,
			Size:         len(items) + 1,
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
  \__ \/ / / / /   / /     
 ___/ / /_/ / /___/ /___   
/____/_____/_____/\\____/   
`
	fmt.Println(colorCyan + banner + colorReset)
}
