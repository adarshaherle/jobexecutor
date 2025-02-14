# JobExecutor

**JobExecutor** is job worker service written in Go. It leverages the new language features (such as generics and built-in fuzz testing) along with advanced system-level functionality:
- Executes arbitrary Linux processes.
- Streams output from process start.
- Controls resource usage using Linux cgroups (CPU, Memory, Disk I/O).
- Isolates processes using Linux namespaces (PID, mount, network).
- Exposes a secure gRPC API with mTLS authentication.
- Provides a CLI client for remote management.
- Comes with a comprehensive Makefile and Dockerfile for automation and containerization.

## Project Structure

- **cmd/server/**: gRPC server entry point.
- **cmd/cli/**: CLI client entry point.
- **internal/job/**: Core job management library.
- **internal/grpcserver/**: gRPC service implementation.
- **proto/**: Protobuf definitions.
- **Dockerfile**: Container build instructions.
- **Makefile**: Automation for build, test, and run.
- **go.mod**: Go module file.

## Prerequisites

- Go 1.21 (new features such as generics and fuzz testing are used)
- Linux for full functionality (cgroups and namespaces). On macOS, stubs are used.
- Docker (optional) for containerization. Run with `--privileged --cgroupns=host` for cgroup support.

## Setup

1. **Clone the repository:**
    ```bash
    git clone https://github.com/adarshaherle/jobexecutor.git
    cd jobexecutor
    ```

2. **Initialize modules and tidy:**
    ```bash
    go mod tidy
    ```

3. **Generate gRPC code:**
    ```bash
    protoc --go_out=. --go-grpc_out=. proto/job.proto
    ```

## Build & Test

- **Build binaries:**
    ```bash
    make build
    ```

- **Run tests (with race detection and fuzz testing):**
    ```bash
    make test
    ```

- **Run the gRPC server locally:**
    ```bash
    make run
    ```

## Docker

- **Build the Docker image:**
    ```bash
    make docker-build
    ```

- **Run the Docker container:**
    ```bash
    make docker-run
    ```
  (Ensure Docker runs with `--privileged --cgroupns=host` for full cgroup functionality.)

## GitHub Integration

To push this project to GitHub under your account (`https://github.com/adarshaherle`):

```bash
git init
git add .
git commit -m "Initial commit: Level 5 job worker service with Go 1.21 features"
git remote add origin https://github.com/adarshaherle/jobexecutor.git
git branch -M main
git push -u origin main
