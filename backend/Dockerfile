# Build stage
FROM golang:1.25.9-alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Runtime stage
FROM alpine:3.22

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/server .

# Expose port
EXPOSE 8080

# Run
CMD ["./server"]
