package cmd

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

// sendSlackNotification posts a message to the configured Slack webhook.
// It returns an error if the HTTP request fails or Slack returns a non-2xx status.
func sendSlackNotification(message string) error {
    if slackWebhook == "" {
        // No webhook configured; nothing to do.
        return nil
    }
    payload := map[string]string{"text": message}
    data, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal slack payload: %w", err)
    }
    resp, err := http.Post(slackWebhook, "application/json", bytes.NewReader(data))
    if err != nil {
        return fmt.Errorf("failed to send slack webhook: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("slack webhook returned status %s", resp.Status)
    }
    return nil
}
