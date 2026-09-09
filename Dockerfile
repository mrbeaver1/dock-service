# syntax=docker/dockerfile:1

FROM golang:1.27.1-alpine3.24 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dock-service ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dock-migrate ./cmd/migrate

FROM alpine:3.24

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/dock-service /dock-service
COPY --from=builder /out/dock-migrate /dock-migrate

USER 65532:65532
ENTRYPOINT ["/dock-service"]
