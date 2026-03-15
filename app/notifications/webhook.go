package notifications

import (
    "bytes"
    "encoding/json"
    "net/http"
)

type WebhookNotifier struct {
    URL string
}

// Send posts the given event payload to the configured webhook URL.
func (w *WebhookNotifier) Send(event map[string]interface{}) error {
    payload, err := json.Marshal(event)
    if err != nil {
        return err
    }
    resp, err := http.Post(w.URL, "application/json", bytes.NewReader(payload))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}
