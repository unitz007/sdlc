package notifications

import (
    "bytes"
    "encoding/json"
    "net/http"
    "os"
)

type SlackNotifier struct {
    WebhookURL string
}

func NewSlackNotifier() *SlackNotifier {
    return &SlackNotifier{WebhookURL: os.Getenv("NOTIFY_SLACK_URL")}
}

func (s *SlackNotifier) Send(event map[string]interface{}) error {
    if s.WebhookURL == "" {
        return nil
    }
    payload, err := json.Marshal(event)
    if err != nil {
        return err
    }
    resp, err := http.Post(s.WebhookURL, "application/json", bytes.NewReader(payload))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}
