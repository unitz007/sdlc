package config

import (
	"os"
	"path/filepath"
	"sdlc/lib"
	"strings"
	"testing"
)

func writeTestConf(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return path
}

func TestParseEnvConfig_ValidMix(t *testing.T) {
	dir := t.TempDir()
	content := `# This is a comment
PORT=8080
DATABASE_HOST=localhost

--verbose=true
NODE_ENV=production
--flag2=value2
`
	path := writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := ParseEnvConfig(path)
	if err != nil {
		t.Fatalf("ParseEnvConfig returned error: %v", err)
	}

	// Check env vars
	if settings.Env["PORT"] != "8080" {
		t.Errorf("expected PORT=8080, got %q", settings.Env["PORT"])
	}
	if settings.Env["DATABASE_HOST"] != "localhost" {
		t.Errorf("expected DATABASE_HOST=localhost, got %q", settings.Env["DATABASE_HOST"])
	}
	if settings.Env["NODE_ENV"] != "production" {
		t.Errorf("expected NODE_ENV=production, got %q", settings.Env["NODE_ENV"])
	}
	if len(settings.Env) != 3 {
		t.Errorf("expected 3 env vars, got %d", len(settings.Env))
	}

	// Check args
	if len(settings.Args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(settings.Args), settings.Args)
	}
	if settings.Args[0] != "--verbose=true" {
		t.Errorf("expected arg[0]=--verbose=true, got %q", settings.Args[0])
	}
	if settings.Args[1] != "--flag2=value2" {
		t.Errorf("expected arg[1]=--flag2=value2, got %q", settings.Args[1])
	}
}

func TestParseEnvConfig_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConf(t, dir, ".sdlc.conf", "")

	settings, err := ParseEnvConfig(path)
	if err != nil {
		t.Fatalf("ParseEnvConfig returned error: %v", err)
	}

	if len(settings.Env) != 0 {
		t.Errorf("expected empty Env, got %v", settings.Env)
	}
	if len(settings.Args) != 0 {
		t.Errorf("expected empty Args, got %v", settings.Args)
	}
}

func TestParseEnvConfig_OnlyCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	content := `# Comment line 1

# Comment line 2

`
	path := writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := ParseEnvConfig(path)
	if err != nil {
		t.Fatalf("ParseEnvConfig returned error: %v", err)
	}

	if len(settings.Env) != 0 {
		t.Errorf("expected empty Env, got %v", settings.Env)
	}
	if len(settings.Args) != 0 {
		t.Errorf("expected empty Args, got %v", settings.Args)
	}
}

func TestParseEnvConfig_EmptyValue(t *testing.T) {
	dir := t.TempDir()
	content := `KEY=
ANOTHER=value
`
	path := writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := ParseEnvConfig(path)
	if err != nil {
		t.Fatalf("ParseEnvConfig returned error: %v", err)
	}

	if val, ok := settings.Env["KEY"]; !ok {
		t.Error("expected KEY to be present in Env")
	} else if val != "" {
		t.Errorf("expected KEY to be empty string, got %q", val)
	}
	if settings.Env["ANOTHER"] != "value" {
		t.Errorf("expected ANOTHER=value, got %q", settings.Env["ANOTHER"])
	}
}

func TestParseEnvConfig_BareKeySkipped(t *testing.T) {
	dir := t.TempDir()
	content := `BAREKEY
VALID=value
`
	path := writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := ParseEnvConfig(path)
	if err != nil {
		t.Fatalf("ParseEnvConfig returned error: %v", err)
	}

	if _, ok := settings.Env["BAREKEY"]; ok {
		t.Error("expected BAREKEY to be skipped (not present in Env)")
	}
	if settings.Env["VALID"] != "value" {
		t.Errorf("expected VALID=value, got %q", settings.Env["VALID"])
	}
	if len(settings.Env) != 1 {
		t.Errorf("expected 1 env var, got %d", len(settings.Env))
	}
}

