# 🧵 Viral Engineering Thread (X / Twitter & LinkedIn)
## Zero-Allocation Go Ingestion: From 41k to 114k RPS (10.5 KB to 3 Bytes/op)

*Bilingual Content Kit: English Primary (Global Reach) + Indonesian Translation (Local Community).*  
*Format: Optimized for high engagement, technical credibility, zero marketing fluff, and verifiable receipts.*

---

### 🐦 Tweet 1: The Hook & The Problem / Pengait & Masalah Utama
**[EN]**  
Most teams respond to high cloud latency by throwing more AWS/GCP vCPUs at the problem.  

We did the opposite: we took a Go telemetry ingestion service on a single Linux node from **41,966 RPS (10,574 B/op)** to **114,384 RPS (3 B/op)** — cutting tail latency p99 by **66.5%** without spending an extra dollar on infrastructure.  

Here is the step-by-step breakdown of how Go allocates memory under load, and how we eliminated it. 🧵👇  

**[ID]**  
Banyak tim engineering mengatasi latensi dan overload server dengan langsung upgrade spec vCPU di AWS/GCP.  

Kami melakukan hal sebaliknya: mengoptimasi HTTP ingestion service Go di satu mesin Linux dari **41.966 RPS (10.574 B/op)** menjadi **114.384 RPS (3 B/op)** — memangkas tail latency p99 sebesar **66,5%** tanpa nambah biaya sewa server sepeser pun.  

Berikut bedah teknis lengkap cara Go mengalokasikan memori dan cara kami menghabisi alokasi tersebut hingga mendekati 0 B/op. 🧵👇  

---

### 🐦 Tweet 2: The Naive Baseline Trap (v0.1.0) / Jebakan Baseline Naif
**[EN]**  
Version 0.1.0: Standard library `net/http` + `io.ReadAll(r.Body)` + `encoding/json.Unmarshal`.  

The problem? At 100k requests, this naive approach burns:  
- **10,574 Bytes/op** across **79 heap allocations per request**.  
- `io.ReadAll` allocates dynamic expanding byte slices on the heap.  
- `encoding/json` walks the reflection tree (`reflect.Type`), allocating new strings and map entries on every payload.  

Result: 41,966 RPS, p99 = 19.41 ms, and the Go garbage collector (GC) spending excessive cycles cleaning up short-lived objects.  

**[ID]**  
Versi 0.1.0: Standard library `net/http` + `io.ReadAll(r.Body)` + `encoding/json.Unmarshal`.  

Masalahnya? Pada beban puluhan ribu request, pendekatan standar ini menguras:  
- **10.574 Bytes/op** di **79 alokasi heap per request**.  
- `io.ReadAll` membuat slice byte dinamis di heap berulang kali.  
- `encoding/json` melakukan inspeksi refleksi tipe runtime, mengalokasikan string baru dan map entry untuk setiap payload.  

Hasil: 41.966 RPS, p99 = 19,41 ms, dan Go GC (Garbage Collector) terbebani membersihkan jutaan objek memori sesaat.  

---

### 🐦 Tweet 3: The `sync.Pool` Illusion (v0.2.0) / Ilusi sync.Pool
**[EN]**  
Version 0.2.0: We added `sync.Pool` for byte buffers and struct recycling.  

Did it help?  
- Memory dropped by 24.1% (10,574 B/op ➡️ 8,020 B/op).  
- Throughput only crawled up slightly: 41.9k ➡️ 43.6k RPS.  

Why didn't `sync.Pool` solve it completely?  
Because `encoding/json` still allocates inside unexported struct fields, and returning dynamic maps to pools requires clearing them, causing memory fragmentation. Pooling alone cannot fix an allocation-heavy parser.  

**[ID]**  
Versi 0.2.0: Kami memasang `sync.Pool` untuk mendaur ulang buffer byte dan struct event.  

Apakah langsung selesai?  
- Memori memang turun 24,1% (10.574 B/op ➡️ 8.020 B/op).  
- Tapi throughput hanya naik tipis: 41,9k ➡️ 43,6k RPS.  

Kenapa `sync.Pool` bukan peluru perak?  
Karena `encoding/json` tetap mengalokasikan memori di balik layar, dan mereset slice/map dinamis tetap menyisakan fragmentasi memori. Pool tidak bisa menyembuhkan parser yang kodratnya boros alokasi.  

---

### 🐦 Tweet 4: Enter SIMD & Zero-Copy HTTP (v0.3.0 & v0.4.0) / SIMD & Engine fasthttp
**[EN]**  
Two major architectural shifts:  
1. **v0.3.0 (ByteDance Sonic):** Switched to JIT/SIMD JSON parsing with AVX2 instructions. Throughput jumped immediately to **73,610 RPS** (+68%).  
2. **v0.4.0 (`valyala/fasthttp`):** Replaced `net/http`'s goroutine-per-connection model. Read raw payloads via zero-copy `ctx.PostBody()` directly from TCP socket buffers.  

Throughput surged to **83,870 RPS**, and heap memory fell to **1,854 B/op** (-82.5% vs baseline). But we still had 20 heap allocs per request.  

Can we achieve near **0 B/op**?  

