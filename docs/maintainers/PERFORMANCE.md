# Performance Benchmarks

This guide defines the benchmark surface for issue `#34` and the maintainer
budget policy for performance regressions.

These benchmarks are not part of the default pull request CI contract. They are
maintainer-facing review tools for tracking regression risk in hot paths.

## Benchmark Commands

Run the three benchmark groups independently:

- `go test ./internal/core -run=^$ -bench BenchmarkValueJSONRoundTrip -benchmem`
- `go test . -run=^$ -bench BenchmarkWebSocketClientSubscriptionThroughput -benchmem`
- `go test ./baseclient -run=^$ -bench BenchmarkReplayOngoingRequests -benchmem`

Capture each command's output to a text file using shell's normal output
redirection.

## Budget Comparison Workflow

1. Run the same benchmark command on the base ref and on the candidate ref.
2. Save both outputs to text files.
3. Compare them with:

```text
go run ./cmd/convex-go-maint bench-compare -old base.txt -new change.txt -max-regression=25
```

`bench-compare` normalizes the `-N` CPU suffix from benchmark names and checks
three budget metrics:

- `ns/op`
- `B/op`
- `allocs/op`

## Budget Policy

Use these defaults unless the PR intentionally changes the benchmarked
semantics:

- keep regressions at or below `25%` for `ns/op`, `B/op`, and `allocs/op`
- compare runs from the same Go version and the same OS class when possible
- treat `allocs/op` regressions as the strongest signal when small `ns/op`
  changes look like runner noise
- if a benchmark is intentionally replaced or renamed, update this guide and
  the repohealth guard in the same PR

## Covered Benchmarks

| Benchmark | Scope | Why it matters |
| --- | --- | --- |
| `BenchmarkValueJSONRoundTrip` | `internal/core` encode plus decode of representative Convex values | Protects the value wire path used by HTTP and realtime APIs. |
| `BenchmarkWebSocketClientSubscriptionThroughput` | root realtime client delivery of subscription updates through the fake sync transport | Protects steady-state subscription update throughput. |
| `BenchmarkReplayOngoingRequests` | `baseclient` reconnect replay over queued mutations | Protects reconnect recovery work as request counts grow. |