func TestMergeEnvSettings_OverrideBehavior(t *testing.T) {
	dir := t.TempDir()

	rootContent := `PORT=3000
HOST=localhost
--verbose=true
`
	rootPath := writeTestConf(t, dir, "root.conf", rootContent)
	rootSettings, err := ParseEnvConfig(rootPath)
	if err != nil {
		t.Fatalf("ParseEnvConfig root returned error: %v", err)
	}

	moduleContent := `PORT=8080
DEBUG=true
--flag2=value2
`
	modulePath := writeTestConf(t, dir, "module.conf", moduleContent)
	moduleSettings, err := ParseEnvConfig(modulePath)
	if err != nil {
		t.Fatalf("ParseEnvConfig module returned error: %v", err)
	}

	merged := MergeEnvSettings(rootSettings, moduleSettings)

	// Module's PORT should override root's PORT
	if merged.Env["PORT"] != "8080" {
		t.Errorf("expected PORT=8080 (module override), got %q", merged.Env["PORT"])
	}
	// Root's HOST should remain
	if merged.Env["HOST"] != "localhost" {
		t.Errorf("expected HOST=localhost (from root), got %q", merged.Env["HOST"])
	}
	// Module's DEBUG should be added
	if merged.Env["DEBUG"] != "true" {
		t.Errorf("expected DEBUG=true (from module), got %q", merged.Env["DEBUG"])
	}
	if len(merged.Env) != 3 {
		t.Errorf("expected 3 env vars, got %d", len(merged.Env))
	}

	// Args: root args first, then module args appended
	if len(merged.Args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(merged.Args), merged.Args)
	}
	if merged.Args[0] != "--verbose=true" {
		t.Errorf("expected arg[0]=--verbose=true (from root), got %q", merged.Args[0])
	}
	if merged.Args[1] != "--flag2=value2" {
		t.Errorf("expected arg[1]=--flag2=value2 (from module), got %q", merged.Args[1])
	}
}

func TestMergeEnvSettings_NilInputs(t *testing.T) {
	// Both nil
	merged := MergeEnvSettings(nil, nil)
	if len(merged.Env) != 0 {
		t.Errorf("expected empty Env, got %v", merged.Env)
	}
	if len(merged.Args) != 0 {
		t.Errorf("expected empty Args, got %v", merged.Args)
	}

	// Only base
	dir := t.TempDir()
	baseContent := `KEY=value
--flag=1
`
	basePath := writeTestConf(t, dir, "base.conf", baseContent)
	base, err := ParseEnvConfig(basePath)
	if err != nil {
		t.Fatalf("ParseEnvConfig returned error: %v", err)
	}

	merged = MergeEnvSettings(base, nil)
	if merged.Env["KEY"] != "value" {
		t.Errorf("expected KEY=value, got %q", merged.Env["KEY"])
	}
	if len(merged.Args) != 1 || merged.Args[0] != "--flag=1" {
		t.Errorf("expected [--flag=1], got %v", merged.Args)
	}

	// Only override
	merged = MergeEnvSettings(nil, base)
	if merged.Env["KEY"] != "value" {
		t.Errorf("expected KEY=value, got %q", merged.Env["KEY"])
	}
	if len(merged.Args) != 1 || merged.Args[0] != "--flag=1" {
		t.Errorf("expected [--flag=1], got %v", merged.Args)
	}
}

func TestLoadEnvConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	settings, err := LoadEnvConfig(dir)
	if err != nil {
		t.Fatalf("expected no error for missing .sdlc.conf, got: %v", err)
	}
	if settings != nil {
		t.Fatalf("expected nil settings for missing .sdlc.conf, got: %+v", settings)
	}
}

func TestLoadEnvConfig_PlainKeyValue(t *testing.T) {
	dir := t.TempDir()
	content := "PORT=8080\nDB_HOST=localhost\n"
	writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := LoadEnvConfig(dir)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}
	if settings.Env["PORT"] != "8080" {
		t.Errorf("expected PORT=8080, got %q", settings.Env["PORT"])
	}
	if settings.Env["DB_HOST"] != "localhost" {
		t.Errorf("expected DB_HOST=localhost, got %q", settings.Env["DB_HOST"])
	}
	if len(settings.Args) != 0 {
		t.Errorf("expected empty Args, got %v", settings.Args)
	}
}

