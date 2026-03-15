# app/config.py
import os

# Notification configuration
NOTIFY_SLACK_URL = os.getenv('NOTIFY_SLACK_URL', '')
NOTIFY_WEBHOOK_URL = os.getenv('NOTIFY_WEBHOOK_URL', '')
ENABLE_SLACK = os.getenv('ENABLE_SLACK', 'false').lower() == 'true'
ENABLE_WEBHOOK = os.getenv('ENABLE_WEBHOOK', 'false').lower() == 'true'
