# RFD 0001 – Proposed Job Executor Design

Authors: Adarsha Raghavendra
Created: 2025-02-27
Status: Proposed

## What

This document outlines the design and implementation plan for Job Executor, a system for running Linux processes as background jobs. It focuses on process lifecycle management, secure remote control, and resource isolation via Linux cgroups. The gRPC API, secured with mTLS, and a CLI tool facilitate job management. Cgroups enforce CPU, memory, and disk I/O limits, ensuring job isolation and preventing resource overuse.

## Why

The Job Executor is designed to provide a secure, efficient, and controlled environment for executing background processes. It ensures:

-  **Secure Execution**: Mutual TLS (mTLS) guarantees encrypted communication and verified client-server authentication.
-  **Robust Job Control**: Provides a structured API to start, stop, query, and monitor jobs seamlessly.
-  **Resource Enforcement**: Uses Linux cgroups to contain CPU, memory, and I/O consumption, preventing resource overuse.
  
With a focus on simplicity, reliability, and maintainability, the Job Executor ensures seamless job execution while maintaining security, efficiency, and system stability.

## Scope

Job Executor is designed for efficient execution and management of background jobs, focusing on security, resource control, and simplicity. It handles:

-  **Job Submission**: Immediately attempts to run the job and returns either a running job ID or an immediate error if the job cannot start.
-  **State Management**: Tracks job status entirely in memory, transitioning through Running → Completed/Failed/Canceled.
-  **Process Control**: Runs jobs as external processes, capturing output for monitoring and retrieval.
-  **Resource Isolation**: Utilizes Linux cgroups to enforce per-job CPU, memory, and I/O limits, preventing system resource monopolization.
-  **Single-Node Operation**: Runs independently without distributed coordination, ensuring simplicity and reliability.

This design ensures a lightweight yet robust job execution framework, balancing security, performance, and maintainability.

## Design Approach

The service is built with a simple, robust design:

-  **In-Memory Store**: A global map (protected by a mutex) tracks active jobs, ensuring efficient access and updates. The mutex prevents race conditions when modifying job states, allowing safe concurrent execution.
-  **Job Submission**: Accepts commands and assigns a unique Job ID. The server attempts to launch the specified command immediately, returning either a valid Job ID for a running job or an error describing why the job failed to start.
-  **Asynchronous Execution**: Jobs are dispatched to worker goroutines that execute the command and capture output.
-  **State Transitions**: Jobs immediately enter the Running state and subsequently transition to Completed, Failed, or Cancelled.
-  **Locking**: A global mutex protects shared job state, ensuring thread-safe updates. Each job operation (submission, status update, termination) acquires the lock before modifying the in-memory store, preventing race conditions and maintaining consistency.

## Architecture

The proposed design will employ a client-server architecture:

### Server

- Implements core job lifecycle management functions.
- Exposes a gRPC API for job lifecycle operations.
- Enforces resource limits through Linux cgroups.

**Server Internal Structure:**

-  **Job Manager Library**: Handles process creation, output capturing, and job state management.
-  **gRPC Service Layer**: Exposes job management functionality over secure connections.
-  **Resource Isolation Layer**: Utilizes Linux cgroups to apply resource limits on a per-job basis.

### CLI Client

- Provides an interface for users to interact with the server.
- Supports commands such as start, status, stop, and logs.

## Job Lifecycle Management

Efficient job lifecycle management ensures seamless execution, monitoring, and termination of tasks, maintaining reliability and consistency.

**Starting a Job**

-   **Initiation**: Clients initiate jobs via the `StartJob` RPC.
-   **Execution**: The server launches the specified command using `exec.Command`.
-   **Identification**: A unique Job ID is generated and returned for future reference.
-   **Output Handling**: Standard output and error are captured and managed.
-   **Tracking**: Jobs are inserted into a global map (key: Job ID, value: Job details) protected by a mutex, ensuring thread-safe operations.

**Monitoring Job Status**

-   **Status Tracking**: The global map tracks each job's state, allowing efficient updates and retrieval.
-   **Transitions**:
    -   Running → Completed: Successful completion.
    -   Running → Failed: Error occurred.
    -   Running → Cancelled: User-initiated termination.
-   **Client Queries**: Status can be checked via the `GetStatus` RPC.
-   **Persistence**: Completed jobs remain in memory until a server restart.

**Stopping a Job**

-   **Termination Request**: Clients send a `StopJob` RPC with the Job ID.
-   **Process Termination**: The server terminates the associated process.
-   **Status Update**: The job's status is updated to Cancelled in the global map (protected by mutex), and associated resources are freed.
-   **Removal from Tracking**: The job is removed from the global map upon cancellation.
-   **Confirmation**: A `JobStopResponse` confirms termination, with errors reported via gRPC status.
-   **Concurrency Control**: Mutex ensures consistent updates and prevents deadlocks.

**Output Capture and Streaming**

