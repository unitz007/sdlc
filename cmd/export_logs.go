package cmd

import (
    "fmt"
    "os"
    "strconv"
    "time"

    "github.com/spf13/cobra"
    "sdlc/config"
    "sdlc/log_exporter"
)

var exportLogsCmd = &cobra.Command{
    Use:   "export-logs",
    Short: "Export audit and execution logs to configured S3 bucket",
    RunE:  runExportLogs,
}

var (
    startDate string
    endDate   string
)

func init() {
    RootCmd.AddCommand(exportLogsCmd)
    exportLogsCmd.Flags().StringVar(&startDate, "start", "", "Start timestamp (RFC3339) (required)")
    exportLogsCmd.Flags().StringVar(&endDate, "end", "", "End timestamp (RFC3339) (required)")
    exportLogsCmd.MarkFlagRequired("start")
    exportLogsCmd.MarkFlagRequired("end")
}

func runExportLogs(cmd *cobra.Command, args []string) error {
    // Parse timestamps
    start, err := time.Parse(time.RFC3339, startDate)
    if err != nil {
        return fmt.Errorf("invalid start timestamp: %w", err)
    }
    end, err := time.Parse(time.RFC3339, endDate)
    if err != nil {
        return fmt.Errorf("invalid end timestamp: %w", err)
    }
    // Load config from environment variables via config package (we'll add getters)
    bucket := os.Getenv("LOG_EXPORT_S3_BUCKET")
    prefix := os.Getenv("LOG_EXPORT_S3_PREFIX")
    region := os.Getenv("AWS_REGION")
    accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
    secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
    if bucket == "" || region == "" || accessKey == "" || secretKey == "" {
        return fmt.Errorf("missing required S3 configuration environment variables")
    }
    exporter, err := log_exporter.NewS3LogExporter(bucket, prefix, region, accessKey, secretKey)
    if err != nil {
        return fmt.Errorf("failed to create S3 exporter: %w", err)
    }
    if err := exporter.ExportLogs(start, end); err != nil {
        return fmt.Errorf("failed to export logs: %w", err)
    }
    fmt.Printf("Exported logs from %s to %s\n", start.Format(time.RFC3339), end.Format(time.RFC3339))
    return nil
}
