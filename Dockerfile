# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with version info
ARG VERSION=docker
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
        -X 'github.com/ogefest/findex/version.Version=${VERSION}' \
        -X 'github.com/ogefest/findex/version.Commit=${COMMIT}' \
        -X 'github.com/ogefest/findex/version.BuildDate=${BUILD_DATE}'" \
    -o findex ./cmd/findex

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
        -X 'github.com/ogefest/findex/version.Version=${VERSION}' \
        -X 'github.com/ogefest/findex/version.Commit=${COMMIT}' \
        -X 'github.com/ogefest/findex/version.BuildDate=${BUILD_DATE}'" \
    -o webserver ./cmd/webserver

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binaries
COPY --from=builder /build/findex /app/
COPY --from=builder /build/webserver /app/

# Copy web assets and templates
COPY --from=builder /build/web/assets /app/web/assets
COPY --from=builder /build/web/templates /app/web/templates

# Copy migration file
COPY --from=builder /build/init.sql /app/

# Create data directory
RUN mkdir -p /app/data

# Default config location
ENV CONFIG_PATH=/app/config.yaml

EXPOSE 8080

# Default command runs the webserver
CMD ["/app/webserver", "-config", "/app/config.yaml"]
