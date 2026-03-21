package config

import (
	"os"
	"path/filepath"
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
