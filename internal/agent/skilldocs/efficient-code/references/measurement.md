# Measurement and capacity

## Turn user count into a workload

Separate connected users, active users, requests in flight, requests per second, and database connections. State the request mix and assumptions before doing arithmetic.

Illustration only: 100,000 users each making one request every 20 seconds average 5,000 requests/second. At 0.2 seconds mean response time, Little's Law suggests roughly 1,000 requests in flight in a stable system. These are assumed inputs, not measured capacity. Think time, bursts, session behavior, and overload change the workload. Use the mean for Little's Law, not p95 or p99.

Estimate total DB operations from request rate, queries/request, and cache hit ratio. Estimate memory from in-flight requests, payloads, buffers, tasks, and bounded caches. Budget all instances' pools plus workers and operational reserve against tested DB capacity; `max_connections` is a limit, not a throughput recommendation. Connection hold time matters independently of query execution time.

Record an end-to-end latency budget, including queue/pool waits, dependencies, processing, and serialization. Preserve timeout ordering and cancellation. Avoid increasing every pool or parallelism limit when the bottleneck is already saturated.

## Establish a reproducible baseline

Capture commit/build, runtime/compiler flags, hardware/container limits, instance count, configuration, dataset distribution, and workload script. Compare the same environment and work before and after.

Use existing profiling and observability tools. Follow time and allocations through request traces, database plans, CPU profiles, GC, I/O, network, locks, and pool waits. Sample telemetry and avoid high-cardinality labels or logging sensitive data. Optimize the dominant measured cost first.

For CPU microbenchmarks, use the project's existing benchmark facility. Otherwise use a small standard-library harness with a monotonic high-resolution clock, repeated runs, realistic inputs, and checked results. Keep setup outside the timed section unless setup is part of production cost. Warm up JITs where appropriate, check optimized builds, prevent dead-code elimination in compiled languages, and distinguish hot/cold cache conditions. A microbenchmark is not an application capacity test.

## Load testing that exposes overload

Use an isolated, authorized environment with representative rows, tenant skew, payload sizes, read/write mix, and slow dependencies. Use the installed load-test tools or a minimal existing harness; do not introduce a test framework by default.

Ramp gradually and capture steady state, bursts, and a sustained run when these matter. Record offered rate, achieved throughput, p50/p95/p99, errors/timeouts, CPU, memory/GC, database I/O/locks, pool wait, active connections, queue depth, and cache behavior.

When testing a target arrival rate, use an arrival-rate workload and verify the generator can sustain it. A closed-loop client waits for responses and can hide overload by slowing its own request rate; account for coordinated omission and include queue/wait time in reported latency. Validate connected-user targets separately from request throughput.

Observe the saturation point and failure mode. Check that overload is bounded and recoverable: queues do not grow forever, deadlines release resources, errors are deliberate, and retries/cache misses do not create a secondary surge. Include cancellation, duplicate requests, dependency failure, and recovery where relevant.

Stop at documented resource or error thresholds. Do not aim a 100k-client test at production unless that target and environment are authorized.

## Interpret and report evidence

Repeat comparable runs enough to understand variance. Report measured sample counts, workload, duration, and limitations alongside improvements. Do not imply that toy data reproduces a 50GB working set or that an uncongested single request proves peak capacity. Separate correctness checks, query-plan evidence, microbenchmarks, and end-to-end load results.

If measurement cannot run, deliver the code and a concrete verification command/plan. State the unavailable environment or missing data. Say "expected to reduce queries from N+1 to two" when only code analysis exists; say "observed two queries" only when instrumentation actually recorded it.

## Book grounding

The [Packt repository for The Art of Writing Efficient Programs](https://github.com/PacktPublishing/The-Art-of-Writing-Efficient-Programs) provides C++ examples and describes hardware utilization, data organization, memory ordering, contention, lock-free trade-offs, compiler optimization, and efficient APIs. Apply these ideas to the user's runtime; do not mechanically transplant C++ memory-model or compiler techniques into managed runtimes or SQL engines.

The service budgeting and load-testing workflow above is additional applied guidance. The repository is companion code, not the full book; do not claim to have read unavailable chapters or copy book text.

Examples inspected when authoring this skill:

- [Chapter02/11_compare_mbm.C](https://github.com/PacktPublishing/The-Art-of-Writing-Efficient-Programs/blob/master/Chapter02/11_compare_mbm.C): string-comparison benchmarks using optimized build flags and `DoNotOptimize`. Keep benchmark setup separate and prevent unused results from disappearing.
- [Chapter07/01b_per_thread_count.C](https://github.com/PacktPublishing/The-Art-of-Writing-Efficient-Programs/blob/master/Chapter07/01b_per_thread_count.C): thread-local accumulation followed by a short locked update. Reduce shared work before attempting sophisticated synchronization.
- [Chapter09/02_rvo.C](https://github.com/PacktPublishing/The-Art-of-Writing-Efficient-Programs/blob/master/Chapter09/02_rvo.C): instrumentation of construction, copying, moving, and returning a value. Inspect actual data movement and runtime/compiler behavior before manual copy-avoidance tricks.