-   **Capture Mechanism**: Job output (stdout and stderr) is captured from the start by redirecting the output pipes of the job’s `exec.Command` to a thread-safe, in-memory byte buffer protected by mutexes.
-   **Streaming to Clients**: Clients access the complete buffered output via the `StreamOutput` RPC, receiving both historical data from job start and new data in real-time.
-   **Handling Late Subscribers**: New clients connecting to the stream first receive all previously captured data before live-streaming resumes.
-   **Concurrent Streaming Support**: Multiple clients can simultaneously stream a job's output, managed through synchronized broadcasting mechanisms ensuring data consistency and integrity.
-   **Buffer Management**: Currently, outputs are fully buffered in memory, suitable for typical use cases. Future enhancements could include rolling buffers or disk-based storage to accommodate very large outputs efficiently.

Utilizing `exec.Command` ensures compatibility with native OS process management, while mutex-protected structures maintain robust lifecycle management and thread safety.

## gRPC API

### API Overview

The Job Executor exposes a gRPC API to manage job execution remotely. The available methods include:

-  **StartJob**: Initiates a new job and returns a unique Job ID.
-  **GetStatus**: Retrieves the current status and exit code of a job.
-  **StopJob**: Sends a termination signal to a specified job.
-  **StreamOutput**: Streams real-time and historical output from a running job.
 
**gRPC API Definition (Proto File)**

```proto
syntax = "proto3";

package job;

// Enum for job status.
enum Status {
    STATUS_UNKNOWN = 0;
    STATUS_RUNNING = 1;
    STATUS_COMPLETED = 2;
    STATUS_FAILED = 3;
    STATUS_CANCELLED = 4;
}

// Request to start a job.
message JobStartRequest {
    string command = 1;
    repeated string args = 2;
}

// Response from StartJob.
message JobStartResponse {
    string job_id = 1;
}

// Request for StopJob.
message StopJobRequest {
    string job_id = 1;
}

// Response for StopJob.
message JobStopResponse {}

// Request for GetStatus.
message GetStatusRequest {
    string job_id = 1;
}

// Response for GetStatus.
message JobStatusResponse {
    string job_id = 1;
    Status status = 2;
    int32 exit_code = 3;
    string error_message = 4;
}

// Request to stream job output.
message JobOutputRequest {
    string job_id = 1;
}

// Output chunk message.
message JobOutputChunk {
    bytes data = 1;
}

// The JobService definition.
service JobService {
    rpc StartJob(JobStartRequest) returns (JobStartResponse);
    rpc StopJob(StopJobRequest) returns (JobStopResponse);
    rpc GetStatus(GetStatusRequest) returns (JobStatusResponse);
    rpc StreamOutput(JobOutputRequest) returns (stream JobOutputChunk);
}
```

## Resource Isolation with Cgroups v2

Each job is assigned a dedicated cgroup to enforce resource limits on CPU, memory, and disk I/O, ensuring controlled execution.

When you start a job, the executor follows these updated steps to isolate resources:

**1. Create Job Cgroup:**
- Create a unique sub-cgroup under the executor’s cgroup path, such as `/sys/fs/cgroup/executor/<job-id>`.
- Enable necessary controllers (CPU, memory, I/O) in the parent’s `cgroup.subtree_control`.

**2. Apply Resource Limits:**
- **CPU:** Set CPU limits in `cpu.max` to enforce hard CPU caps.
- **Memory:** Configure maximum memory usage in `memory.max`. Exceeding this limit triggers an Out-of-Memory (OOM) kill.
- **Disk I/O:** Define absolute throughput limits for read/write operations in `io.max` for specific devices.

**3. Prepare Command Execution Context:**
- Instantiate the command using `exec.Command`.
- Use `SysProcAttr` with Linux-specific flags to ensure the process starts in a controlled state.

```go
cmd := exec.Command(command, args...)
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true, // New process group for controlled execution
}
```

**4. Assign Process to Cgroup Immediately Upon Creation:**
- Start the process using `cmd.Start()`.
- Immediately assign the process to the job-specific cgroup by writing its PID to `cgroup.procs`. This ensures all subprocesses inherit the cgroup:

```go
pid := cmd.Process.Pid
cgroupProcsPath := fmt.Sprintf("/sys/fs/cgroup/executor/%s/cgroup.procs", jobID)
if err := os.WriteFile(cgroupProcsPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
    log.Errorf("Failed to assign PID %d to cgroup: %v", pid, err)
    cmd.Process.Kill()
    return err
}
```

**5. Enforce Limits During Execution:**
- CPU usage exceeding configured limits is throttled by the kernel.
- Memory usage is monitored, triggering OOM kills if limits are exceeded.
- Disk I/O operations are throttled to maintain resource constraints.

**6. Cleanup After Job Completion:**
- Upon job completion, remove the job’s cgroup directory.
- Optionally, log resource usage statistics for auditing and troubleshooting.

### Behavior & Flexibility

-  **Linux Enforcement**: Cgroup limits are fully applied.

-  **Non-Linux/Disabled Mode**: Uses no-op stubs for development environments without cgroup support.

-  **Graceful Degradation**: If cgroups are unavailable, logs warnings but still executes jobs without restrictions.

