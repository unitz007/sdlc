# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app

# Cache go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . ./

# Build the binary
# Disable CGO for static binary, target linux/amd64
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o sdlc ./

# Final stage
FROM alpine:latest
WORKDIR /app
# Add non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
# Copy binary from builder
COPY --from=builder /app/sdlc ./sdlc

# Expose default port if any (optional, example 8080)
EXPOSE 8080

# Default entrypoint runs the CLI. Users can override with docker run args.
ENTRYPOINT ["./sdlc"]
# Default command runs the dashboard (example) – adjust as needed.
CMD ["run"]
