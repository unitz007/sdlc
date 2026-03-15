package log_exporter

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3LogExporter struct {
    bucket   string
    prefix   string
    client   *s3.Client
    uploader *manager.Uploader
}

// NewS3LogExporter creates a new exporter with provided AWS credentials and region.
func NewS3LogExporter(bucket, prefix, region, accessKey, secretKey string) (*S3LogExporter, error) {
    cfg, err := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion(region),
        config.WithCredentialsProvider(aws.NewCredentialsCache(
            aws.StaticCredentialsProvider{
                Value: aws.Credentials{
                    AccessKeyID:     accessKey,
                    SecretAccessKey: secretKey,
                    Source:          "manual",
                },
            },
        )),
    )
    if err != nil {
        return nil, fmt.Errorf("unable to load AWS config: %w", err)
    }
    client := s3.NewFromConfig(cfg)
    uploader := manager.NewUploader(client)
    return &S3LogExporter{bucket: bucket, prefix: prefix, client: client, uploader: uploader}, nil
}

// ExportLogs uploads a JSON‑lines payload containing the start and end timestamps.
func (e *S3LogExporter) ExportLogs(start, end time.Time) error {
    // Simple payload – in a real system this would be the actual log entries.
    payload := fmt.Sprintf("{\"start\":\"%s\",\"end\":\"%s\"}\n", start.Format(time.RFC3339), end.Format(time.RFC3339))
    key := fmt.Sprintf("%s/logs-%d.jsonl", strings.TrimSuffix(e.prefix, "/"), time.Now().Unix())
    input := &s3.PutObjectInput{
        Bucket: aws.String(e.bucket),
        Key:    aws.String(key),
        Body:   strings.NewReader(payload),
    }
    _, err := e.uploader.Upload(context.TODO(), input)
    if err != nil {
        return fmt.Errorf("failed to upload logs to S3: %w", err)
    }
    return nil
}
