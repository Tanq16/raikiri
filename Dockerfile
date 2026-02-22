FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -ldflags="-s -w -X github.com/tanq16/raikiri/cmd.AppVersion=docker" -o raikiri .

FROM alpine:latest
WORKDIR /app
RUN mkdir -p /app/media /app/music /app/cache
RUN apk add ffmpeg
COPY --from=builder /app/raikiri .
EXPOSE 8080
CMD ["/app/raikiri", "serve", "--media", "/app/media", "--music", "/app/music", "--cache", "/app/cache"]
