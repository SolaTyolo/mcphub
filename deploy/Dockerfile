FROM golang:1.23-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /mcphub ./cmd/mcphub

FROM node:20-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /mcphub /app/mcphub
COPY .env.example /app/.env.example
EXPOSE 8090
CMD ["/app/mcphub"]
