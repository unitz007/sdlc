package workflow

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sync"
)

// Callback is a function that is executed during stage transitions.
// It returns an error to indicate failure which aborts the transition.
type Callback func() error

// Stage represents a point in the workflow. It can have callbacks that run
// when entering or exiting the stage.
type Stage struct {
    Name    string
    onEnter []Callback
    onExit  []Callback
}

// OnEnter registers a callback to be executed when the workflow enters the stage.
func (s *Stage) OnEnter(cb Callback) {
    s.onEnter = append(s.onEnter, cb)
}

// OnExit registers a callback to be executed when the workflow exits the stage.
func (s *Stage) OnExit(cb Callback) {
    s.onExit = append(s.onExit, cb)
}

// Transition defines a permitted move from one stage to another.
// Condition, if provided, must return true for the transition to be allowed.
// Action is an optional callback executed after the exit callbacks and before the enter callbacks.
type Transition struct {
    From      string
    To        string
    Condition func() bool // optional
    Action    Callback      // optional
}

// persistedState is the JSON representation stored on disk.
type persistedState struct {
    Current string `json:"current"`
}

// Workflow models a set of stages and allowed transitions between them.
// It keeps track of the current stage and can optionally persist its state.
type Workflow struct {
    stages      map[string]*Stage
    transitions []Transition
    current     string
    persistPath string
    mu          sync.Mutex
}

// New creates a new Workflow. If persistPath is non-empty, the workflow will load
// any saved state from that file and later persist state on each successful transition.
func New(persistPath string) (*Workflow, error) {
    wf := &Workflow{
        stages:      make(map[string]*Stage),
        transitions: []Transition{},
        persistPath: persistPath,
    }
    if persistPath != "" {
        // Ensure directory exists
        if err := os.MkdirAll(filepath.Dir(persistPath), 0o755); err != nil {
            return nil, fmt.Errorf("cannot create persistence directory: %w", err)
        }
        // Load existing state if file exists
        if data, err := os.ReadFile(persistPath); err == nil {
            var ps persistedState
            if err := json.Unmarshal(data, &ps); err == nil {
                wf.current = ps.Current
            }
        }
    }
    return wf, nil
}

// AddStage creates a new stage with the given name. It returns the stage for further configuration.
func (wf *Workflow) AddStage(name string) (*Stage, error) {
    wf.mu.Lock()
    defer wf.mu.Unlock()
    if _, exists := wf.stages[name]; exists {
        return nil, fmt.Errorf("stage %s already exists", name)
    }
    s := &Stage{Name: name}
    wf.stages[name] = s
    // If there is no current stage yet, set this as the initial stage.
    if wf.current == "" {
        wf.current = name
        if err := wf.persist(); err != nil {
            return nil, err
        }
    }
    return s, nil
}

// AddTransition registers a permitted transition from one stage to another.
func (wf *Workflow) AddTransition(from, to string, condition func() bool, action Callback) error {
    wf.mu.Lock()
    defer wf.mu.Unlock()
    if _, ok := wf.stages[from]; !ok {
        return fmt.Errorf("unknown from stage %s", from)
    }
    if _, ok := wf.stages[to]; !ok {
        return fmt.Errorf("unknown to stage %s", to)
    }
    wf.transitions = append(wf.transitions, Transition{From: from, To: to, Condition: condition, Action: action})
    return nil
}

// Current returns the name of the current stage.
func (wf *Workflow) Current() string {
    wf.mu.Lock()
    defer wf.mu.Unlock()
    return wf.current
}

// Move attempts to transition the workflow to the specified target stage.
// It validates that a transition exists from the current stage, checks any condition,
// runs exit callbacks, the transition action, and then enter callbacks.
func (wf *Workflow) Move(to string) error {
    wf.mu.Lock()
    defer wf.mu.Unlock()
    if wf.current == "" {
        return errors.New("workflow has no current stage")
    }
    // Find a matching transition.
    var tr *Transition
    for i := range wf.transitions {
        t := &wf.transitions[i]
        if t.From == wf.current && t.To == to {
            tr = t
            break
        }
    }
    if tr == nil {
        return fmt.Errorf("no transition from %s to %s", wf.current, to)
    }
    // Check condition if present.
    if tr.Condition != nil && !tr.Condition() {
        return fmt.Errorf("transition condition from %s to %s not satisfied", wf.current, to)
    }
    // Execute exit callbacks of current stage.
    if curStage, ok := wf.stages[wf.current]; ok {
        for _, cb := range curStage.onExit {
            if err := cb(); err != nil {
                return fmt.Errorf("exit callback error on stage %s: %w", wf.current, err)
            }
        }
    }
    // Execute transition action.
    if tr.Action != nil {
        if err := tr.Action(); err != nil {
            return fmt.Errorf("transition action error from %s to %s: %w", wf.current, to, err)
        }
    }
    // Update current stage.
    wf.current = to
    // Execute enter callbacks of new stage.
    if newStage, ok := wf.stages[wf.current]; ok {
        for _, cb := range newStage.onEnter {
            if err := cb(); err != nil {
                return fmt.Errorf("enter callback error on stage %s: %w", wf.current, err)
            }
        }
    }
    // Persist state if configured.
    if wf.persistPath != "" {
        if err := wf.persist(); err != nil {
            return err
        }
    }
    return nil
}

// persist writes the current stage to the JSON file.
func (wf *Workflow) persist() error {
    if wf.persistPath == "" {
        return nil
    }
    ps := persistedState{Current: wf.current}
    data, err := json.MarshalIndent(ps, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(wf.persistPath, data, 0o644)
}
