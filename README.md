# ⚡ Penny to Scale Lab (`penny-to-scale-lab`)

> **"Breaking systems on a cheap VPS so you don't break them in production."**  
> *Mendobrak sistem di VPS murah agar Anda tidak merusaknya di production.*

---

## 🌍 Language / Bahasa
- [English](#-english)
- [Bahasa Indonesia](#-bahasa-indonesia)

---

## 🇬🇧 English

### 🎯 Manifesto & Lab Rules
1. **Empirical Over Opinions:** Zero speculation without hard numbers. Every optimization claim must be substantiated by **Requests Per Second (RPS)**, **p50/p90/p99 tail latencies**, and **Bytes/op (heap allocations)**.
2. **Zero-Production Leak (100% Synthetic & Open):** All test suites operate strictly on synthetic mock data (~1KB telemetry/transaction payload) running on independent lab hardware. Zero confidential employer data, 100% NDA compliant.
3. **Continuous Benchmark Changelog:** Every commit or architectural refactor modifying request execution **MUST** record a benchmark matrix and hardware resource telemetry directly in [`CHANGELOG.md`](./CHANGELOG.md).

### 💻 Hardware & Environment Benchmark Lab

| Resource | Specification |
| :--- | :--- |
| **CPU** | Intel(R) Xeon(R) Platinum 8173M CPU @ 2.00GHz (10 vCPUs) |
| **Memory (RAM)** | 23 GiB Total |
| **Operating System** | Linux 6.8.0-147-generic x86_64 (Ubuntu) |
| **Runtime** | `go1.23.2 linux/amd64` |
| **Load Tool** | `bombardier v1.2.6` (125 concurrent connections, 10s duration) |
| **Target Endpoint** | `POST /events` (~1KB JSON Event Ingestion Payload) |

### 📈 Evolution Matrix Across Versions

| Version | Architecture Description | Throughput (RPS) | Latency p99 | Heap Alloc (B/op) | Allocs/Op | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **v0.1.0** | `net/http` + `encoding/json` + `io.ReadAll` (Naive) | **41,966 req/s** | **19.41 ms** | **10,574 B/op** | **79 allocs/op** | 🟢 **Released** |
| **v0.2.0** | Buffer & Struct Recycling via `sync.Pool` | **43,647 req/s** | **16.64 ms** | **8,020 B/op** | **71 allocs/op** | 🟢 **Released** |
| *v0.3.0* | Zero-Reflection JSON Serializer (`bytedance/sonic` / `easyjson`) | *TBD* | *TBD* | *TBD* | *TBD* | ⚪ *Planned* |
| *v0.4.0* | High-Performance Engine (`valyala/fasthttp` + SIMD) | *TBD* | *TBD* | *TBD* | *TBD* | ⚪ *Planned* |

---

## 🇮🇩 Bahasa Indonesia

### 🎯 Manifesto & Aturan Lab
1. **Empiris di Atas Opini:** Tidak ada debat kusir tanpa angka riil. Setiap klaim optimasi wajib dibuktikan dengan data **Requests Per Second (RPS)**, **latensi p50/p90/p99**, dan **Bytes/op (alokasi memori heap)**.
2. **Nol Kebocoran Data Produksi (100% Sintetis & Terbuka):** Semua pengujian menggunakan data sintetis (~1KB payload telemetri/transaksi) dan dijalankan di mesin lab pribadi. Nol data rahasia perusahaan, 100% mematuhi etika kerahasiaan NDA.
3. **Continuous Benchmark Changelog:** Setiap commit atau perubahan arsitektur yang mengubah cara server memproses request **WAJIB** melampirkan tabel benchmark dan spesifikasi resource perangkat keras di [`CHANGELOG.md`](./CHANGELOG.md).

### 📈 Ringkasan Metrik Lintas Versi
Lihat tabel perbandingan performa pada seksi [Evolution Matrix](#-evolution-matrix-across-versions) di atas. Log pengujian mendalam setiap rilis tercatat lengkap di dokumen [`CHANGELOG.md`](./CHANGELOG.md).

---

## 🚀 How to Run & Reproduce / Cara Menjalankan

### 1. Prerequisites / Prasyarat
* [Go 1.23+](https://golang.org)
* [Bombardier](https://github.com/codesenberg/bombardier) (`go install github.com/codesenberg/bombardier@latest`)
* `jq` and `curl`

### 2. Run Local Server / Menjalankan Server Lokal
```bash
# Build and run server on port 8080 / Build dan jalankan server pada port 8080
make run
```

Health check:
```bash
curl http://127.0.0.1:8080/health
# Response: {"status":"HEALTHY","version":"0.1.0-naive"}
```

Send manual test event / Kirim event pengujian manual:
```bash
curl -X POST http://127.0.0.1:8080/events \
  -H "Content-Type: application/json" \
  -d @scripts/payload_sample.json
```

### 3. Run Automated Benchmark / Menjalankan Otomasi Benchmark
Run automated benchmark and view results in terminal:
```bash
make bench
```

Run benchmark and auto-append metrics to `CHANGELOG.md`:
```bash
make bench-changelog VERSION=0.1.0 TITLE="Naive Baseline (net/http + encoding/json)"
```

---

## 📁 Repository Structure / Struktur Repositori
* `cmd/server/main.go`: HTTP Server entrypoint with `/events`, `/health`, and `/debug/pprof/`.
* `internal/models/`: Event request struct definition and payload validation rules.
* `internal/handler/`: Versioned handler implementations (starting from `naive.go`).
* `scripts/bench.sh`: Automated load testing, process RSS memory telemetry, and Markdown table generator.

---
*Built for open software engineering performance experimentation.*  
*Dibuat untuk eksperimen performa rekayasa perangkat lunak terbuka.*
