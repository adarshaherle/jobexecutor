  

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

  

-  **Job Submission**: Accepts commands and assigns a unique Job ID, initiating the job lifecycle from Queued to completion.

-  **State Management**: Tracks job status entirely in memory, transitioning through Queued → Running → Completed/Failed/Canceled.

-  **Process Control**: Runs jobs as external processes, capturing output for monitoring and retrieval.

-  **Resource Isolation**: Utilizes Linux cgroups to enforce per-job CPU, memory, and I/O limits, preventing system resource monopolization.

-  **Single-Node Operation**: Runs independently without distributed coordination, ensuring simplicity and reliability.

This design ensures a lightweight yet robust job execution framework, balancing security, performance, and maintainability.

  

## Design Approach

  

The service is built with a simple, robust design:

-  **In-Memory Store**: A global map (protected by a mutex) tracks active jobs, ensuring efficient access and updates. The mutex prevents race conditions when modifying job states, allowing safe concurrent execution.

-  **Job Submission**: Accepts commands and assigns a unique Job ID, initiating the job lifecycle from Queued to completion. Before starting, the service checks if a job with the same ID is already running, ensuring job uniqueness and preventing duplicate executions.

-  **Asynchronous Execution**: Jobs are dispatched to worker goroutines that execute the command and capture output.

-  **State Transitions**: Jobs move from Queued to Running, then to Completed, Failed, or Canceled.

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

  

## **Job Lifecycle Management**

Efficient job lifecycle management ensures seamless execution, monitoring, and termination of tasks, maintaining reliability and consistency.

**Starting a Job**

-  **Initiation**: Clients initiate jobs via the `StartJob` RPC.

-  **Execution**: The server launches the specified command using `exec.Command`.

-  **Identification**: A unique Job ID is generated and returned for future reference.

-  **Output Handling**: Standard output and error are captured and managed.

-  **Tracking**: Jobs are stored in a thread-safe, in-memory structure.

  

**Monitoring Job Status**

-  **Status Tracking**: The job manager monitors each job's state.

-  **Transitions**:

	  -	*Queued* → *Running*: Execution begins.

	 -	*Running* → *Completed*: Successful completion.

	  -	*Running* → *Failed*: Error occurred.

	 -	Running* → *Cancelled*: User-initiated termination.

  

-  **Client Queries**: Status can be checked via the `GetStatus` RPC.

-  **Persistence**: Completed jobs remain in memory until a server restart.

  

**Stopping a Job**

-  **Termination Request**: Clients send a `StopJob` RPC with the Job ID.

-  **Process Termination**: The server terminates the associated process.

-  **Status Update**: The job's status changes to *Cancelled*, and resources are freed.

-  **Confirmation**: A `JobStopResponse` confirms termination, with errors reported via gRPC status.

-  **Concurrency Control**: Locks ensure consistent state updates and prevent deadlocks.

  

**Output Capture and Streaming**

-  **Capture Mechanism**: Upon job initiation, the system captures stdout and stderr by redirecting them into an in-memory, thread-safe buffer.

-  **Streaming to Clients**: Clients can access the captured output via the `StreamOutput` RPC, which provides both historical and real-time data.

-  **Handling Late Subscribers**: New subscribers receive the entire buffered output first, followed by live data, ensuring no loss of information.

