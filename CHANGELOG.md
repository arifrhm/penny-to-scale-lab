# Changelog - Penny to Scale Lab

Semua benchmark dijalankan secara otomatis dengan data resource hardware riil terlampir pada setiap rilis arsitektur.

## [0.1.0] - 2026-11-15 - Naive Baseline (net/http + encoding/json)

### 💻 System & Environment Telemetry
- **Host CPU:** Intel(R) Xeon(R) Platinum 8173M CPU @ 2.00GHz (10 vCPUs)
- **Memory (RAM):** 23Gi Total
- **Operating System:** Linux 6.8.0-147-generic x86_64
- **Runtime:** go1.23.2 linux/amd64
- **Load Tool:** bombardier v1.2.6 (125 connections, 10s duration, ~1KB JSON payload)

### 📊 Benchmark Results
| Metric Category | Metric Name | Measured Value | Unit / Note |
| :--- | :--- | :--- | :--- |
| **Throughput** | Mean Request Rate | **41966.72** | requests/sec |
| **Total Volume** | Successful 2xx Requests | **419663** | requests in 10s |
| **Tail Latency** | Median (p50) | **0.80** | milliseconds |
| **Tail Latency** | 90th percentile (p90) | **10.15** | milliseconds |
| **Tail Latency** | 99th percentile (p99) | **19.41** | milliseconds |
| **Memory Alloc** | Per-operation Heap Alloc | **10574** | Bytes/op (reflection overhead) |
| **Memory Alloc** | Per-operation Allocs Count | **79** | allocs/op |
| **Microbenchmark**| Single Operation Duration | **51852** | ns/op |
| **Process RSS** | Baseline -> Peak RAM | **7.64 MB -> 16.62 MB** | Resident Set Size |

### 🛠️ Architectural Details
- Standalone HTTP server on standard library `net/http`.
- `io.ReadAll(r.Body)` buffers full incoming payload in heap.
- Reflection-based `encoding/json.Unmarshal` into struct.
- SHA-256 enrichment and validation.
- Reflection-based `encoding/json.Marshal` serialization.

