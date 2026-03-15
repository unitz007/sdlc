package cmd

import (
    "sync"
    "time"
)

type ProjectStatus struct {
    Path       string    `json:"path"`
    Status     string    `json:"status"`
    LastUpdate time.Time `json:"lastUpdate"`
}

var (
    statusMu   sync.RWMutex
    statuses   = make(map[string]*ProjectStatus)
    statusChan = make(chan struct{}, 1) // broadcast signal for updates
)

// SetStatus updates the status for a given project path and notifies listeners.
func SetStatus(path, status string) {
    statusMu.Lock()
    defer statusMu.Unlock()
    s, ok := statuses[path]
    if !ok {
        s = &ProjectStatus{Path: path}
        statuses[path] = s
    }
    s.Status = status
    s.LastUpdate = time.Now()
    // Notify listeners non‑blocking.
    select {
    case statusChan <- struct{}{}:
    default:
    }
}

// GetAll returns a slice copy of all project statuses.
func GetAll() []*ProjectStatus {
    statusMu.RLock()
    defer statusMu.RUnlock()
    out := make([]*ProjectStatus, 0, len(statuses))
    for _, v := range statuses {
        // copy to avoid race when caller modifies
        copied := *v
        out = append(out, &copied)
    }
    return out
}

// WaitForUpdate returns a channel that receives a struct when a status update occurs.
func WaitForUpdate() <-chan struct{} {
    return statusChan
}
