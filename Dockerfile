FROM golang:1.26.7-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git curl make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev-build

# make assets populates the tree that //go:embed static needs at compile time.
RUN make assets && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X 'github.com/tanq16/raikiri/cmd.AppVersion=${VERSION}'" -o raikiri .

FROM alpine:3.24.1

RUN apk add --no-cache ca-certificates tzdata ffmpeg

WORKDIR /app

COPY --from=builder /app/raikiri .

RUN mkdir -p /app/media /app/music /app/cache

EXPOSE 8080
ENTRYPOINT ["./raikiri"]
CMD ["serve", "--media", "/app/media", "--music", "/app/music", "--cache", "/app/cache"]
