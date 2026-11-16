#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "================================================================"
echo "🛡️  [Penny to Scale Lab] Local CI & Performance Gatekeeper"
echo "================================================================"

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# -------------------------------------------------------------
# 1. Static Analysis & Compilation Check
# -------------------------------------------------------------
echo -e "\n${BLUE}🔍 [Stage 1/4] Code Formatting, Vet & Compile Check...${NC}"
cd "${ROOT_DIR}"

UNFORMATTED=$(gofmt -l . | grep -v "^vendor" || true)
if [[ -n "${UNFORMATTED}" ]]; then
    echo -e "${RED}❌ Unformatted Go files detected:${NC}"
    echo "${UNFORMATTED}"
    echo "Run 'gofmt -w .' to fix."
    exit 1
fi
echo -e "${GREEN}✅ Formatting: All Go files properly formatted.${NC}"

go vet ./...
echo -e "${GREEN}✅ Go Vet: Static analysis clean (no issues detected).${NC}"

go build -o /dev/null ./cmd/server/...
echo -e "${GREEN}✅ Build Check: Binary compiles cleanly.${NC}"

# -------------------------------------------------------------
# 2. Race Detection & Functional Unit Tests
# -------------------------------------------------------------
echo -e "\n${BLUE}🧪 [Stage 2/4] Concurrency Race Detector & Unit Tests...${NC}"
go test -v -race -timeout=30s ./internal/...
echo -e "${GREEN}✅ Unit Tests & Race Detection: 100% PASS with zero data races.${NC}"

# -------------------------------------------------------------
# 3. Zero-Allocation Regression Guard (Strict Performance Contract)
# -------------------------------------------------------------
echo -e "\n${BLUE}⚡ [Stage 3/4] Zero-Allocation Regression Guard (Strict 0 B/op contract)...${NC}"

TARGET_BENCHMARKS=(
    "BenchmarkZeroAllocHandler_Serve"
    "BenchmarkRingBufferHandler_Serve"
)

REGRESSION_FAILED=0

for BENCH in "${TARGET_BENCHMARKS[@]}"; do
    echo -e "🔬 Verifying allocation contract for ${YELLOW}${BENCH}${NC}..."
    BENCH_LOG=$(go test -bench="^${BENCH}$" -benchmem -benchtime=2s ./internal/handler/...)
    
    # Extract metrics
    LINE=$(echo "${BENCH_LOG}" | grep "^${BENCH}" | head -n1 || true)
    if [[ -z "${LINE}" ]]; then
        echo -e "${RED}❌ Benchmark ${BENCH} failed to execute!${NC}"
        echo "${BENCH_LOG}"
        exit 1
    fi
    
    B_OP=$(echo "${LINE}" | awk '{print $5}')
    ALLOCS_OP=$(echo "${LINE}" | awk '{print $7}')
    NS_OP=$(echo "${LINE}" | awk '{print $3}')
    
    echo "   Result: ${B_OP} B/op, ${ALLOCS_OP} allocs/op (${NS_OP} ns/op)"
    
    # Zero allocation contract: MUST be strictly 0 B/op and 0 allocs/op
    if [[ "${B_OP}" != "0" ]] || [[ "${ALLOCS_OP}" != "0" ]]; then
        echo -e "${RED}❌ REGRESSION BREACH: ${BENCH} produced ${B_OP} B/op and ${ALLOCS_OP} allocs/op!${NC}"
        echo -e "${RED}   Zero-allocation contract violated. Only 0 B/op and 0 allocs/op are permitted.${NC}"
        REGRESSION_FAILED=1
    else
        echo -e "${GREEN}✅ CONTRACT HONORED: ${BENCH} verified at 0 B/op & 0 allocs/op.${NC}"
    fi
done

if [[ "${REGRESSION_FAILED}" -ne 0 ]]; then
    echo -e "\n${RED}🛑 CI FAILED: Zero-allocation performance regression detected!${NC}"
    exit 1
fi

# -------------------------------------------------------------
# 4. Smoke Test Local Server Lifecycle & Routing
# -------------------------------------------------------------
echo -e "\n${BLUE}🚀 [Stage 4/4] Smoke Testing Server Lifecycle & HTTP Contracts...${NC}"
TEST_PORT="8999"
go build -o "${ROOT_DIR}/bin/ci_server" cmd/server/main.go

# Test in ringbuffer mode
HANDLER_MODE="ringbuffer" PORT="${TEST_PORT}" "${ROOT_DIR}/bin/ci_server" > /tmp/ci_server.log 2>&1 &
SERVER_PID=$!

cleanup() {
    if kill -0 "${SERVER_PID}" 2>/dev/null; then
        kill -TERM "${SERVER_PID}" 2>/dev/null || true
        wait "${SERVER_PID}" 2>/dev/null || true
    fi
    rm -f "${ROOT_DIR}/bin/ci_server"
}
trap cleanup EXIT

# Wait for server
HEALTH_URL="http://127.0.0.1:${TEST_PORT}/health"
READY=0
for i in {1..20}; do
    if curl -s "${HEALTH_URL}" | grep -q "HEALTHY"; then
        READY=1
        break
    fi
    sleep 0.1
done

if [[ "${READY}" -ne 1 ]]; then
    echo -e "${RED}❌ Server failed to respond on ${HEALTH_URL}! Log:${NC}"
    cat /tmp/ci_server.log
    exit 1
fi
echo -e "${GREEN}✅ Health Check: Server started and responded HEALTHY on port ${TEST_PORT}.${NC}"

# Send sample event payload
PAYLOAD_FILE="${ROOT_DIR}/scripts/payload_sample.json"
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "http://127.0.0.1:${TEST_PORT}/events" \
    -H "Content-Type: application/json" \
    -d @"${PAYLOAD_FILE}")

HTTP_STATUS=$(echo "${RESP}" | grep "HTTP_STATUS" | cut -d':' -f2)
BODY=$(echo "${RESP}" | grep -v "HTTP_STATUS")

if [[ "${HTTP_STATUS}" != "200" ]]; then
    echo -e "${RED}❌ Ingestion test failed! Status: ${HTTP_STATUS}, Body: ${BODY}${NC}"
    exit 1
fi

# Validate JSON response fields
if ! echo "${BODY}" | grep -q '"status":"QUEUED"' || ! echo "${BODY}" | grep -q '"sequence"'; then
    echo -e "${RED}❌ Malformed response payload: ${BODY}${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Event Ingestion: Payload successfully ingested and queued (HTTP 200 OK).${NC}"

cleanup
trap - EXIT

echo -e "\n================================================================"
echo -e "${GREEN}🎉 ALL LOCAL CI CHECKS & PERFORMANCE CONTRACTS PASSED!${NC}"
echo -e "================================================================\n"
