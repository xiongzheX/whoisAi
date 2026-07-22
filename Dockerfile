# syntax=docker/dockerfile:1

FROM node:22-alpine AS socketio-client

WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --omit=dev

FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
COPY runner_race/go.mod ./runner_race/go.mod
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY runner_race ./runner_race

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/whoisai \
    ./cmd/go-server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/whoisai ./whoisai
COPY --from=socketio-client /src/node_modules/socket.io/client-dist ./node_modules/socket.io/client-dist
COPY client ./client
COPY runner_race/web ./runner_race/web

USER app

ENV PORT=10000
EXPOSE 10000

CMD ["./whoisai"]