-  **Concurrent Streaming Support**: Multiple clients can stream the same job's output simultaneously, managed through a list of active subscribers and synchronized broadcasting.
  

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
// Request by job ID.
message JobIDRequest {
    string job_id = 1;
}
// Response for StopJob.
message JobStopResponse {}
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
    rpc StopJob(JobIDRequest) returns (JobStopResponse);
    rpc GetStatus(JobIDRequest) returns (JobStatusResponse);
    rpc StreamOutput(JobOutputRequest) returns (stream JobOutputChunk);
}
```
 

## Resource Isolation with Cgroups v2

Each job is assigned a dedicated cgroup to enforce resource limits on CPU, memory, and disk I/O, ensuring controlled execution.

### Execution Flow

  

-  **Cgroup Creation**: A unique cgroup is created for each job under `/sys/fs/cgroup/jobexec/<job_id>`.

  

-  **Process Association**: The job's PID is assigned to `cgroup.procs`, isolating it and its child processes.

  

-  **Resource Limits Applied**:

	-	**CPU**: `cpu.max` restricts CPU allocation.

   -	**Memory**: `memory.max` sets a memory ceiling.

   -	**Disk I/O**: `io.max` throttles disk operations.

  

### Behavior & Flexibility

-  **Linux Enforcement**: Cgroup limits are fully applied.

-  **Non-Linux/Disabled Mode**: Uses no-op stubs for development environments without cgroup support.

-  **Graceful Degradation**: If cgroups are unavailable, logs warnings but still executes jobs without restrictions.

  

This mechanism ensures jobs operate within defined limits, preventing resource contention across the system. &#x20;

## Security: mTLS Authentication and Authorization

Security is a cornerstone of the Job Executor design. The server enforces mutual TLS (mTLS) for all client connections, ensuring secure and authenticated communication. Both the server and client use X.509 certificates to verify each other’s identity, preventing unauthorized access.

### Authentication

- The gRPC server is configured to require client authentication with a trusted certificate authority (CA).

- The server certificate, key, and CA certificate must be provided via configuration flags (`--cert`, `--key`, `--ca`).

- The CLI client also loads its client certificate and CA certificate to authenticate with the server.

### Authorization

- Authentication is handled at the TLS layer, allowing only clients with valid certificates to connect.

- Currently, authorization is implicit: any authenticated client can invoke RPC methods.

- Future enhancements could restrict access further by validating certificate attributes such as Common Name (CN) against an allowlist.

### Secure Configuration

- The server enforces strong cryptographic defaults, supporting only TLS 1.2 and higher.

- Secure cipher suites are used by default, avoiding weak encryption configurations.

- Certificates must be generated using robust cryptographic algorithms such as RSA 4096 or ECC.

  

This approach ensures that communication between clients and the Job Executor server remains secure while keeping the authentication mechanism simple and effective.

## CLI Tool and User Experience

The Job Executor provides a command-line interface (CLI) tool, `jobexec`, built using the Cobra library. This CLI tool serves as a client for the gRPC API, enabling users to manage job execution seamlessly.

### Commands

-  **`jobexec start <cmd> [args...]`** – Submits a new job request via `StartJob` RPC and returns a unique Job ID upon success.

-  **`jobexec status <job-id>`** – Queries job status using `GetStatus` RPC, displaying the current state and exit code if the job has finished.

-  **`jobexec stop <job-id>`** – Sends a termination request via `StopJob` RPC to cancel an active job.

-  **`jobexec logs <job-id>`** – Streams job output in real-time using `StreamOutput` RPC, displaying both historical and live logs until job completion or user interruption.

  

### Additional Features

-  **`--output=json`** – Formats responses as machine-readable JSON for integration with scripts.

-  **`--verbose`** – Enables detailed logging for debugging purposes.

Under the hood, the CLI tool will be configured to load the necessary TLS certificates (via flags or environment variables) to establish a secure mTLS connection to the server.The design proposes a CLI tool (referred to as the jobexec command) to be built using the Cobra library. This planned tool will serve as a client for the gRPC API, enabling users to interact with the Job Executor server.

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
    
-   **Basic Concurrency:** Job execution uses simple concurrency controls (e.g., semaphores) without a sophisticated worker pool. This limits dynamic scaling and persistence of queued jobs.
    
-   **Immediate Execution:** Jobs run as soon as a slot is available, with no built-in scheduling for future execution.
    
-   **Security Trade-off:** mTLS ensures strong security but lacks granular access controls (like RBAC or API tokens), treating all authenticated clients equally.
    
-   **Limited Features:** Missing capabilities such as automatic retries, dependency management, and multi-node distribution restrict the system to simple, controlled environments.
    

Overall, the design emphasizes simplicity and rapid development, making it suitable for development, demos, or prototyping, while requiring additional enhancements for production use.
