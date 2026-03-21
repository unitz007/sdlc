package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"sdlc/lib"
)

var (
	cfgFile    string
	workDir    string
	extraArgs  []string
	targetMod  string
	ignoreMods []string
	runAllMods bool
	watchMode        bool
	debounceDuration string
	dryRun           bool
	verbose          bool
	noColor          bool
	parallelFlag     parallelValue
)

// parallelValue is a custom pflag.Value that allows --parallel to be used
// without a value (treated as "true" = unbounded concurrency) or with a
// numeric value (e.g., --parallel=4 or -p 4).
type parallelValue struct {
	raw string
}

func (p *parallelValue) String() string { return p.raw }
func (p *parallelValue) Set(s string) error {
	p.raw = s
	return nil
}
func (p *parallelValue) Type() string { return "string" }

// NoOptDefValue implements the pflag NoOptDefVal interface so that bare --parallel/-p
// (without a value) is accepted and treated as unbounded concurrency.
func (p *parallelValue) NoOptDefValue() string { return "true" }

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:    "sdlc",
	Short:  "SDLC helps manage the full lifecycle of your software project",
	Long: `SDLC is a lightweight CLI tool that provides a unified interface 
for common software development lifecycle commands — run, test, and build — 
across different project types.`,
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		lib.InitColor(noColor)

		resolved, err := resolveConfigDir(cfgFile)
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			return err
		}
		cfgFile = resolved

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
// Exit code propagation: when a subcommand returns an *ExitCodeError (e.g. from a
// failed test with exit code 42), we extract that code and pass it to os.Exit so
// that CI/CD pipelines see the real failure code instead of the default 1.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		var exitErr *ExitCodeError
		if errors.As(err, &exitErr) {
			fmt.Println(err)
			os.Exit(exitErr.Code)
		}
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.SetVersionTemplate("sdlc version {{.Version}}\n")
	RootCmd.PersistentFlags().StringVarP(&workDir, "dir", "d", "", "Absolute path to project directory")
	RootCmd.PersistentFlags().StringSliceVarP(&extraArgs, "extra-args", "e", []string{}, "Extra arguments to pass to the build tool (repeatable, or space-separated within a single value)")
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Path to directory for configuration file")
	RootCmd.PersistentFlags().StringVarP(&targetMod, "module", "m", "", "Specific module/path to run (relative path)")
	RootCmd.PersistentFlags().StringSliceVarP(&ignoreMods, "ignore", "i", []string{}, "Ignore specific modules (by path or name)")
	RootCmd.PersistentFlags().BoolVarP(&runAllMods, "all", "a", false, "Run command for all detected modules")
	RootCmd.PersistentFlags().BoolVarP(&watchMode, "watch", "w", false, "Watch for file changes and restart")
	RootCmd.PersistentFlags().StringVar(&debounceDuration, "debounce", "500ms", "Debounce window for watch mode (e.g. 500ms, 1s)")
	RootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would happen without executing commands (dry run)")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show resolved commands and environment variables before execution")
	RootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	RootCmd.PersistentFlags().VarP(&parallelFlag, "parallel", "p", "Run modules concurrently (e.g., -p, -p 4)")
}

// resolveWorkDir handles the directory resolution logic including tilde expansion
func resolveWorkDir(dirFlag string) (string, error) {
	wd := dirFlag
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	if strings.HasPrefix(wd, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		wd = strings.ReplaceAll(wd, "~", homeDir)
	}

	if dirFlag != "" && !filepath.IsAbs(wd) {
		return "", fmt.Errorf("--dir requires an absolute path, got %q", dirFlag)
	}

	return wd, nil
}

// resolveConfigDir resolves and validates the --config directory path.
// It returns "" immediately if raw is empty (no custom config specified).
// Otherwise it expands ~/ to the home directory, resolves to an absolute path,
// and validates that the path exists and is a directory.
func resolveConfigDir(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}

	dir := raw

	if strings.HasPrefix(dir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = strings.ReplaceAll(dir, "~", homeDir)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("config directory does not exist: %s", abs)
		}
		return "", fmt.Errorf("failed to access config directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("config directory does not exist: %s", abs)
	}

	return abs, nil
}
