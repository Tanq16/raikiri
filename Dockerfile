FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -ldflags="-s -w" -o raikiri .

FROM alpine:latest
WORKDIR /app
RUN mkdir -p /app/media
COPY --from=builder /app/raikiri .
EXPOSE 8080
CMD ["/app/raikiri"]
