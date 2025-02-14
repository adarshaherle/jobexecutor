# Stage 1: Build the binary
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o jobworker-server ./cmd/server/main.go

# Stage 2: Create a minimal image
FROM alpine:3.18
RUN addgroup -S app && adduser -S app -G app
WORKDIR /home/app
COPY --from=builder /app/jobworker-server ./
USER app
EXPOSE 50051
ENTRYPOINT ["./jobworker-server"]
