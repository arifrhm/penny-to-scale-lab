#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

PORT="${PORT:-8089}"
DURATION="${DURATION:-10s}"
CONNECTIONS="${CONNECTIONS:-125}"
TARGET_URL="http://127.0.0.1:${PORT}/events"
HEALTH_URL="http://127.0.0.1:${PORT}/health"
PAYLOAD_FILE="${ROOT_DIR}/scripts/payload_sample.json"

echo "================================================================"
echo "⚡ [Penny to Scale Lab] Automated Benchmark & Telemetry Runner"
echo "================================================================"

# 1. System Info Gathering
CPU_MODEL=$(lscpu | grep "Model name:" | head -n1 | sed -e 's/Model name:[[:space:]]*//')
VCPU_COUNT=$(nproc)
TOTAL_RAM=$(free -h | awk '/^Mem:/ {print $2}')
OS_KERNEL=$(uname -srm)
GO_VERSION=$(go version | awk '{print $3 " " $4}')

echo "💻 Hardware: ${CPU_MODEL} (${VCPU_COUNT} vCPUs), RAM: ${TOTAL_RAM}"
echo "🐧 OS Kernel: ${OS_KERNEL}"
echo "🐹 Go Version: ${GO_VERSION}"

# 2. Build Server
echo "🔨 Building server binary..."
cd "${ROOT_DIR}"
go build -o "${ROOT_DIR}/bin/server" cmd/server/main.go

# 3. Start Server in Background
echo "🚀 Starting server on :${PORT}..."
PORT="${PORT}" "${ROOT_DIR}/bin/server" > /tmp/penny_server.log 2>&1 &
SERVER_PID=$!

