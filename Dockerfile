FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -ldflags="-s -w" -o raikiri .

FROM alpine:latest
WORKDIR /app
RUN mkdir -p /app/media /app/music /app/cache
COPY --from=builder /app/raikiri .
EXPOSE 8080
CMD ["/app/raikiri", "-media", "/app/media", "-music", "/app/music", "-cache", "/app/cache"]
