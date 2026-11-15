# ⚡ Penny to Scale Lab (`penny-to-scale-lab`)

> **"Breaking systems on a cheap VPS so you don't break them in production."**  
> Laboratorium eksperimen mandiri untuk menguji batas throughput, latensi p99, dan alokasi memori (*heap allocations*) pada server HTTP Go, didokumentasikan secara publik dan transparan dari implementasi naif hingga skala tinggi.

---

## 🎯 Manifesto & Aturan Lab

1. **Empirical Over Opinions:** Tidak ada debat kusir tanpa angka riil. Setiap klaim optimasi wajib dibuktikan dengan data *Requests Per Second (RPS)*, latensi *p50/p90/p99*, dan *Bytes/op*.
2. **Zero-Production Leak (100% Synthetic & Open):** Semua skenario menggunakan data sintetis (~1KB telemetry/transaction payload) dan dijalankan di mesin lab pribadi. Nol data rahasia perusahaan, 100% mematuhi etika NDA.
3. **Continuous Benchmark Changelog:** Setiap *commit* atau perubahan arsitektur yang mengubah cara server memproses request **WAJIB** melampirkan tabel benchmark dan spesifikasi resource perangkat keras di [`CHANGELOG.md`](./CHANGELOG.md).

---

## 💻 Hardware & Environment Benchmark Lab

| Resource | Spesifikasi Mesin Pengujian |
| :--- | :--- |
| **CPU** | Intel(R) Xeon(R) Platinum 8173M CPU @ 2.00GHz (10 vCPUs) |
| **Memory (RAM)** | 23 GiB Total |
| **Operating System** | Linux 6.8.0-147-generic x86_64 (Ubuntu) |
| **Runtime** | `go1.23.2 linux/amd64` |
| **Load Tool** | `bombardier v1.2.6` (125 concurrent connections, 10s duration) |
| **Target Endpoint** | `POST /events` (~1KB JSON Event Payload) |

---

## 📈 Rangkuman Metrik Lintas Versi (Evolution Matrix)

| Versi | Deskripsi Arsitektur | Throughput (RPS) | Latensi p99 | Heap Alloc (B/op) | Allocs/Op | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **v0.1.0** | `net/http` + `encoding/json` + `io.ReadAll` (Naive) | **41,966 req/s** | **19.41 ms** | **10,574 B/op** | **79 allocs/op** | 🟢 **Released** |
| *v0.2.0* | Buffer Recycling via `sync.Pool` | *TBD* | *TBD* | *TBD* | *TBD* | 🟡 *In Progress* |
| *v0.3.0* | Zero-Reflection JSON Serializer (`bytedance/sonic` / `easyjson`) | *TBD* | *TBD* | *TBD* | *TBD* | ⚪ *Planned* |
| *v0.4.0* | High-Performance Engine (`valyala/fasthttp` + SIMD) | *TBD* | *TBD* | *TBD* | *TBD* | ⚪ *Planned* |

> *Detail lengkap log pengujian setiap rilis dapat dilihat pada dokumen [CHANGELOG.md](./CHANGELOG.md).*

---

## 🚀 Cara Menjalankan & Menguji Sendiri (Quick Start)

### 1. Prasyarat
* [Go 1.23+](https://golang.org)
* [Bombardier](https://github.com/codesenberg/bombardier) (`go install github.com/codesenberg/bombardier@latest`)
* `jq` dan `curl`

### 2. Menjalankan Server Lokal
```bash
# Build dan jalankan server pada port 8080
make run
```

Uji kesehatan server:
```bash
curl http://127.0.0.1:8080/health
# Response: {"status":"HEALTHY","version":"0.1.0-naive"}
```

Kirim event pengujian manual:
```bash
curl -X POST http://127.0.0.1:8080/events \
  -H "Content-Type: application/json" \
  -d @scripts/payload_sample.json
```

### 3. Menjalankan Otomasi Benchmark
Jalankan benchmark otomatis dan lihat hasilnya di terminal:
```bash
make bench
```

Jalankan benchmark dan otomatis catat ke `CHANGELOG.md`:
```bash
make bench-changelog VERSION=0.1.0 TITLE="Naive Baseline (net/http + encoding/json)"
```

---

## 📁 Struktur Kode
* `cmd/server/main.go`: Entry point server HTTP dengan endpoint `/events`, `/health`, dan `/debug/pprof/`.
* `internal/models/`: Definisi struct payload dan validasi input.
* `internal/handler/`: Implementasi handler per versi (dimulai dari `naive.go`).
* `scripts/bench.sh`: Skrip otomasi pengujian beban bombardier, rekaman memori RSS, dan pemformat tabel markdown.

---
*Dibuat untuk eksperimen performa rekayasa perangkat lunak terbuka.*