## Security: mTLS Authentication and Authorization

Security in Job Executor relies on mutual TLS (mTLS), where both client and server authenticate each other using X.509 certificates signed by a trusted shared Certificate Authority (CA).

### Authentication and Granular Authorization

-   Clients authenticate with an X.509 certificate, which includes a unique identifier in the **Common Name (CN)** field.
-   Upon successful authentication, the server extracts the CN from the client's certificate and associates it with jobs submitted by that user.
-   Users are restricted to interacting exclusively with their own jobs based on the CN, effectively preventing unauthorized access to resources owned by others.

### X.509 Certificate Setup (OpenSSL)

Certificates are generated and signed using the following OpenSSL commands:

```bash
# Generate CA key and certificate
openssl genrsa -out ca.key 4096
openssl req -x509 -new -key ca.key -sha256 -days 3650 -subj "/CN=JobExec-CA" -out ca.crt

# Generate Server key and certificate
openssl genrsa -out server.key 2048
openssl req -new -key server.key -subj "/CN=jobexec.example.com" -out server.csr
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 365 -sha256 -out server.crt

# Generate Client key and certificate (example for user "alice")
openssl genrsa -out alice.key 2048
openssl req -new -key alice.key -subj "/CN=alice" -out alice.csr
openssl x509 -req -in alice.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 365 -sha256 -out alice.crt
```

Generated Files:

-   **ca.key / ca.crt**: CA’s private key and self-signed certificate for signing other certificates.
-   **server.key / server.crt**: Server’s private key and CA-signed certificate.
-   **alice.key / alice.crt**: Client’s private key and CA-signed certificate uniquely identifying the user "alice."

Server and client configurations are set up to mutually trust certificates issued by this shared CA.

The server certificate, key, and CA certificate must be provided via configuration flags (`--cert`, `--key`, `--ca`).

The CLI client also loads its client certificate and CA certificate to authenticate with the server.

### TLS Version Policy

-   TLS **1.3 is enforced by default**, providing enhanced security features including faster handshakes, improved encryption algorithms, and better performance.
-   Cipher suites are explicitly limited to modern and secure options (AES-GCM, ChaCha20-Poly1305).
-   TLS **1.2 fallback is optional** and generally not recommended unless necessary for compatibility reasons, given that TLS 1.3 is strongly preferred due to its improved security posture.

## CLI Tool and User Experience

The Job Executor includes a user-friendly command-line interface (CLI) tool named `jobexec`, built using the Cobra library. This tool acts as a client interacting with the server's secure gRPC API, enabling users to manage job execution efficiently.

### Commands

- **`jobexec start <cmd> [args...]`**
  - Initiates a new job via the `StartJob` RPC.
  - Returns a unique Job ID upon successful submission.

- **`jobexec status <job-id>`**
  - Checks job status through the `GetStatus` RPC.
  - Displays the current job state and exit code (if completed).

- **`jobexec stop <job-id>`**
  - Sends a termination request via the `StopJob` RPC.
  - The server sends a graceful termination signal (`SIGTERM`), escalating to a forceful kill (`SIGKILL`) if necessary after a timeout.
  - Updates job status to "Cancelled" and cleans up resources.

- **`jobexec logs <job-id>`**
  - Streams historical and real-time job output via the `StreamOutput` RPC.

### Additional Features

- **`--output=json`**
  - Outputs responses in JSON format for scripting.

- **`--verbose`**
  - Enables detailed logging for debugging.

The CLI securely connects to the server via mutual TLS (mTLS) using configured certificates.

## Testing and Reliability Considerations

The test plan ensures reliable operation of the Job Executor:

- **Unit Tests**: Validate job initiation, execution, termination, state transitions, and output handling.

- **Integration Tests**: Confirm cgroup resource isolation and secure mTLS communication in Docker setups.

- **Concurrency Testing**: Detect and resolve concurrency issues using Go's race detector.

**Goals**:

- Prevent resource leaks and ensure clean goroutine management.

- Provide consistent, clear error logging for stability and ease of debugging.

## Trade-offs

During the design of Job Executor, several trade-offs were made to keep the system simple and focused. This section outlines the key limitations and decisions, and the reasoning behind them:

-   **In-Memory Storage:** Job definitions and statuses are kept in memory, eliminating the need for a database. This simplifies deployment and speeds development but means data is lost on service restarts and can't be shared between instances.
    
-   **No Caching:** Every request reads fresh from memory. While adequate for small loads, it may lead to inefficiencies if operations become costly at scale.
    
-   **Basic Concurrency:** Job execution uses simple concurrency controls (e.g., semaphores) without a sophisticated worker pool. This limits dynamic scaling and persistence of jobs.
    
-   **Immediate Execution:** Jobs run as soon as a slot is available, with no built-in scheduling for future execution.
    
-   **Security Trade-off:** mTLS ensures strong security but lacks granular access controls (like RBAC or API tokens), treating all authenticated clients equally.
    
-   **Limited Features:** Missing capabilities such as automatic retries, dependency management, and multi-node distribution restrict the system to simple, controlled environments.