func TestLoadEnvConfig_ValueWithEquals(t *testing.T) {
	dir := t.TempDir()
	content := "DB_CONN=postgres://user:pass@host/db?ssl=true\n"
	writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := LoadEnvConfig(dir)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}
	expected := "postgres://user:pass@host/db?ssl=true"
	if settings.Env["DB_CONN"] != expected {
		t.Errorf("expected DB_CONN=%q, got %q", expected, settings.Env["DB_CONN"])
	}
}

func TestLoadEnvConfig_CommentsIgnored(t *testing.T) {
	dir := t.TempDir()
	content := "# This is a comment\nPORT=9090\n# Another comment\n"
	writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := LoadEnvConfig(dir)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}
	if len(settings.Env) != 1 {
		t.Errorf("expected 1 env var, got %d: %v", len(settings.Env), settings.Env)
	}
	if settings.Env["PORT"] != "9090" {
		t.Errorf("expected PORT=9090, got %q", settings.Env["PORT"])
	}
}

func TestLoadEnvConfig_EmptyLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	content := "\nPORT=8080\n\nHOST=localhost\n\n"
	writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := LoadEnvConfig(dir)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}
	if len(settings.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d: %v", len(settings.Env), settings.Env)
	}
	if settings.Env["PORT"] != "8080" {
		t.Errorf("expected PORT=8080, got %q", settings.Env["PORT"])
	}
	if settings.Env["HOST"] != "localhost" {
		t.Errorf("expected HOST=localhost, got %q", settings.Env["HOST"])
	}
}

func TestLoadEnvConfig_Flags(t *testing.T) {
	dir := t.TempDir()
	content := "--debug\n--verbose\n"
	writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := LoadEnvConfig(dir)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}
	if len(settings.Env) != 0 {
		t.Errorf("expected empty Env, got %v", settings.Env)
	}
	if len(settings.Args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(settings.Args), settings.Args)
	}
	if settings.Args[0] != "--debug" {
		t.Errorf("expected arg[0]=--debug, got %q", settings.Args[0])
	}
	if settings.Args[1] != "--verbose" {
		t.Errorf("expected arg[1]=--verbose, got %q", settings.Args[1])
	}
}

func TestLoadEnvConfig_Mixed(t *testing.T) {
	dir := t.TempDir()
	content := "# Environment Variables\nPORT=8080\nDB_HOST=localhost\n\n# Extra Flags\n--debug\n"
	writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := LoadEnvConfig(dir)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}
	if len(settings.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d: %v", len(settings.Env), settings.Env)
	}
	if settings.Env["PORT"] != "8080" {
		t.Errorf("expected PORT=8080, got %q", settings.Env["PORT"])
	}
	if settings.Env["DB_HOST"] != "localhost" {
		t.Errorf("expected DB_HOST=localhost, got %q", settings.Env["DB_HOST"])
	}
	if len(settings.Args) != 1 || settings.Args[0] != "--debug" {
		t.Errorf("expected Args=[--debug], got %v", settings.Args)
	}
}

func TestLoadEnvConfig_DollarPrefixBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	content := "$LEGACY_VAR=value\n"
	writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := LoadEnvConfig(dir)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}
	if settings.Env["LEGACY_VAR"] != "value" {
		t.Errorf("expected LEGACY_VAR=value, got %q", settings.Env["LEGACY_VAR"])
	}
}

func TestLoadEnvConfig_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	content := "GREETING=\"hello world\"\nPATH_VAR='/usr/local/bin'\n"
	writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := LoadEnvConfig(dir)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}
	if settings.Env["GREETING"] != "hello world" {
		t.Errorf("expected GREETING=hello world, got %q", settings.Env["GREETING"])
	}
	if settings.Env["PATH_VAR"] != "/usr/local/bin" {
		t.Errorf("expected PATH_VAR=/usr/local/bin, got %q", settings.Env["PATH_VAR"])
	}
}

func TestParseEnvConfig_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	content := `GREETING="hello world"
PATH_VAR='/usr/local/bin'
UNQUOTED=simple
`
	path := writeTestConf(t, dir, ".sdlc.conf", content)

	settings, err := ParseEnvConfig(path)
	if err != nil {
		t.Fatalf("ParseEnvConfig returned error: %v", err)
	}

	if settings.Env["GREETING"] != "hello world" {
		t.Errorf("expected GREETING=hello world, got %q", settings.Env["GREETING"])
	}
	if settings.Env["PATH_VAR"] != "/usr/local/bin" {
		t.Errorf("expected PATH_VAR=/usr/local/bin, got %q", settings.Env["PATH_VAR"])
	}
	if settings.Env["UNQUOTED"] != "simple" {
		t.Errorf("expected UNQUOTED=simple, got %q", settings.Env["UNQUOTED"])
	}
}

