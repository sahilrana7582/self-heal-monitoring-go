# self-heal-monitoring-go

A small **self-healing job monitor** written in Go. You register background jobs (goroutines). If any job panics or exits, the monitor notices and restarts it automatically.

## Why

In Go, a background goroutine can die quietly:

- An **unrecovered panic** in any goroutine crashes the **whole process**.
- A job that **returns early** because of a bug or an unexpected condition just stops. Nothing restarts it, and often nobody notices.

Long-running services (workers, pollers, cleaners, heartbeats) need someone watching them. Erlang supervisors, `systemd` `Restart=always` and Kubernetes restart policies all do this job. This project builds that idea inside one Go process, from scratch, so we understand every piece before making it production grade.

## How it works

```
register(name, fn)          runJob(name)              monitor()
       │                         │                        │
       ▼                         ▼                        ▼
 wrap fn with:              status = true           every N seconds:
  - recover() panic         wg.Add(1)                 for each job:
  - status = false          go wrapper()                if status == false
  - wg.Done()                                             → restart it
```

1. **`register`** wraps the job function. The wrapper:
   - recovers any panic, so one bad job can't kill the program,
   - marks the job `status = false` (not running) when it panics or returns,
   - calls `wg.Done()`.
2. **`runJob`** marks the job as running and starts it in a goroutine. It won't start a second copy if the job is already running.
3. **`monitor`** runs a ticker in the background. On every tick it restarts every job whose status is `false`.
4. **`main`** blocks on `wg.Wait()`. The monitor holds its own slot in the WaitGroup, so the process keeps running even when every job is down at the same moment.

All access to the `jobs` map goes through `m.lock`.

## Demo jobs

`main.go` ships with four jobs. Each one fails in a different way:

| Job             | What it does                    | How it fails                                         |
|-----------------|---------------------------------|------------------------------------------------------|
| `a`             | Prints `A Running` 5 times      | Always panics with a string after 5s                 |
| `heartbeat`     | Prints a beat every second      | Random panic (~1 in 8 beats)                         |
| `order-worker`  | "Processes" fake orders         | Random **runtime panic**: index out of range         |
| `cache-cleaner` | Cleans a cache every 2 seconds  | Random **runtime panic**: assignment to entry in nil map |

The monitor checks every **5 seconds**. You set this in `main()` with `newJobMonitor(5 * time.Second)`.

## Run

```bash
go run main.go

# with the race detector (recommended while developing)
go run -race main.go
```

Sample output:

```
Heartbeat: beat 3
Job Panicked: heartbeat: heartbeat lost at beat 3
Worker: processing order-104
Job Panicked: order-worker: runtime error: index out of range [5] with length 5
A Running
Job Panicked: a: A Panic
Job Restart: heartbeat
Job Restart: order-worker
Job Restart: a
...
```

Stop it with `Ctrl+C`.

## TODO: toward a production-grade monitor

### Must have
- [ ] **Add `go.mod`** (`go mod init ...`) so the project builds as a proper module.
- [ ] **Graceful shutdown**: pass a `context.Context` to every job, catch `SIGINT`/`SIGTERM`, stop the ticker, cancel jobs and wait for them to finish.
- [ ] **Jobs return an error**: change the signature to `func(ctx context.Context) error`, so a job can report failure without panicking.
- [ ] **Restart backoff**: use exponential backoff with jitter instead of restarting at a fixed interval. A crash-looping job should not spin the CPU or flood logs.
- [ ] **Max restarts / crash-loop protection**: give up (or alert) after N restarts in a time window.
- [ ] **Capture stack traces** on panic with `runtime/debug.Stack()`, so the logs show *where* the job died.
- [ ] **Unit tests** for register, run, panic recovery and restart, always run with `go test -race`.

### Should have
- [ ] **Restart immediately**: the wrapper sends a "job died" event on a channel and the monitor reacts at once, with no ticker delay.
- [ ] **Per-job restart policy**: `always`, `on-failure` or `never` (some jobs are meant to finish).
- [ ] **Job metadata**: restart count, last error, last start time, uptime.
- [ ] **Structured logging** with `log/slog` instead of `fmt.Printf`.
- [ ] **Stop / unregister** a single job at runtime.
- [ ] **Move the monitor into its own package** (`monitor/`) with an exported API. `main.go` then only wires up jobs.

### Nice to have
- [ ] **Hung-job detection**: jobs send heartbeats, and the monitor restarts a job that is alive but stuck.
- [ ] **Metrics** (Prometheus): `job_up`, `job_restarts_total`, `job_panics_total`.
- [ ] **HTTP status endpoint**: `/healthz` and `/jobs` show live job state.
- [ ] **Alerting hook**: Slack or a webhook when a job crash-loops.
- [ ] **Configuration** for the interval, backoff and restart limits (flags, env vars or a config file).
