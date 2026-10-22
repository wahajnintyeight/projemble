---
name: efficient-code
description: Write, refactor, optimize, and review clean code with bounded resource use for production applications targeting 100k or more concurrent users and databases of 50GB or more. Use for backend endpoints, SQL or ORM queries, large datasets, batch jobs, concurrency, throughput, latency, memory, connection pools, caching, and performance-sensitive frontend work. Apply during feature implementation as well as performance fixes when scale is stated or evident. Use measurement and query plans to choose simple optimizations; do not invent capacity guarantees or add speculative infrastructure. Do not use for prose-only tasks or trivial cosmetic edits.
---

# Efficient Code

Write understandable code whose work, memory, I/O, and concurrency stay bounded as traffic and data grow. Treat 100k+ concurrent users and 50GB+ database storage as design targets, not proof of capacity. Preserve correctness, security, tenant isolation, and existing contracts.

## Read only what applies

- Read [database guidance](references/database.md) for SQL, ORM, schema, pagination, or database-bound work.
- Read [measurement and capacity guidance](references/measurement.md) for benchmarks, load tests, profiling, resource budgets, and claims about supported traffic.
- The user's [book repository](https://github.com/PacktPublishing/The-Art-of-Writing-Efficient-Programs) supplies the measurement, memory, data-structure, and concurrency foundation. The database and service guidance here extends those ideas; it is not a summary of an unseen book.

## 1. Trace the real work before editing

Read repository instructions, the affected code, callers, tests, schema, indexes, and existing utilities. Search with search_files. Follow the path from input through queries, computation, dependencies, serialization, and output. Fix shared causes rather than one symptom or one caller.

Establish what actually scales: rows scanned, bytes moved, query count, allocations, concurrent tasks, connections, queue depth, or lock duration. Reuse the existing language, framework, libraries, instrumentation, and deployment patterns.

For new features, choose sound algorithms and bounded access patterns immediately; a missing baseline is not permission to ship obvious N+1 queries or unbounded reads. For optimizations, gather evidence before claiming a speedup.

## 2. Define a small performance budget

Infer available requirements from the task and repository. Ask only for information that blocks a correct implementation; otherwise state assumptions and proceed.

Record the relevant target and context:

- Endpoint or job, p95/p99 latency or completion time, throughput, error budget.
- Active versus connected users, request rate, burst shape, request mix, payload size.
- Database engine/version, table cardinality, tenant skew, read/write mix, hot working set.
- Deployment instance count, CPU/memory limits, database connection budget, downstream limits.

100k connected users do not imply 100k simultaneous database queries. Total database bytes do not reveal table row counts or selectivity. Calculate per-request work and aggregate resource demand using explicit assumptions; see the measurement reference.

## 3. Choose the smallest change that removes the bottleneck

Prefer, in order:

1. Eliminate unnecessary work, repeated calls, unused data, or redundant conversions.
2. Reuse existing code, standard-library algorithms, and native database/platform features.
3. Improve algorithmic complexity or access patterns: indexed lookup, batching, bounded pagination, streaming.
4. Reduce copies, allocation churn, payloads, and hot-path contention where evidence supports it.
5. Add a cache, concurrency, or infrastructure only when the simpler options cannot meet the requirement.

Optimize the dominant path. A local speedup has little effect when another stage dominates. Do not introduce microservices, sharding, lock-free algorithms, speculative abstractions, or dependencies solely because the target says 100k users.

Keep names clear, functions cohesive, and error paths explicit. Follow local formatting and types. Keep every code or instruction file at or below 1,000 lines; split by responsibility before reaching that limit. Avoid arbitrary layers and helpers with only one trivial caller.

## 4. Keep data access bounded

Select needed fields and push appropriate filtering, aggregation, and limiting into the database. Avoid materializing a full table to select a page. Eliminate N+1 queries without replacing them with unbounded eager loading or huge `IN` lists.

Match indexes to actual predicates and ordering, accounting for writes, storage, selectivity, and existing indexes. Use engine-specific plans rather than assuming every index or join is faster. Preserve row cardinality and business semantics when changing joins or aggregation.

