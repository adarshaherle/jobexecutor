# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /app
# Install protoc, protobuf development files, build tools, and make.
RUN apk add --no-cache protobuf protobuf-dev build-base make
# Install protoc plugins.
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# Ensure /go/bin is in PATH.
ENV PATH="/go/bin:${PATH}"
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Generate gRPC code.
RUN make proto
# Build binaries.
RUN make build

# Stage 2: Tester – runs tests.
FROM builder AS tester
CMD ["go", "test", "-v", "-race", "./internal/..."]

# Stage 3: Runtime
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/jobexecutor-server .
COPY certs/server.crt .
COPY certs/server.key .
COPY certs/ca.crt .
EXPOSE 50051
CMD ["./jobexecutor-server", "--cert=server.crt", "--key=server.key", "--ca=ca.crt", "--addr=:50051"]