func TestLoadHomeDir_FileExists(t *testing.T) {
	dir := t.TempDir()
	content := `{"go.mod": {"run": "go run .", "test": "go test ./..."}}`
	writeTestConf(t, dir, ".sdlc.json", content)

	tasks, err := LoadHomeDir(dir)
	if err != nil {
		t.Fatalf("LoadHomeDir returned error: %v", err)
	}
	if tasks == nil {
		t.Fatal("LoadHomeDir returned nil, expected non-nil map")
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(tasks))
	}

	task, ok := tasks["go.mod"]
	if !ok {
		t.Fatal("expected key 'go.mod' to exist")
	}
	if task.Run != "go run ." {
		t.Errorf("expected Run='go run .', got %q", task.Run)
	}
	if task.Test != "go test ./..." {
		t.Errorf("expected Test='go test ./...', got %q", task.Test)
	}
}

func TestLoadHomeDir_FileNotExists(t *testing.T) {
	dir := t.TempDir()

	tasks, err := LoadHomeDir(dir)
	if err != nil {
		t.Fatalf("LoadHomeDir returned error: %v", err)
	}
	if tasks != nil {
		t.Fatalf("expected nil tasks, got %+v", tasks)
	}
}

func TestLoadHomeDir_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeTestConf(t, dir, ".sdlc.json", "{invalid json content")

	tasks, err := LoadHomeDir(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if tasks != nil {
		t.Fatalf("expected nil tasks on error, got %+v", tasks)
	}
	// Assert the error message contains the file path
	expectedPath := filepath.Join(dir, ".sdlc.json")
	if !strings.Contains(err.Error(), expectedPath) {
		t.Errorf("expected error to contain path %q, got %q", expectedPath, err.Error())
	}
}

func TestLoadHomeDir_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeTestConf(t, dir, ".sdlc.json", "")

	tasks, err := LoadHomeDir(dir)
	if err != nil {
		t.Fatalf("LoadHomeDir returned error: %v", err)
	}
	if tasks == nil {
		t.Fatal("expected non-nil map for empty file, got nil")
	}
	if len(tasks) != 0 {
		t.Errorf("expected empty map, got %d entries", len(tasks))
	}
}

func TestMergeTasks_OverrideWins(t *testing.T) {
	base := map[string]lib.Task{
		"go.mod": {Run: "go run ."},
	}
	override := map[string]lib.Task{
		"go.mod": {Run: "go run ./cmd/server"},
	}

	result := MergeTasks(base, override)

	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result["go.mod"].Run != "go run ./cmd/server" {
		t.Errorf("expected Run='go run ./cmd/server' (override), got %q", result["go.mod"].Run)
	}
}

func TestMergeTasks_BasePreserved(t *testing.T) {
	base := map[string]lib.Task{
		"go.mod":        {Run: "go run ."},
		"package.json":  {Run: "npm start"},
	}
	override := map[string]lib.Task{
		"go.mod": {Run: "go run ./cmd/server"},
	}

	result := MergeTasks(base, override)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	// go.mod should come from override
	if result["go.mod"].Run != "go run ./cmd/server" {
		t.Errorf("expected go.mod Run='go run ./cmd/server' (override), got %q", result["go.mod"].Run)
	}
	// package.json should come from base
	if result["package.json"].Run != "npm start" {
		t.Errorf("expected package.json Run='npm start' (base), got %q", result["package.json"].Run)
	}
}

