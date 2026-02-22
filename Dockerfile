# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git curl make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Download assets and build
RUN make assets && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X 'github.com/tanq16/raikiri/cmd.AppVersion=docker'" -o raikiri .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata ffmpeg
WORKDIR /app

RUN mkdir -p /app/media /app/music /app/cache
COPY --from=builder /app/raikiri .

EXPOSE 8080
ENTRYPOINT ["./raikiri"]
CMD ["serve", "--media", "/app/media", "--music", "/app/music", "--cache", "/app/cache"]
