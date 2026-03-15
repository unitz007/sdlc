package main

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/robfig/cron/v3"
)

type Workflow struct {
    ID        string    `json:"id"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}

type Schedule struct {
    ID         string    `json:"id"`
    WorkflowID string    `json:"workflow_id"`
    CronExpr   string    `json:"cron_expr,omitempty"`
    Interval   int       `json:"interval_seconds,omitempty"`
    Active     bool      `json:"active"`
    NextRun    time.Time `json:"next_run,omitempty"`
}

type workflowStore struct {
    sync.Mutex
    data map[string]*Workflow
}

type scheduleStore struct {
    sync.Mutex
    data     map[string]*Schedule
    entryIDs map[string]cron.EntryID // map schedule ID to cron entry ID
}

var (
    store       = &workflowStore{data: make(map[string]*Workflow)}
    schedStore  = &scheduleStore{data: make(map[string]*Schedule), entryIDs: make(map[string]cron.EntryID)}
    cronScheduler = cron.New(cron.WithSeconds())
)

func main() {
    // Start scheduler goroutine
    go startScheduler()

    mux := http.NewServeMux()
    mux.HandleFunc("/workflows", workflowsHandler)
    mux.HandleFunc("/workflows/", workflowActionHandler)
    mux.HandleFunc("/schedules", schedulesHandler)
    mux.HandleFunc("/schedules/", scheduleActionHandler)

    srv := &http.Server{Addr: ":8080", Handler: mux}
    log.Println("Workflow API server listening on :8080")
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("listen: %s", err)
    }
}

// Scheduler control
func startScheduler() {
    cronScheduler.Start()
}

// Simple trigger that creates a new workflow instance with status "triggered"
func triggerWorkflow(workflowID string) {
    wf := &Workflow{ID: generateID(), Status: "triggered", CreatedAt: time.Now()}
    store.Lock()
    store.data[wf.ID] = wf
    store.Unlock()
    log.Printf("[Scheduler] Triggered workflow %s (new instance %s)\n", workflowID, wf.ID)
}

// ---------- Workflow Handlers ----------
func workflowsHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        createWorkflow(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

func workflowActionHandler(w http.ResponseWriter, r *http.Request) {
    // Expected: /workflows/{id} or /workflows/{id}/action
    path := r.URL.Path[len("/workflows/"):]
    parts := splitPath(path)
    if len(parts) == 0 {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    id := parts[0]
    if len(parts) == 1 && r.Method == http.MethodGet {
        getWorkflow(w, r, id)
        return
    }
    if len(parts) == 2 && r.Method == http.MethodPost {
        action := parts[1]
        switch action {
        case "start":
            modifyStatus(w, r, id, "running")
        case "pause":
            modifyStatus(w, r, id, "paused")
        case "resume":
            modifyStatus(w, r, id, "running")
        default:
            http.Error(w, "unknown action", http.StatusBadRequest)
        }
        return
    }
    http.Error(w, "bad request", http.StatusBadRequest)
}

func splitPath(p string) []string {
    var res []string
    for _, seg := range strings.Split(p, "/") {
        if seg != "" {
            res = append(res, seg)
        }
    }
    return res
}

func createWorkflow(w http.ResponseWriter, r *http.Request) {
    id := generateID()
    wf := &Workflow{ID: id, Status: "created", CreatedAt: time.Now()}
    store.Lock()
    store.data[id] = wf
    store.Unlock()
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(wf)
}

func getWorkflow(w http.ResponseWriter, r *http.Request, id string) {
    store.Lock()
    wf, ok := store.data[id]
    store.Unlock()
    if !ok {
        http.Error(w, "workflow not found", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(wf)
}

func modifyStatus(w http.ResponseWriter, r *http.Request, id, newStatus string) {
    store.Lock()
    wf, ok := store.data[id]
    if ok {
        wf.Status = newStatus
    }
    store.Unlock()
    if !ok {
        http.Error(w, "workflow not found", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(wf)
}

func generateID() string {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        return fmt.Sprintf("%d", time.Now().UnixNano())
    }
    return hex.EncodeToString(b)
}

// ---------- Schedule Handlers ----------
func schedulesHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        createSchedule(w, r)
    case http.MethodGet:
        listSchedules(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

func scheduleActionHandler(w http.ResponseWriter, r *http.Request) {
    // Expected: /schedules/{id} (DELETE) or /schedules/{id}/{action} (POST)
    path := r.URL.Path[len("/schedules/"):]
    parts := splitPath(path)
    if len(parts) == 0 {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    id := parts[0]
    if len(parts) == 1 && r.Method == http.MethodDelete {
        deleteSchedule(w, r, id)
        return
    }
    if len(parts) == 2 && r.Method == http.MethodPost {
        action := parts[1]
        switch action {
        case "pause":
            pauseSchedule(w, r, id)
        case "resume":
            resumeSchedule(w, r, id)
        default:
            http.Error(w, "unknown action", http.StatusBadRequest)
        }
        return
    }
    http.Error(w, "bad request", http.StatusBadRequest)
}

func createSchedule(w http.ResponseWriter, r *http.Request) {
    var req struct {
        WorkflowID string `json:"workflow_id"`
        CronExpr   string `json:"cron_expr,omitempty"`
        Interval   int    `json:"interval_seconds,omitempty"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid json", http.StatusBadRequest)
        return
    }
    if req.WorkflowID == "" || (req.CronExpr == "" && req.Interval <= 0) {
        http.Error(w, "workflow_id and either cron_expr or interval_seconds required", http.StatusBadRequest)
        return
    }
    expr := req.CronExpr
    if expr == "" {
        expr = fmt.Sprintf("@every %ds", req.Interval)
    }
    // Validate expression
    if _, err := cron.ParseStandard(expr); err != nil {
        http.Error(w, "invalid cron expression", http.StatusBadRequest)
        return
    }
    // Create schedule
    id := generateID()
    sched := &Schedule{ID: id, WorkflowID: req.WorkflowID, CronExpr: expr, Interval: req.Interval, Active: true}
    // Add to cron
    entryID, err := cronScheduler.AddFunc(expr, func() { triggerWorkflow(req.WorkflowID) })
    if err != nil {
        http.Error(w, "failed to schedule", http.StatusInternalServerError)
        return
    }
    schedStore.Lock()
    schedStore.data[id] = sched
    schedStore.entryIDs[id] = entryID
    schedStore.Unlock()
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(sched)
}

