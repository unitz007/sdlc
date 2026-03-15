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
    _, err := rand.Read(b)
    if err != nil {
        // fallback to timestamp based ID
        return fmt.Sprintf("%d", time.Now().UnixNano())
    }
    return hex.EncodeToString(b)
}
