package plugin

import (
	"testing"
)

func TestHookValidate(t *testing.T) {
	tests := []struct {
		name    string
		hook    Hook
		wantErr bool
	}{
		{
			name:    "valid pre hook",
			hook:    Hook{Name: "pre-build", Command: "npm run build"},
			wantErr: false,
		},
		{
			name:    "valid post hook",
			hook:    Hook{Name: "post-test", Command: "go test ./..."},
			wantErr: false,
		},
		{
			name:    "missing name",
			hook:    Hook{Name: "", Command: "echo hello"},
			wantErr: true,
		},
		{
			name:    "missing command",
			hook:    Hook{Name: "pre-build", Command: ""},
			wantErr: true,
		},
		{
			name:    "invalid name prefix",
			hook:    Hook{Name: "build", Command: "make build"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.hook.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Hook.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHookPhase(t *testing.T) {
	tests := []struct {
		hook Hook
		want string
	}{
		{Hook{Name: "pre-build"}, "pre"},
		{Hook{Name: "post-test"}, "post"},
		{Hook{Name: "custom", HookPhase: "pre"}, "pre"},
		{Hook{Name: "custom"}, ""},
	}

	for _, tt := range tests {
		got := tt.hook.Phase()
		if got != tt.want {
			t.Errorf("Hook{%q}.Phase() = %q, want %q", tt.hook.Name, got, tt.want)
		}
	}
}

func TestHookMatchesProject(t *testing.T) {
	hook := Hook{Name: "pre-build", Command: "echo", ProjectType: "node"}

	tests := []struct {
		projectType string
		want        bool
	}{
		{"node", true},
		{"NODE", true},
		{"go", false},
		{"", false},
	}

	for _, tt := range tests {
		got := hook.MatchesProject(tt.projectType)
		if got != tt.want {
			t.Errorf("MatchesProject(%q) = %v, want %v", tt.projectType, got, tt.want)
		}
	}

	// Empty project type matches everything
	openHook := Hook{Name: "pre-build", Command: "echo"}
	if !openHook.MatchesProject("node") {
		t.Error("expected open hook (no project type) to match any project")
	}
	if !openHook.MatchesProject("") {
		t.Error("expected open hook to match empty project type")
	}
}

func TestSortHooks(t *testing.T) {
	hooks := []Hook{
		{Name: "pre-test", Command: "slow", Priority: 10},
		{Name: "pre-build", Command: "lint", Priority: 1},
		{Name: "pre-test", Command: "fast", Priority: 0},
		{Name: "pre-build", Command: "format", Priority: 1},
	}

	sorted := SortHooks(hooks)

	// Check priority ordering
	if sorted[0].Priority != 0 {
		t.Errorf("expected first hook priority 0, got %d", sorted[0].Priority)
	}
	if sorted[1].Priority != 1 || sorted[1].Name != "pre-build" {
		t.Errorf("expected second hook pre-build/1, got %s/%d", sorted[1].Name, sorted[1].Priority)
	}
	if sorted[2].Priority != 1 || sorted[2].Name != "pre-build" {
		t.Errorf("expected third hook pre-build/1, got %s/%d", sorted[2].Name, sorted[2].Priority)
	}
	if sorted[3].Priority != 10 {
		t.Errorf("expected fourth hook priority 10, got %d", sorted[3].Priority)
	}
}
