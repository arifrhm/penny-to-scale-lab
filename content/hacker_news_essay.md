# From 41k to 114k RPS: The Anatomy of a Zero-Allocation Go Ingestion Pipeline
**By Arif R. | Penny to Scale Lab**  
*Original Article published in the Penny to Scale Lab Technical Series.*

---

## Abstract
In cloud-native architectures, the default response to high tail latencies and ingestion bottlenecks is horizontal scaling: provisioning additional container instances or upgrading virtual machine sizes. However, when an I/O-bound microservice stalls, the root cause is rarely insufficient compute capacity; rather, it is memory churn, GC pause overhead, and runtime reflection bottlenecks in application runtimes.

In this paper, we dissect the progressive re-engineering of a Go HTTP telemetry ingestion pipeline running on a single 10-vCPU Linux node. By moving systematically from standard library `net/http` to buffer pooling, SIMD/JIT JSON parsing, zero-copy HTTP primitives, and finally in-place byte slicing with stack scratch buffers, we collapsed per-request heap allocations from **10,574 Bytes/op (79 allocs)** down to **3 Bytes/op (3 allocs)** — an **allocation reduction of 99.97%**. Concurrently, sustained throughput increased from **41,966 RPS to 114,384 RPS (+172.5%)**, while 99th-percentile tail latency dropped from **19.41 ms to 6.50 ms (-66.5%)**.

All benchmarks are synthetic, reproducible, and released openly without proprietary dependencies or internal employer data.

---

## 1. Hardware & Test Rig Specifications
All benchmarks in this series were executed against identical hardware and load conditions:
* **Host Processor:** Intel(R) Xeon(R) Platinum 8173M CPU @ 2.00GHz (10 vCPUs pinned)
* **Memory:** 23 GiB DDR4 RAM
* **Operating System:** Linux 6.8.0-147-generic x86_64 (Ubuntu)
* **Runtime:** Go `go1.23.2 linux/amd64`
* **Benchmarking Client:** `bombardier v1.2.6` (125 concurrent TCP keep-alive connections, 10-second continuous pressure, ~1KB JSON payload)
* **Payload Structure:** Real-world telemetry event containing UUIDs, tenant identifiers, ISO-8601 timestamps, nested metric maps, string slices, and payload attributes.

---

## 2. The Architectural Progression

### 2.1 Baseline (v0.1.0): The Idiomatic Go Anti-Pattern
Standard Go tutorials recommend parsing incoming HTTP payloads using standard library components:

```go
// The Idiomatic Baseline (v0.1.0)
func NaiveEventHandler(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "read error", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    var req models.EventRequest
    if err := json.Unmarshal(body, &req); err != nil {
        http.Error(w, "invalid payload", http.StatusBadRequest)
        return
    }
    
    hasher := sha256.New()
    hasher.Write(body)
    digest := hex.EncodeToString(hasher.Sum(nil))
    // ...
}
```

#### Why it degrades under high concurrency:
1. **Dynamic buffer reallocation (`io.ReadAll`):** `io.ReadAll` initializes a 512-byte buffer and doubles it repeatedly. For a ~1KB payload, it creates multiple intermediate heap slices that are immediately discarded.
2. **Reflection tree traversal (`encoding/json`):** Standard `json.Unmarshal` inspects struct field tags at runtime using the `reflect` package. Every nested string and map entry causes separate heap allocations.
3. **Hasher heap escapes:** `hasher.Sum(nil)` always allocates a 32-byte slice on the heap.
4. **Result:** **10,574 B/op**, **79 allocs/op**, throughput capped at **41,966 RPS**, and p99 latency climbing to **19.41 ms**.

---

### 2.2 Buffer & Struct Recycling via `sync.Pool` (v0.2.0)
The common intermediate optimization is recycling memory buffers with `sync.Pool`:

```go
var (
    bufferPool = sync.Pool{
        New: func() any { return bytes.NewBuffer(make([]byte, 0, 2048)) },
    }
    eventPool = sync.Pool{
        New: func() any { return new(models.EventRequest) },
    }
)
```

#### The Limits of Pooling:
While heap allocations dropped by **24.1%** (to **8,020 B/op** and **71 allocs/op**), throughput only improved marginally to **43,647 RPS**. 
Why? Because `sync.Pool` only recycles the top-level container. As soon as `json.Unmarshal` encounters map fields or variable-length string slices inside the struct, it triggers individual heap allocations that bypass the pool entirely. Furthermore, clearing nested maps (`clear(m)`) causes pointer churn in the Go runtime.

---

### 2.3 SIMD & JIT Acceleration: ByteDance Sonic (v0.3.0)
To eliminate reflection overhead, we replaced `encoding/json` with ByteDance Sonic (`github.com/bytedance/sonic`), a parser leveraging JIT compilation and AVX2 SIMD instructions:

```go
// Zero-reflection SIMD deserialization
err := sonic.Unmarshal(buf.Bytes(), req)
```

Sonic processes JSON tokens by vectorizing string scanning across 256-bit SIMD registers.
* **Impact:** Throughput surged by **68.6%** to **73,610 RPS**, and microbenchmark latency fell from 51.8 µs to 13.9 µs.
* **Bottleneck:** While CPU parsing was solved, the underlying transport layer (`net/http`) was still allocating goroutines and connection buffers per client.

