package cmd

import (
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/spf13/cobra"
)

func init() {
    RootCmd.AddCommand(dashboardCmd)
}

var dashboardCmd = &cobra.Command{
    Use:   "dashboard",
    Short: "Start a web dashboard for monitoring active workflows",
    RunE: func(cmd *cobra.Command, args []string) error {
        port := cmd.Flag("port").Value.String()
        if port == "" {
            port = "8080"
        }
        http.HandleFunc("/status", statusHandler)
        http.HandleFunc("/events", eventsHandler)
        fmt.Printf("Dashboard listening on http://localhost:%s\n", port)
        return http.ListenAndServe(":"+port, nil)
    },
}

func init() {
    dashboardCmd.Flags().StringP("port", "p", "8080", "Port for the dashboard server")
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(GetAll())
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    // Send initial data
    data, _ := json.Marshal(GetAll())
    fmt.Fprintf(w, "data: %s\n\n", data)
    flusher.Flush()
    // Listen for updates
    notify := WaitForUpdate()
    for {
        select {
        case <-r.Context().Done():
            return
        case <-notify:
            data, _ := json.Marshal(GetAll())
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
        }
    }
}