Use stable pagination and explicit page/batch limits. Use keyset pagination for deep sequential browsing when the API permits it. Include a unique tie-breaker; preserve null, time-zone, tenant, and authorization semantics. See the database reference before changing query behavior.

## 5. Bound concurrency and protect shared resources

- Bound task fan-out, pool checkout waits, batches, retries, queues, and in-flight work. A semaphore around one task per input still creates unbounded tasks for a huge input; use bounded workers or bounded batches.
- Budget connection pools across all instances, workers, background jobs, and autoscaling limits. Do not set pool size equal to user count. Account for operational headroom and observed database capacity.
- Apply dependency timeouts within an end-to-end deadline; cancel work and release resources when the request ends.
- Retry only eligible transient failures and safe/idempotent operations. Bound attempts, backoff, jitter, and total deadline so retries do not multiply an outage.
- Keep transactions and critical sections short. Use database constraints and atomic operations to protect correctness; avoid holding a transaction open across external network calls.
- Apply backpressure or deliberate overload rejection when capacity is exhausted. Bound memory instead of accumulating an unlimited queue.
- Avoid shared mutable state where possible. Use established synchronization before considering custom atomics or lock-free code.

Async I/O can overlap waiting; it does not add database capacity or make CPU-bound code faster. Respect runtime event-loop and thread-safety constraints.

## 6. Make caching and bulk work explicit

Cache only when reuse and acceptable staleness justify it. Specify authorization/tenant-aware keys, TTL or invalidation, size bounds, ownership, and outage behavior. Prevent hot-key cache stampedes through existing request coalescing or appropriate coordination. Local coordination covers only one process; explain aggregate refresh demand across instances. Never share personalized data under a global key.

Stream or chunk exports and imports with bounded buffers and transactions. Define retry/checkpoint semantics for long-running work. Avoid copying entire datasets or scheduling every row concurrently. Keep background work from starving interactive requests.

For CPU-bound work, inspect algorithmic complexity, repeated computation, allocations, copies, and data locality. Use the runtime's built-in containers and optimized operations first. Profile before SIMD, compiler-specific tuning, custom allocators, or lock-free structures. Benchmark in the relevant optimized build; do not sacrifice numerical or concurrency correctness for misleading gains.

For frontend work, bound API responses and rendering work, remove repeated requests, and use existing pagination or virtualization when the visible dataset warrants it. Keep accessibility and loading/error states intact.

## 7. Prove behavior and measure impact

Leave one small runnable regression check for nontrivial logic, using existing tests or a standard-library self-check when no harness exists. Test the failure that motivated the change and meaningful boundaries: empty input, ties in pagination, tenant separation, duplicates, concurrent calls, cancellation, or dependency failure as applicable.

Compare before and after on the same workload and environment. Use representative cardinality, skew, read/write mix, payloads, and contention; tiny uniform fixtures prove behavior, not production capacity. Capture query count, rows/bytes processed, memory, p95/p99 latency, errors, and resource saturation as relevant. Repeat enough to distinguish a gain from noise; retain reproducible commands.

When realistic measurement is unavailable, implement the bounded design, run available correctness checks, and state precisely what remains unverified. Never fabricate baseline numbers, benchmarks, index use, or a guarantee that 100k users are supported.

Treat production load tests, expensive execution plans, and schema changes according to the user's authorization and environment. Prepare reviewable migrations and rollback notes. Assess lock duration, disk/WAL growth, build overhead, and engine support before applying large-table changes.

## Deliver the result

Implement the authorized change rather than stopping at recommendations. Keep the final response brief unless an analysis was requested:

- What changed and which source of work it removes.
- What checks and measurements actually ran, with before/after evidence if available.
- The remaining capacity assumption or material trade-off, and the next measurement needed.

Separate measured facts from expected effects. For a review, rank concrete findings by impact with file/line evidence and a minimal fix; avoid a generic performance checklist.
