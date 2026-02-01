# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o opencrawler ./cmd/crawler

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    chromium \
    chromium-chromedriver \
    nss \
    freetype \
    harfbuzz \
    ttf-freefont \
    && rm -rf /var/cache/apk/*

# Set Chrome environment variables
ENV CHROME_BIN=/usr/bin/chromium-browser \
    CHROME_PATH=/usr/lib/chromium/ \
    CHROMIUM_FLAGS="--no-sandbox --disable-dev-shm-usage"

# Create non-root user
RUN addgroup -S opencrawler && adduser -S opencrawler -G opencrawler

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/opencrawler /usr/local/bin/opencrawler

# Create output directory
RUN mkdir -p /output && chown -R opencrawler:opencrawler /output

USER opencrawler

ENTRYPOINT ["opencrawler"]
CMD ["--help"]
