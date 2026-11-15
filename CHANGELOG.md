# Changelog - Penny to Scale Lab

All benchmarks are executed automatically with real hardware resource telemetry attached to each architectural release.  
*Semua benchmark dijalankan secara otomatis dengan data resource hardware riil terlampir pada setiap rilis arsitektur.*

---

## [0.1.0] - 2026-11-15 - Naive Baseline (net/http + encoding/json)

### 💻 System & Environment Telemetry / Telemetri Lingkungan Sistem
- **Host CPU:** Intel(R) Xeon(R) Platinum 8173M CPU @ 2.00GHz (10 vCPUs)
- **Memory (RAM):** 23Gi Total
- **Operating System / Sistem Operasi:** Linux 6.8.0-147-generic x86_64
- **Runtime:** `go1.23.2 linux/amd64`
- **Load Tool / Alat Uji:** `bombardier v1.2.6` (125 concurrent connections, 10s duration, ~1KB JSON payload)

### 📊 Benchmark Results / Hasil Pengujian Benchmark
| Metric Category / Kategori | Metric Name / Nama Metrik | Measured Value / Nilai Riil | Unit & Notes / Satuan & Catatan |
| :--- | :--- | :--- | :--- |
| **Throughput** | Mean Request Rate | **41,966.72** | requests/sec (RPS) |
| **Total Volume** | Successful 2xx Requests | **419,663** | requests in 10s (0 errors / 100% 2xx) |
| **Tail Latency** | Median (p50) | **0.80** | milliseconds (ms) |
| **Tail Latency** | 90th percentile (p90) | **10.15** | milliseconds (ms) |
| **Tail Latency** | 99th percentile (p99) | **19.41** | milliseconds (ms) |
| **Memory Alloc** | Per-operation Heap Alloc | **10,574** | Bytes/op (`B/op` reflection overhead) |
| **Memory Alloc** | Per-operation Allocs Count | **79** | allocs/op (heap allocations per request) |
| **Microbenchmark**| Single Operation Duration | **51,852** | ns/op (~51.8 µs per op) |
| **Process RSS** | Baseline -> Peak RAM | **7.64 MB -> 16.62 MB** | Resident Set Size |

### 🛠️ Architectural Details / Rincian Arsitektur
- **English:**
  - Standalone HTTP server on standard library `net/http`.
  - `io.ReadAll(r.Body)` buffers the full incoming payload in the heap for every single request.
  - Reflection-based `encoding/json.Unmarshal` into struct.
  - SHA-256 enrichment and input validation.
  - Reflection-based `encoding/json.Marshal` serialization.
- **Bahasa Indonesia:**
  - Server HTTP mandiri berbasis library standar `net/http`.
  - `io.ReadAll(r.Body)` mengalokasikan slice baru di heap untuk membaca seluruh body pada setiap request.
  - Parsing JSON berbasis refleksi menggunakan `encoding/json.Unmarshal`.
  - Pengayaan (*enrichment*) checksum SHA-256 dan validasi field.
  - Serialisasi JSON balasan menggunakan `encoding/json.Marshal` standar.
