# ADR-0001: Single-instance backend deployment

**Status:** Accepted — 2026-06-10

## Context

Three subsystems hold per-process in-memory state:

- `services/checker/jobbus.go` — SSE progress fan-out and job cancellation via in-process channel maps.
- `services/checker/jobrunner.go` — check jobs run as in-process goroutines; startup recovery (`services/checker/service.go`) assumes any `queued`/`running` row at boot is orphaned.
- `services/scheduler/scheduler.go`, `services/notify/notify.go` — robfig/cron entries registered in memory per process.

## Decision

The backend is deployed as exactly one instance (single Docker container behind nginx). Horizontal scaling of the backend is out of scope.

## Consequences

- SSE progress, job cancellation, and cron scheduling are correct only with one replica. Running >1 replica causes: progress streams that never receive events, duplicate cron fires, and startup recovery failing jobs that are still running on another replica.
- Future multi-replica support requires: a Redis/PubSub-backed `JobBus` adapter (the `JobBus` interface is the seam), a distributed cron lease (DB advisory lock or leader election), and heartbeat-based orphan detection instead of recover-all-at-boot.
- Architecture reviews should not re-flag the in-process JobBus/cron as defects while this ADR stands.
