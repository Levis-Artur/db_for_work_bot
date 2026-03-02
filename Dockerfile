FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bot ./cmd/bot

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D appuser
WORKDIR /app
COPY --from=builder /out/bot /app/bot
USER appuser
ENTRYPOINT ["/app/bot"]