func TestMergeTasks_NilInputs(t *testing.T) {
	sample := map[string]lib.Task{
		"go.mod": {Run: "go run ."},
	}

	t.Run("both nil", func(t *testing.T) {
		result := MergeTasks(nil, nil)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("only base non-nil", func(t *testing.T) {
		result := MergeTasks(sample, nil)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result["go.mod"].Run != "go run ." {
			t.Errorf("expected Run='go run .', got %q", result["go.mod"].Run)
		}
	})

	t.Run("only override non-nil", func(t *testing.T) {
		result := MergeTasks(nil, sample)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result["go.mod"].Run != "go run ." {
			t.Errorf("expected Run='go run .', got %q", result["go.mod"].Run)
		}
	})
}

func TestLoadAndMerge_HomeOnly(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := t.TempDir()

	writeTestConf(t, homeDir, ".sdlc.json", `{"go.mod": {"run": "go run .", "test": "go test ./..."}}`)

	homeTasks, err := LoadHomeDir(homeDir)
	if err != nil {
		t.Fatalf("LoadHomeDir returned error: %v", err)
	}
	projectTasks, err := LoadLocal(projectDir)
	if err != nil {
		t.Fatalf("LoadLocal returned error: %v", err)
	}

	result := MergeTasks(homeTasks, projectTasks)

	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	task, ok := result["go.mod"]
	if !ok {
		t.Fatal("expected key 'go.mod' to exist")
	}
	if task.Run != "go run ." {
		t.Errorf("expected Run='go run .', got %q", task.Run)
	}
	if task.Test != "go test ./..." {
		t.Errorf("expected Test='go test ./...', got %q", task.Test)
	}
}

func TestLoadAndMerge_ProjectOnly(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := t.TempDir()

	writeTestConf(t, projectDir, ".sdlc.json", `{"package.json": {"run": "npm start", "test": "npm test"}}`)

	homeTasks, err := LoadHomeDir(homeDir)
	if err != nil {
		t.Fatalf("LoadHomeDir returned error: %v", err)
	}
	projectTasks, err := LoadLocal(projectDir)
	if err != nil {
		t.Fatalf("LoadLocal returned error: %v", err)
	}

	result := MergeTasks(homeTasks, projectTasks)

	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	task, ok := result["package.json"]
	if !ok {
		t.Fatal("expected key 'package.json' to exist")
	}
	if task.Run != "npm start" {
		t.Errorf("expected Run='npm start', got %q", task.Run)
	}
	if task.Test != "npm test" {
		t.Errorf("expected Test='npm test', got %q", task.Test)
	}
}

func TestLoadAndMerge_Merged(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := t.TempDir()

	// Home config: go.mod and package.json
	writeTestConf(t, homeDir, ".sdlc.json", `{"go.mod": {"run": "go run .", "test": "go test ./..."}, "package.json": {"run": "npm start"}}`)

	// Project config: overrides go.mod, adds pom.xml
	writeTestConf(t, projectDir, ".sdlc.json", `{"go.mod": {"run": "go run ./cmd/server", "test": "go test -v ./..."}, "pom.xml": {"build": "mvn package"}}`)

	homeTasks, err := LoadHomeDir(homeDir)
	if err != nil {
		t.Fatalf("LoadHomeDir returned error: %v", err)
	}
	projectTasks, err := LoadLocal(projectDir)
	if err != nil {
		t.Fatalf("LoadLocal returned error: %v", err)
	}

	result := MergeTasks(homeTasks, projectTasks)

	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	// go.mod should come from project (override)
	goTask, ok := result["go.mod"]
	if !ok {
		t.Fatal("expected key 'go.mod' to exist")
	}
	if goTask.Run != "go run ./cmd/server" {
		t.Errorf("expected go.mod Run='go run ./cmd/server' (override), got %q", goTask.Run)
	}
	if goTask.Test != "go test -v ./..." {
		t.Errorf("expected go.mod Test='go test -v ./...' (override), got %q", goTask.Test)
	}

	// package.json should come from home (base, not overridden)
	pkgTask, ok := result["package.json"]
	if !ok {
		t.Fatal("expected key 'package.json' to exist")
	}
	if pkgTask.Run != "npm start" {
		t.Errorf("expected package.json Run='npm start' (base), got %q", pkgTask.Run)
	}

	// pom.xml should come from project (unique to override)
	pomTask, ok := result["pom.xml"]
	if !ok {
		t.Fatal("expected key 'pom.xml' to exist")
	}
	if pomTask.Build != "mvn package" {
		t.Errorf("expected pom.xml Build='mvn package', got %q", pomTask.Build)
	}
}
