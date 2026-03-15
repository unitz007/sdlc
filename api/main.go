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
)

type Workflow struct {
    ID        string    `json:"id"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
    // Definition stored as raw YAML/JSON string
    Definition string `json:"definition_yaml,omitempty"`
    // Versions slice holds immutable version history
    Versions []*WorkflowVersion `json:"-"`
}

// WorkflowVersion captures an immutable snapshot of a workflow definition.
type WorkflowVersion struct {
    VersionNumber int       `json:"version_number"`
    Definition    string    `json:"definition_yaml"`
    CreatedAt     time.Time `json:"created_at"`
}



type workflowStore struct {
    sync.Mutex
    data map[string]*Workflow
}

var store = &workflowStore{data: make(map[string]*Workflow)}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/workflows", workflowsHandler)
    mux.HandleFunc("/workflows/", workflowActionHandler)
    // No separate version handler registration; handled within workflowActionHandler

    srv := &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }
    log.Println("Workflow API server listening on :8080")
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("listen: %s", err)
    }
}

// /workflows endpoint: POST to create, GET to list (optional)
func workflowsHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        createWorkflow(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

// /workflows/{id} and action subpaths
func workflowActionHandler(w http.ResponseWriter, r *http.Request) {
    // Expected path: /workflows/{id} or /workflows/{id}/action
    path := r.URL.Path[len("/workflows/":)]
    // split by '/'
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
    // Check for version subpaths
    if len(parts) >= 2 && parts[1] == "versions" {
        // /workflows/{id}/versions
        if len(parts) == 2 && r.Method == http.MethodGet {
            // List versions
            store.Lock()
            wf, ok := store.data[id]
            store.Unlock()
            if !ok {
                http.Error(w, "workflow not found", http.StatusNotFound)
                return
            }
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(wf.Versions)
            return
        }
        // /workflows/{id}/versions/{ver}
        if len(parts) >= 3 {
            verStr := parts[2]
            var verNum int
            _, err := fmt.Sscanf(verStr, "%d", &verNum)
            if err != nil {
                http.Error(w, "invalid version number", http.StatusBadRequest)
                return
            }
            store.Lock()
            wf, ok := store.data[id]
            store.Unlock()
            if !ok {
                http.Error(w, "workflow not found", http.StatusNotFound)
                return
            }
            var target *WorkflowVersion
            for _, v := range wf.Versions {
                if v.VersionNumber == verNum {
                    target = v
                    break
                }
            }
            if target == nil {
                http.Error(w, "version not found", http.StatusNotFound)
                return
            }
            if len(parts) == 3 && r.Method == http.MethodGet {
                // Detail
                w.Header().Set("Content-Type", "application/json")
                json.NewEncoder(w).Encode(target)
                return
            }
            if len(parts) == 4 && parts[3] == "rollback" && r.Method == http.MethodPost {
                // Rollback: create new version copying target definition
                now := time.Now()
                newVerNum := len(wf.Versions) + 1
                newVer := &WorkflowVersion{VersionNumber: newVerNum, Definition: target.Definition, CreatedAt: now}
                wf.Definition = target.Definition
                wf.Versions = append(wf.Versions, newVer)
                w.Header().Set("Content-Type", "application/json")
                json.NewEncoder(w).Encode(newVer)
                return
            }
        }
        http.Error(w, "bad request", http.StatusBadRequest)
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
    // Parse request body for definition
    var payload struct {
        Definition string `json:"definition_yaml"`
    }
    if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }
    if payload.Definition == "" {
        http.Error(w, "definition_yaml is required", http.StatusBadRequest)
        return
    }
    id := generateID()
    now := time.Now()
    // initial version
    version := &WorkflowVersion{VersionNumber: 1, Definition: payload.Definition, CreatedAt: now}
    wf := &Workflow{ID: id, Status: "created", CreatedAt: now, Definition: payload.Definition, Versions: []*WorkflowVersion{version}}
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
    _, err := rand.Read(b)
    if err != nil {
        // fallback to timestamp based ID
        return fmt.Sprintf("%d", time.Now().UnixNano())
    }
    return hex.EncodeToString(b)
}