func listSchedules(w http.ResponseWriter, r *http.Request) {
    schedStore.Lock()
    list := make([]*Schedule, 0, len(schedStore.data))
    for _, s := range schedStore.data {
        if s.Active {
            if entryID, ok := schedStore.entryIDs[s.ID]; ok {
                e := cronScheduler.Entry(entryID)
                s.NextRun = e.Next
            }
        }
        list = append(list, s)
    }
    schedStore.Unlock()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(list)
}

func deleteSchedule(w http.ResponseWriter, r *http.Request, id string) {
    schedStore.Lock()
    defer schedStore.Unlock()
    if entryID, ok := schedStore.entryIDs[id]; ok {
        cronScheduler.Remove(entryID)
        delete(schedStore.entryIDs, id)
    }
    if _, ok := schedStore.data[id]; !ok {
        http.Error(w, "schedule not found", http.StatusNotFound)
        return
    }
    delete(schedStore.data, id)
    w.WriteHeader(http.StatusNoContent)
}

func pauseSchedule(w http.ResponseWriter, r *http.Request, id string) {
    schedStore.Lock()
    defer schedStore.Unlock()
    s, ok := schedStore.data[id]
    if !ok {
        http.Error(w, "schedule not found", http.StatusNotFound)
        return
    }
    if entryID, ok := schedStore.entryIDs[id]; ok {
        cronScheduler.Remove(entryID)
        delete(schedStore.entryIDs, id)
    }
    s.Active = false
    w.WriteHeader(http.StatusNoContent)
}

func resumeSchedule(w http.ResponseWriter, r *http.Request, id string) {
    schedStore.Lock()
    defer schedStore.Unlock()
    s, ok := schedStore.data[id]
    if !ok {
        http.Error(w, "schedule not found", http.StatusNotFound)
        return
    }
    if s.Active {
        w.WriteHeader(http.StatusNoContent)
        return
    }
    entryID, err := cronScheduler.AddFunc(s.CronExpr, func() { triggerWorkflow(s.WorkflowID) })
    if err != nil {
        http.Error(w, "failed to resume schedule", http.StatusInternalServerError)
        return
    }
    schedStore.entryIDs[id] = entryID
    s.Active = true
    w.WriteHeader(http.StatusNoContent)
}
