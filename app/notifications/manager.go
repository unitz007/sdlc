package notifications

import (
    "bytes"
    "encoding/json"
    "net/http"
    "os"
    "strconv"
)

type SlackNotifier struct {
    WebhookURL string
}

func NewSlackNotifier() *SlackNotifier {
    return &SlackNotifier{WebhookURL: os.Getenv("NOTIFY_SLACK_URL")}
}

func (s *SlackNotifier) Send(event map[string]interface{}) error {
    if s.WebhookURL == "" {
        return nil // disabled
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

type WebhookNotifier struct {
    URL string
}

func NewWebhookNotifier() *WebhookNotifier {
    return &WebhookNotifier{URL: os.Getenv("NOTIFY_WEBHOOK_URL")}
}

func (w *WebhookNotifier) Send(event map[string]interface{}) error {
    if w.URL == "" {
        return nil // disabled
    }
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

type NotificationManager struct {
    Slack   *SlackNotifier
    Webhook *WebhookNotifier
    EnableSlack   bool
    EnableWebhook bool
}

func NewNotificationManager() *NotificationManager {
    enableSlack, _ := strconv.ParseBool(os.Getenv("ENABLE_SLACK"))
    enableWebhook, _ := strconv.ParseBool(os.Getenv("ENABLE_WEBHOOK"))
    return &NotificationManager{
        Slack:   NewSlackNotifier(),
        Webhook: NewWebhookNotifier(),
        EnableSlack:   enableSlack,
        EnableWebhook: enableWebhook,
    }
}

func (nm *NotificationManager) Notify(eventType string, payload map[string]interface{}) error {
    // Build event with type
    event := map[string]interface{}{"event_type": eventType, "payload": payload}
    if nm.EnableSlack {
        if err := nm.Slack.Send(event); err != nil {
            return err
        }
    }
    if nm.EnableWebhook {
        if err := nm.Webhook.Send(event); err != nil {
            return err
        }
    }
    return nil
}