---

### 2.4 Low-Overhead Transport: `valyala/fasthttp` (v0.4.0)
Standard library `net/http` allocates one goroutine per connection and wraps TCP streams in standard interfaces (`io.Reader`, `http.ResponseWriter`). In v0.4.0, we replaced `net/http` with `fasthttp`:

```go
func FastHTTPHandler(ctx *fasthttp.RequestCtx) {
    rawBody := ctx.PostBody() // Zero-copy slice directly into fasthttp's TCP ring buffer
    var req models.EventRequest
    if err := sonic.Unmarshal(rawBody, &req); err != nil {
        ctx.SetStatusCode(fasthttp.StatusBadRequest)
        return
    }
    // ...
}
```

* **Impact:** Because `ctx.PostBody()` avoids copying data out of the TCP buffer, per-operation heap allocation collapsed from **9,619 B/op down to 1,854 B/op (-80.7%)**, and allocs dropped to **20 allocs/op**. Throughput rose to **83,870 RPS**.

---

### 2.5 The Zero-Allocation Breakthrough: In-Place Byte Slicing (v0.5.0)
Despite reaching 83k RPS, 1,854 B/op and 20 allocations remained. In an ingestion engine, *why deserialize the entire JSON tree into a typed Go struct when only a subset of fields (`event_id`, `tenant_id`, `timestamp`) are required for routing and validation?*

In v0.5.0, we re-architected the pipeline around three core zero-allocation techniques:

#### 1. In-Place Byte Slicing via `buger/jsonparser`:
Instead of unpacking the document into Go heap memory, `jsonparser` scans the raw byte buffer using pointer arithmetic and returns byte slices pointing directly into the original `fasthttp` connection buffer:

```go
eventID, err := jsonparser.GetUnsafeString(rawBody, "event_id")
tenantID, err := jsonparser.GetUnsafeString(rawBody, "tenant_id")
ts, err := jsonparser.GetInt(rawBody, "timestamp")
```

#### 2. Stack-Allocated Hash Calculation:
Standard Go hashing calls `hasher.Sum(nil)`, which forces a 32-byte heap escape. We replaced this with a stack-allocated scratch array:

```go
var scratch [32]byte
digestBuf := res.hasher.Sum(scratch[:0]) // Appends to stack slice; 0 heap alloc!
```

#### 3. Concrete Pointer Pooling:
Pooling an interface type (such as `hash.Hash`) causes the Go compiler to insert `runtime.convIface`, allocating 16-24 bytes on the heap. We wrapped the hasher inside a concrete pointer struct (`type zeroAllocResources struct { hasher hash.Hash }`) to maintain static type layout.

---

## 3. Empirical Results & Evolution Matrix

The table below summarizes the architectural evolution across all 5 versions tested under identical loads:

| Release | Architecture & Primary Mechanism | Throughput (RPS) | Tail Latency p99 | Heap Alloc (B/op) | Allocs/Op | Micro Time | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **v0.1.0** | `net/http` + `encoding/json` + `io.ReadAll` | 41,966 | 19.41 ms | 10,574 B | 79 | 51,852 ns | Released |
| **v0.2.0** | Buffer & Struct Recycling (`sync.Pool`) | 43,647 | 16.64 ms | 8,020 B | 71 | 47,582 ns | Released |
| **v0.3.0** | AVX2 SIMD JSON Parsing (`bytedance/sonic`) | 73,610 | 10.29 ms | 9,619 B | 40 | 13,941 ns | Released |
| **v0.4.0** | Zero-Copy Engine (`valyala/fasthttp` + Sonic) | 83,870 | 10.76 ms | 1,854 B | 20 | 11,527 ns | Released |
| **v0.5.0** | In-Place Slicing (`fasthttp` + `jsonparser`) | **114,384** | **6.50 ms** | **3 B** | **3** | **3,353 ns** | Released |

### Key Takeaways:
1. **Throughput scaled by +172.5%** (from 41.9k to 114.3k requests/sec) on identical hardware.
2. **Heap memory allocation dropped by -99.97%** (from 10.5 KB to 3 Bytes per operation).
3. **99th-percentile tail latency dropped by -66.5%** (from 19.41 ms to 6.50 ms), completely eliminating latency spikes caused by Garbage Collection stop-the-world pauses.
4. **Microbenchmark execution improved by 15.4x** (from 51.8 µs to 3.3 µs per request).

---

## 4. Reproducibility & Open Source
All code, harness scripts, and benchmarks are committed under an open MIT license:
* **Repository:** `github.com/arifrhm/penny-to-scale-lab`
* **Benchmarking Command:**
  ```bash
  git clone https://github.com/arifrhm/penny-to-scale-lab.git
  cd penny-to-scale-lab
  ./scripts/bench.sh --mode zeroalloc
  ```

---

## 5. Conclusion
Software engineering efficiency is not an aesthetic pursuit; in high-scale infrastructure, it represents the boundary between needing a \$10,000/month multi-node Kubernetes cluster and running seamlessly on a \$50/month dedicated node. 

Before requesting more compute quota from cloud providers, measure your runtime allocations. Eliminating reflection, adopting zero-copy transport buffers, and using in-place byte slicing can deliver an order-of-magnitude leap in throughput at zero additional infrastructure cost.
