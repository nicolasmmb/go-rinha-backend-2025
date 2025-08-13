#
FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY cmd ./cmd/
COPY internal ./internal/
COPY libs ./libs/

RUN GOEXPERIMENT=arenas CGO_ENABLED=0 go build -ldflags="-w -s" -o server ./cmd/api/main.go

#
FROM alpine:latest AS api-stage
WORKDIR /root/

RUN apk --no-cache add curl

COPY --from=builder /app/server .

CMD ["./server"]
