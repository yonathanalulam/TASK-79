# ---- Build stage ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

# ---- Runtime stage ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=builder /bin/server /app/server

# Copy static assets (CSS, JS) that are served from disk
COPY web/static ./web/static

# Create writable directories for uploads and exports
RUN mkdir -p /app/web/uploads /app/web/exports

EXPOSE 8080

ENV APP_PORT=8080 \
    UPLOADS_DIR=/app/web/uploads \
    EXPORTS_DIR=/app/web/exports

ENTRYPOINT ["/app/server"]
