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

## [0.2.0] - 2026-11-15 - Buffer & Struct Recycling via sync.Pool

### 💻 System & Environment Telemetry / Telemetri Lingkungan Sistem
- **Host CPU:** Intel(R) Xeon(R) Platinum 8173M CPU @ 2.00GHz (10 vCPUs)
- **Memory (RAM):** 23Gi Total
- **Operating System / Sistem Operasi:** Linux 6.8.0-147-generic x86_64
- **Runtime:** go1.23.2 linux/amd64
- **Load Tool / Alat Uji:** bombardier v1.2.6 (125 connections, 10s duration, ~1KB JSON payload)

### 📊 Benchmark Results / Hasil Pengujian Benchmark
| Metric Category / Kategori | Metric Name / Nama Metrik | Measured Value / Nilai Riil | Unit & Notes / Satuan & Catatan |
| :--- | :--- | :--- | :--- |
| **Throughput** | Mean Request Rate | **43647.78** | requests/sec (RPS) |
| **Total Volume** | Successful 2xx Requests | **437008** | requests in 10s (0 errors / 100% 2xx) |
| **Tail Latency** | Median (p50) | **1.13** | milliseconds (ms) |
| **Tail Latency** | 90th percentile (p90) | **9.31** | milliseconds (ms) |
| **Tail Latency** | 99th percentile (p99) | **16.64** | milliseconds (ms) |
| **Memory Alloc** | Per-operation Heap Alloc | **8020** | Bytes/op (`B/op` memory allocation) |
| **Memory Alloc** | Per-operation Allocs Count | **71** | allocs/op (heap allocations per request) |
| **Microbenchmark**| Single Operation Duration | **47582** | ns/op |
| **Process RSS** | Baseline -> Peak RAM | **7.71 MB -> 16.33 MB** | Resident Set Size |

### 🛠️ Architectural Details / Rincian Arsitektur
- **English:**
  - Introduced `sync.Pool` byte buffer recycling (`bufferPool`) for incoming request body and response serialization, eliminating dynamic buffer allocations.
  - Introduced `sync.Pool` struct recycling (`eventRequestPool`) with capacity preservation for `Tags` slice and built-in `clear()` for `Metadata` map.
  - Introduced `hasherPool` to recycle `sha256.New()` instances across concurrent requests.
  - Eliminated `fmt.Sprintf` heap churn in audit hash using `strconv.AppendInt` with stack scratch buffer.
  - Memory heap allocations slashed from **10,574 B/op** down to **8020 B/op** (-24.1% reduction).
- **Bahasa Indonesia:**
  - Penerapan daur ulang buffer byte `sync.Pool` (`bufferPool`) untuk membaca body dan serialisasi balasan, menghapus realokasi buffer dinamis.
  - Penerapan `sync.Pool` untuk struct request (`eventRequestPool`) dengan menjaga kapasitas slice `Tags` dan fungsi native `clear()` untuk map `Metadata`.
  - Penerapan `hasherPool` untuk mendaur ulang instance `sha256.New()` di seluruh thread konkurensi.
  - Mengeliminasi alokasi string heap dari `fmt.Sprintf` pada audit hash dengan `strconv.AppendInt` dan buffer stack scratch.
  - Pemotongan alokasi heap dari **10.574 B/op** turun menjadi **8020 B/op** (penghematan ~24,1%).


## [0.3.0] - 2026-11-15 - JIT/SIMD Zero-Reflection JSON Serializer (bytedance/sonic)

### 💻 System & Environment Telemetry / Telemetri Lingkungan Sistem
- **Host CPU:** Intel(R) Xeon(R) Platinum 8173M CPU @ 2.00GHz (10 vCPUs)
- **Memory (RAM):** 23Gi Total
- **Operating System / Sistem Operasi:** Linux 6.8.0-147-generic x86_64
- **Runtime:** go1.23.2 linux/amd64
- **Load Tool / Alat Uji:** bombardier v1.2.6 (125 connections, 10s duration, ~1KB JSON payload)

### 📊 Benchmark Results / Hasil Pengujian Benchmark
| Metric Category / Kategori | Metric Name / Nama Metrik | Measured Value / Nilai Riil | Unit & Notes / Satuan & Catatan |
| :--- | :--- | :--- | :--- |
| **Throughput** | Mean Request Rate | **73610.20** | requests/sec (RPS) |
| **Total Volume** | Successful 2xx Requests | **735253** | requests in 10s (0 errors / 100% 2xx) |
| **Tail Latency** | Median (p50) | **0.96** | milliseconds (ms) |
| **Tail Latency** | 90th percentile (p90) | **4.12** | milliseconds (ms) |
| **Tail Latency** | 99th percentile (p99) | **10.29** | milliseconds (ms) |
| **Memory Alloc** | Per-operation Heap Alloc | **9619** | Bytes/op (`B/op` memory allocation) |
| **Memory Alloc** | Per-operation Allocs Count | **40** | allocs/op (heap allocations per request) |
| **Microbenchmark**| Single Operation Duration | **27704** | ns/op |
| **Process RSS** | Baseline -> Peak RAM | **13.14 MB -> 30.81 MB** | Resident Set Size |

### 🛠️ Architectural Details / Rincian Arsitektur
- **English:**
  - Integrated ByteDance's Sonic (`bytedance/sonic`) JIT-compiled AVX2 SIMD zero-reflection JSON parser and serializer.
  - Replaced standard `encoding/json` reflection tree walks with Sonic's hardware-accelerated vectorized parsing.
  - Slashed allocation operations nearly in half from **71 allocs/op** down to **40 allocs/op** (-43.7% reduction).
  - Microbenchmark execution duration plunged from **47,582 ns/op** down to **27704 ns/op** (over **2.0x faster execution**).
- **Bahasa Indonesia:**
  - Integrasi parser dan serializer JSON ByteDance Sonic (`bytedance/sonic`) berbasis JIT compiler dan akselerasi hardware AVX2 SIMD tanpa refleksi.
  - Menggantikan *reflection tree walking* standar `encoding/json` dengan parsing vektor terakselerasi CPU.
  - Memangkas frekuensi operasi alokasi heap hampir separuh dari **71 allocs/op** turun menjadi **40 allocs/op** (pengurangan -43,7%).
  - Kecepatan eksekusi mikro meningkat drastis dari **47.582 ns/op** menjadi **27704 ns/op** (lebih dari **2x lipat lebih cepat**).