**[ID]**  
Dua lompatan arsitektur berikutnya:  
1. **v0.3.0 (ByteDance Sonic):** Beralih ke JSON parser berbasis JIT/SIMD instruksi hardware AVX2. Throughput langsung melonjak ke **73.610 RPS** (+68%).  
2. **v0.4.0 (`valyala/fasthttp`):** Mengganti model goroutine-per-conn `net/http`. Membaca raw payload via zero-copy `ctx.PostBody()` langsung dari buffer socket TCP.  

Throughput melesat ke **83.870 RPS**, dan alokasi heap rontok ke **1.854 B/op** (-82,5% vs baseline). Tapi masih tersisa 20 alokasi heap per request.  

Bisakah kita tembus mendekati **0 B/op** mutlak?  

---

### 🐦 Tweet 5: The Zero-Alloc In-Place Slicing Breakthrough (v0.5.0) / Terobosan Zero-Alloc
**[EN]**  
Version 0.5.0: We asked: *Why deserialize JSON into Go structs at all if we only need specific fields for ingestion & routing?*  

The Zero-Alloc Architecture:  
- Use `buger/jsonparser` to slice bytes in-place directly from the raw TCP buffer without allocating Go strings or structs.  
- Calculate SHA-256 validation digests using stack scratch arrays: `hasher.Sum(scratch[:0])` — zero heap escaping!  
- Concrete pointer pooling for hashers to avoid Go runtime interface boxing allocations (`CONVIFACE`).  

The result? Per-op heap allocations collapsed from **10,574 B/op to 3 B/op** (99.97% drop!).  

**[ID]**  
Versi 0.5.0: Kami bertanya: *Kenapa harus parsing JSON ke struct Go kalau kita cuma butuh validasi dan routing field tertentu?*  

Arsitektur Zero-Alloc:  
- Menggunakan `buger/jsonparser` untuk memotong byte slice langsung di tempat dari TCP buffer tanpa konversi string atau struct heap.  
- Menghitung digest SHA-256 menggunakan buffer stack lokal: `hasher.Sum(scratch[:0])` — nol heap escape!  
- Menggunakan pointer konkrit pada pool hasher untuk mencegah alokasi interface boxing Go runtime (`CONVIFACE`).  

Hasilnya? Alokasi heap per request ambruk dari **10.574 B/op menjadi 3 B/op** (penurunan 99,97%!).  

---

### 🐦 Tweet 6: The Verifiable Receipts / Bukti Empiris Benchmark
**[EN]**  
Tested on a single 10-vCPU Intel Xeon node under continuous concurrency (bombardier, 125 conns, 10s, 1KB payload):  

📊 **Evolution from v0.1.0 ➡️ v0.5.0:**  
• Throughput: **41,966 RPS ➡️ 114,384 RPS** (+172.5%)  
• Tail Latency p99: **19.41 ms ➡️ 6.50 ms** (-66.5%)  
• Tail Latency p90: **10.15 ms ➡️ 2.00 ms** (-80.3%)  
• Heap Allocation: **10,574 B/op ➡️ 3 B/op** (-99.97%)  
• Allocs per op: **79 allocs ➡️ 3 allocs** (-96.2%)  
• Microbench Speed: **51.8 µs ➡️ 3.3 µs** (15x faster)  

Zero marketing hype. Every number is reproducible.  

**[ID]**  
Diuji pada satu node 10-vCPU Intel Xeon di bawah beban konkurensi stabil (bombardier, 125 conns, 10s, 1KB payload):  

📊 **Evolusi dari v0.1.0 ➡️ v0.5.0:**  
• Throughput: **41.966 RPS ➡️ 114.384 RPS** (+172,5%)  
• Tail Latency p99: **19,41 ms ➡️ 6,50 ms** (-66,5%)  
• Tail Latency p90: **10,15 ms ➡️ 2,00 ms** (-80,3%)  
• Alokasi Heap: **10.574 B/op ➡️ 3 B/op** (-99,97%)  
• Allocs per op: **79 allocs ➡️ 3 allocs** (-96,2%)  
• Durasi Mikro: **51,8 µs ➡️ 3,3 µs** (15x lebih cepat)  

Tanpa klaim kosong. Semua angka terverifikasi dan dapat direproduksi di komputer lokal.  

---

### 🐦 Tweet 7: Open Source Lab & What's Next / Repositori Terbuka
**[EN]**  
I built this as part of the **"Penny to Scale Lab"** — an open-source experiment documenting high-performance systems engineering built entirely on verifiable benchmarks and synthetic data (zero internal NDA violations).  

The full repo, automated benchmark runner (`bench.sh`), and bilingual changelog are open source:  
🔗 GitHub: `github.com/arifrhm/penny-to-scale-lab`  

Next up: Kernel-bypass networking via `ebpf` and DPDK packet rings.  

If you care about high-throughput systems and performance engineering, give this a RT and star the repo! ⭐  

**[ID]**  
Saya membangun lab ini sebagai bagian dari eksperimen terbuka **"Penny to Scale Lab"** — eksplorasi rekayasa sistem performa tinggi berbasis bukti empiris dan 100% data sintetis (tanpa melanggar NDA internal perusahaan mana pun).  

Seluruh kode, script benchmark otomatis (`bench.sh`), dan changelog dwibahasa terbuka untuk umum:  
🔗 GitHub: `github.com/arifrhm/penny-to-scale-lab`  

Langkah berikutnya: Eksplorasi kernel-bypass networking via `ebpf` & DPDK packet rings.  

Jika Anda menyukai rekayasa sistem berkinerja tinggi, silakan retweet/share dan beri bintang di repo! ⭐  
