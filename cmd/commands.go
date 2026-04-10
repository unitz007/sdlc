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

// dynamicCommands tracks which custom actions have been registered as
// Cobra sub-commands to avoid double-registration.
var dynamicCommands map[string]bool

// registerDynamicCommands loads the project config and registers any custom
// actions as Cobra sub-commands on the root command.
func registerDynamicCommands(workDir string) {
	if dynamicCommands != nil {
		return // Already registered
	}
	dynamicCommands = make(map[string]bool)

	// Load configuration to discover custom actions
	tasks, err := loadTasks(workDir)
	if err != nil {
		return // Don't fail — dynamic commands are optional
	}
	if tasks == nil {
		return
	}

	// Collect all unique custom action names across all project types
	customActions := make(map[string]bool)
	for _, task := range tasks {
		for _, action := range task.CustomActions() {
			customActions[action] = true
		}
	}

	if len(customActions) == 0 {
		return
	}

	// Sort for deterministic ordering
	sortedActions := make([]string, 0, len(customActions))
	for a := range customActions {
		sortedActions = append(sortedActions, a)
	}
	sort.Strings(sortedActions)

	for _, action := range sortedActions {
		action := action // capture for closure
		dynamicCommands[action] = true
		cmd := &cobra.Command{
			Use:   action,
			Short: fmt.Sprintf("Custom command: %s", action),
			RunE: func(cmd *cobra.Command, args []string) error {
				return executeTask(cmd, action)
			},
		}
		RootCmd.AddCommand(cmd)
	}
}

// loadTasks is a helper to load tasks from config (used for dynamic command discovery).
func loadTasks(workDir string) (map[string]lib.Task, error) {
	var tasks map[string]lib.Task
	var err error

	if cfgFile != "" {
		tasks, err = config.Load(cfgFile)
	} else {
		tasks, err = config.LoadLocal(workDir)
		if err != nil {
			return nil, err
		}
		if tasks == nil {
			tasks, err = config.Load("")
		}
	}

	return tasks, err
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
				if sp.Path == sp.Path {
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

func watchAndRunLoop(ctx context.Context, projects []engine.Project, allProjects []engine.Project, action string, rootEnvConfig *config.EnvSettings) error {
	fmt.Println("[SDLC] Starting smart watchAndRunLoop")
	defer fmt.Println("[SDLC] Exiting watchAndRunLoop")

	type projectState struct {
		cancel  context.CancelFunc
		wg      *sync.WaitGroup
		lastMod time.Time
	}

	states := make(map[string]*projectState)
	var mu sync.Mutex

	// Helper to start (or restart) a project
	startProject := func(p engine.Project) {
		mu.Lock()
		defer mu.Unlock()

		state, exists := states[p.Path]
		if exists {
			// Stop existing
			state.cancel()
			state.wg.Wait()
			// Add a small delay to ensure file handles are released
			time.Sleep(500 * time.Millisecond)
		}

		// New context
		runCtx, cancel := context.WithCancel(ctx)
		wg := &sync.WaitGroup{}
		wg.Add(1)

		newState := &projectState{
			cancel:  cancel,
			wg:      wg,
			lastMod: time.Now(),
		}
		states[p.Path] = newState

		// Find original index for coloring
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
			// Pass a derived context that handles cancellation properly
			err := runProject(runCtx, p, idx, action, env, args, len(projects) > 1)
			if err != nil && err != context.Canceled {
				fmt.Printf("[SDLC] Module %s exited with error: %v\n", p.Name, err)
			}
		}()
	}

	// Initial start
	for _, p := range projects {
		startProject(p)
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

			// Wait for all goroutines to finish with a timeout
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
			// Check for changes
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
					fmt.Printf("\n[SDLC] File change detected: %s in %s. Restarting module...\n", filepath.Base(changedFile), p.Path)
					startProject(p)
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
	cmdStr = substituteEnv(cmdStr, env)

	// Run the main command
	mainErr := runCommand(ctx, cmdStr, p.AbsPath, out, errOut, env)
	if mainErr != nil {
		fmt.Fprintf(errOut, "Command failed: %v\n", mainErr)
	}

	// Execute post-hook (always runs, even if the main command failed)
	postErr := runHook(ctx, "post", action, p, env, out, errOut, multi)
	if postErr != nil {
		// Post-hook failure is reported but doesn't override the main error
		fmt.Fprintf(errOut, "[SDLC] Warning: post-%s hook failed: %v\n", action, postErr)
	}

	// Return the main command error (if any)
	return mainErr
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

	// Otherwise return empty list (caller will handle ambiguous case)
	// Actually, returning all projects here and letting the caller decide
	// based on count is better for the error message "multiple projects found"
	return projects, nil
}

func promptModuleSelection(projects []engine.Project) ([]engine.Project, error) {
	// If interactive mode is not possible (e.g. non-terminal), default to all
	// For now, we assume terminal is available if we are here.

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
			Label: "Select modules to run (Select to toggle)",
			Items: items,
			Size:  len(items) + 1,
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
  / ___// __ \/ /   / ____/
  \__ \/ / / / /   / /     
 ___/ / /_/ / /___/ /___   
/____/_____/_____/\____/   
`
	fmt.Println(colorCyan + banner + colorReset)
}
