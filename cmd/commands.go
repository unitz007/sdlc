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
		selectedProjects, err = promptModuleSelection(selectedProjects)
		if err != nil {
			return err
		}
	}

	if len(projects) > 1 {
		fmt.Printf("[SDLC] Multi-module project detected (%d modules):\n", len(projects))
		for i, p := range projects {
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
		return nil
	}

	if watchMode {
		fmt.Printf("[SDLC] Watch mode enabled. Watching for changes in detected projects...\n")
		return watchAndRunLoop(ctx, selectedProjects, projects, action, rootEnvConfig)
	}

	// Execute for each selected project once
	// Use a shared output lock to prevent garbled output from concurrent goroutines
	multi := len(selectedProjects) > 1
	outputLock := lib.NewOutputLock()

	var wg sync.WaitGroup
	var writersMu sync.Mutex
	var writers []*lib.BufferedPrefixWriter

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
			writer := runProject(ctx, p, index, action, env, args, multi, outputLock)
			if writer != nil {
				writersMu.Lock()
				writers = append(writers, writer)
				writersMu.Unlock()
			}
		}(project, originalIdx)
	}

	wg.Wait()

	// Flush any remaining buffered partial lines from all writers
	for _, w := range writers {
		w.Flush()
	}

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

	outputLock := lib.NewOutputLock()

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
			writer := runProject(runCtx, p, idx, action, env, args, len(projects) > 1, outputLock)
			// Flush any remaining buffered output after the project finishes
			if writer != nil {
				writer.Flush()
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

	if rootEnvConfig != nil {
		for k, v := range rootEnvConfig.Env {
			finalEnv[k] = v
		}
		finalArgs = append(finalArgs, rootEnvConfig.Args...)
	}

	modEnvConfig, err := config.LoadEnvConfig(p.AbsPath)
	if err == nil && modEnvConfig != nil {
		for k, v := range modEnvConfig.Env {
			finalEnv[k] = v
		}
		finalArgs = append(finalArgs, modEnvConfig.Args...)
	}

	if extraArgs != "" {
		finalArgs = append(finalArgs, strings.Split(extraArgs, " ")...)
	}
	return finalEnv, finalArgs
}

// runProject prepares and executes a single project. When multi is true, it
// creates a BufferedPrefixWriter using the shared outputLock to prevent
// interleaved output with other concurrent projects. It returns the stdout
// writer (or nil if multi is false) so the caller can Flush() any remaining
// partial lines after all projects finish.
func runProject(ctx context.Context, p engine.Project, index int, action string, env map[string]string, args []string, multi bool, outputLock *lib.OutputLock) *lib.BufferedPrefixWriter {
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
	var prefixWriter *lib.BufferedPrefixWriter

	if multi {
		prefixWriter = lib.NewBufferedPrefixWriter(os.Stdout, prefix, outputLock)
		out = prefixWriter
		errOut = lib.NewBufferedPrefixWriter(os.Stderr, prefix, outputLock)
	} else {
		out = os.Stdout
		errOut = os.Stderr
		fmt.Printf("[SDLC] Executing %s for module: %s%s%s\n", action, color, p.Path, colorReset)
	}

	cmdArgsStr := strings.Join(args, " ")

	cmdStr, err := p.Task.Command(action)
	if err != nil {
		fmt.Fprintf(errOut, "Error getting command: %v\n", err)
		return prefixWriter
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

	if err := runCommand(ctx, cmdStr, p.AbsPath, out, errOut, env); err != nil {
		fmt.Fprintf(errOut, "Command failed: %v\n", err)
	}
	return prefixWriter
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
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if info.Name() == "node_modules" || info.Name() == "dist" || info.Name() == "build" || info.Name() == "target" || info.Name() == "bin" || info.Name() == "pkg" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".log") || strings.HasSuffix(info.Name(), ".tmp") || strings.HasSuffix(info.Name(), ".lock") || strings.HasSuffix(info.Name(), ".pid") || strings.HasSuffix(info.Name(), ".swp") {
			return nil
		}

		if info.ModTime().After(sinceTime) {
			changed = true
			changedFile = path
			return io.EOF
		}
		return nil
	})

	if err == io.EOF {
		return true, changedFile, nil
	}
	return changed, "", err
}

func filterProjects(projects []engine.Project) ([]engine.Project, error) {
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

	if len(projects) == 1 {
		return projects, nil
	}

	return projects, nil
}

func promptModuleSelection(projects []engine.Project) ([]engine.Project, error) {
	selected := make(map[int]bool)
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
