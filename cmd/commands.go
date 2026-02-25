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
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
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

	if len(projects) > 1 {
		fmt.Printf("[SDLC] Multi-module project detected (%d modules):\n", len(projects))
		for i, p := range projects {
			color := getModuleColor(i)
			fmt.Printf(" • %s%s%s (%s)\n", color, p.Path, colorReset, p.Name)
		}
		fmt.Println()
	}

	// Load root .sdlc.conf if available
	rootEnvConfig, err := config.LoadEnvConfig(wd)
	if err != nil {
		fmt.Printf("Warning: failed to load root .sdlc.conf: %v\n", err)
	}

	// Filter projects based on flags
	selectedProjects := filterProjects(projects)

	if len(selectedProjects) == 0 {
		return fmt.Errorf("no projects matched the criteria")
	}

	if len(selectedProjects) > 1 && !runAllMods {
		fmt.Printf("[SDLC] Multiple projects detected. Running all modules by default.\n")
	}

	if watchMode {
		fmt.Printf("[SDLC] Watch mode enabled. Watching for changes in detected projects...\n")
		return watchAndRunLoop(ctx, selectedProjects, action, rootEnvConfig)
	}

	// Execute for each selected project once
	var wg sync.WaitGroup
	for i, project := range selectedProjects {
		wg.Add(1)
		go func(p engine.Project, index int) {
			defer wg.Done()
			env, args := prepareProjectEnv(p, rootEnvConfig)
			runProject(ctx, p, index, action, env, args, len(selectedProjects) > 1)
		}(project, i)
	}

	wg.Wait()
	return nil
}

func watchAndRunLoop(ctx context.Context, projects []engine.Project, action string, rootEnvConfig *config.EnvSettings) error {
	fmt.Println("[SDLC] Starting watchAndRunLoop")
	defer fmt.Println("[SDLC] Exiting watchAndRunLoop")
	lastModTime := time.Now()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Create a cancellable context for this run iteration
		runCtx, cancel := context.WithCancel(ctx)

		// Start all projects
		var wg sync.WaitGroup
		for i, project := range projects {
			wg.Add(1)
			go func(p engine.Project, index int) {
				defer wg.Done()
				env, args := prepareProjectEnv(p, rootEnvConfig)
				runProject(runCtx, p, index, action, env, args, len(projects) > 1)
			}(project, i)
		}

		// Wait for changes
		changed := false
		var changedPath string

		// Loop to check for changes
	changeLoop:
		for {
			select {
			case <-ctx.Done():
				fmt.Println("[SDLC] Context cancelled, exiting watch loop")
				cancel()
				wg.Wait()
				return nil
			case <-ticker.C:
				// Check all projects for changes
				for _, p := range projects {
					c, err := hasChanges(p.AbsPath, lastModTime)
					if err != nil {
						fmt.Printf("[SDLC] Watch error in %s: %v\n", p.Path, err)
						continue
					}
					if c {
						changed = true
						changedPath = p.Path
						break changeLoop
					}
				}
			}
		}

		if changed {
			fmt.Printf("\n[SDLC] File change detected in %s. Restarting all modules...\n", changedPath)
			lastModTime = time.Now()

			// Cancel current processes
			cancel()

			// Wait for them to cleanup
			wg.Wait()

			// Add a small delay to ensure file handles are released (fixes EPERM issues with tools like Vite)
			time.Sleep(1 * time.Second)

			// Loop continues -> restarts all
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
	prefix := fmt.Sprintf("[DEBUG-%s%s%s] ", color, p.Path, colorReset)
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

// hasChanges checks if any file in root has been modified since sinceTime
func hasChanges(root string, sinceTime time.Time) (bool, error) {
	var changed bool
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
			return io.EOF // Stop walking
		}
		return nil
	})

	if err == io.EOF {
		return true, nil
	}
	return changed, err
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

func filterProjects(projects []engine.Project) []engine.Project {
	if runAllMods {
		return projects
	}

	if targetMod != "" {
		for _, p := range projects {
			if p.Path == targetMod {
				return []engine.Project{p}
			}
		}
		return []engine.Project{}
	}

	// If only one project exists, default to it
	if len(projects) == 1 {
		return projects
	}

	// Otherwise return empty list (caller will handle ambiguous case)
	// Actually, returning all projects here and letting the caller decide
	// based on count is better for the error message "multiple projects found"
	return projects
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