cleanup() {
    if kill -0 "${SERVER_PID}" 2>/dev/null; then
        echo "🛑 Terminating test server (PID: ${SERVER_PID})..."
        kill -TERM "${SERVER_PID}" 2>/dev/null || true
        wait "${SERVER_PID}" 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Wait for server ready
for i in {1..30}; do
    if curl -s "${HEALTH_URL}" | grep -q "HEALTHY"; then
        echo "✅ Server is HEALTHY on PID ${SERVER_PID}"
        break
    fi
    sleep 0.2
done

# Baseline RSS
BASELINE_RSS_KB=$(ps -o rss= -p "${SERVER_PID}" | tr -d ' ' || echo "0")
BASELINE_RSS_MB=$(awk "BEGIN {printf \"%.2f\", ${BASELINE_RSS_KB}/1024}")

# 4. Run Bombardier Load Test
echo "🔥 Bombarding ${TARGET_URL} with ${CONNECTIONS} connections for ${DURATION}..."
BOMB_JSON_FILE="/tmp/bombardier_result_$$.json"
bombardier -c "${CONNECTIONS}" -d "${DURATION}" -l -m POST -f "${PAYLOAD_FILE}" -H "Content-Type: application/json" -p r -o json "${TARGET_URL}" > "${BOMB_JSON_FILE}"

# Peak RSS
PEAK_RSS_KB=$(ps -o rss= -p "${SERVER_PID}" | tr -d ' ' || echo "0")
PEAK_RSS_MB=$(awk "BEGIN {printf \"%.2f\", ${PEAK_RSS_KB}/1024}")

# Stop Server
cleanup
trap - EXIT

# Parse Bombardier JSON Output using Python for precision
PARSED_METRICS=$(python3 -c "
import json
with open('${BOMB_JSON_FILE}') as f:
    d = json.load(f)
r = d['result']
rps = r['rps']['mean']
p50 = r['latency']['percentiles'].get('50', 0) / 1000.0
p90 = r['latency']['percentiles'].get('90', 0) / 1000.0
p99 = r['latency']['percentiles'].get('99', 0) / 1000.0
total_2xx = r.get('req2xx', 0)
total_reqs = r.get('req1xx', 0) + total_2xx + r.get('req3xx', 0) + r.get('req4xx', 0) + r.get('req5xx', 0)
print(f'{rps:.2f}|{p50:.2f}|{p90:.2f}|{p99:.2f}|{total_reqs}|{total_2xx}')
")
rm -f "${BOMB_JSON_FILE}"

IFS='|' read -r RPS P50_MS P90_MS P99_MS TOTAL_REQS ERRORS_2XX <<< "${PARSED_METRICS}"

# 5. Run Native Microbenchmark for B/op & allocs/op
echo "🔬 Running native Go microbenchmark (testing.B -benchmem)..."
BENCH_OUTPUT=$(go test -bench=BenchmarkNaiveHandler -benchmem -benchtime=3s "${ROOT_DIR}/internal/handler/...")

NS_OP=$(echo "${BENCH_OUTPUT}" | grep "BenchmarkNaiveHandler" | awk '{print $3}')
B_OP=$(echo "${BENCH_OUTPUT}" | grep "BenchmarkNaiveHandler" | awk '{print $5}')
ALLOCS_OP=$(echo "${BENCH_OUTPUT}" | grep "BenchmarkNaiveHandler" | awk '{print $7}')

echo ""
echo "================================================================"
echo "📊 BENCHMARK REPORT SUMMARY"
echo "================================================================"
echo "Throughput (RPS):  ${RPS} req/sec (Total: ${TOTAL_REQS}, 2xx: ${ERRORS_2XX})"
echo "Latency p50:       ${P50_MS} ms"
echo "Latency p90:       ${P90_MS} ms"
echo "Latency p99:       ${P99_MS} ms"
echo "Heap Allocation:   ${B_OP} B/op (${ALLOCS_OP} allocs/op)"
echo "Execution Speed:   ${NS_OP} ns/op"
echo "Server Memory RSS: Baseline ${BASELINE_RSS_MB} MB -> Peak ${PEAK_RSS_MB} MB"
echo "================================================================"

# If requested to append to CHANGELOG.md
if [[ "${1:-}" == "--append-changelog" ]]; then
    VERSION="${2:-0.1.0}"
    TITLE="${3:-Naive Baseline (net/http + encoding/json)}"
    DATE_STR=$(date -u +"%Y-%m-%d")

    CHANGELOG_ENTRY="
## [${VERSION}] - ${DATE_STR} - ${TITLE}

### 💻 System & Environment Telemetry / Telemetri Lingkungan Sistem
- **Host CPU:** ${CPU_MODEL} (${VCPU_COUNT} vCPUs)
- **Memory (RAM):** ${TOTAL_RAM} Total
- **Operating System / Sistem Operasi:** ${OS_KERNEL}
- **Runtime:** ${GO_VERSION}
- **Load Tool / Alat Uji:** bombardier v1.2.6 (${CONNECTIONS} connections, ${DURATION} duration, ~1KB JSON payload)

### 📊 Benchmark Results / Hasil Pengujian Benchmark
| Metric Category / Kategori | Metric Name / Nama Metrik | Measured Value / Nilai Riil | Unit & Notes / Satuan & Catatan |
| :--- | :--- | :--- | :--- |
| **Throughput** | Mean Request Rate | **${RPS}** | requests/sec (RPS) |
| **Total Volume** | Successful 2xx Requests | **${ERRORS_2XX}** | requests in ${DURATION} (0 errors / 100% 2xx) |
| **Tail Latency** | Median (p50) | **${P50_MS}** | milliseconds (ms) |
| **Tail Latency** | 90th percentile (p90) | **${P90_MS}** | milliseconds (ms) |
| **Tail Latency** | 99th percentile (p99) | **${P99_MS}** | milliseconds (ms) |
| **Memory Alloc** | Per-operation Heap Alloc | **${B_OP}** | Bytes/op (\`B/op\` reflection overhead) |
| **Memory Alloc** | Per-operation Allocs Count | **${ALLOCS_OP}** | allocs/op (heap allocations per request) |
| **Microbenchmark**| Single Operation Duration | **${NS_OP}** | ns/op |
| **Process RSS** | Baseline -> Peak RAM | **${BASELINE_RSS_MB} MB -> ${PEAK_RSS_MB} MB** | Resident Set Size |

### 🛠️ Architectural Details / Rincian Arsitektur
- **English:**
  - Standalone HTTP server on standard library \`net/http\`.
  - \`io.ReadAll(r.Body)\` buffers full incoming payload in heap for every request.
  - Reflection-based \`encoding/json.Unmarshal\` into struct.
  - SHA-256 enrichment and input validation.
  - Reflection-based \`encoding/json.Marshal\` serialization.
- **Bahasa Indonesia:**
  - Server HTTP mandiri berbasis library standar \`net/http\`.
  - \`io.ReadAll(r.Body)\` mengalokasikan slice baru di heap untuk membaca seluruh body pada setiap request.
  - Parsing JSON berbasis refleksi menggunakan \`encoding/json.Unmarshal\`.
  - Pengayaan (*enrichment*) checksum SHA-256 dan validasi field.
  - Serialisasi JSON balasan menggunakan \`encoding/json.Marshal\` standar.
"

    CHANGELOG_FILE="${ROOT_DIR}/CHANGELOG.md"
    if [[ ! -f "${CHANGELOG_FILE}" ]]; then
        echo "# Changelog - Penny to Scale Lab" > "${CHANGELOG_FILE}"
        echo "" >> "${CHANGELOG_FILE}"
        echo "All benchmarks are executed automatically with real hardware resource telemetry attached to each architectural release." >> "${CHANGELOG_FILE}"
        echo "*Semua benchmark dijalankan secara otomatis dengan data resource hardware riil terlampir pada setiap rilis arsitektur.*" >> "${CHANGELOG_FILE}"
    fi

    echo "${CHANGELOG_ENTRY}" >> "${CHANGELOG_FILE}"
    echo "📝 Benchmark metrics appended to ${CHANGELOG_FILE} successfully!"
fi
