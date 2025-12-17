# build
FROM golang:1.25.4 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o statsys .

# copy
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/statsys .
COPY --from=builder /app/www /app/www

EXPOSE 8888/tcp
CMD [ "/app/statsys", "-config", "data/config.toml", "-db", "data/status.db" ]