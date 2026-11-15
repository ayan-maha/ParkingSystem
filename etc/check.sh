
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "== Smart Parking: build and verification =="
make build

echo
echo "== Starting two parking servers =="

./bin/server -id n1 -http :18080 -grpc :15051 \
  >/tmp/parking-n1.log 2>&1 &
P1=$!

./bin/server -id n2 -http :18081 -grpc :15052 \
  -target localhost:15051 \
  >/tmp/parking-n2.log 2>&1 &
P2=$!

cleanup() {
    kill "$P1" "$P2" 2>/dev/null || true
}
trap cleanup EXIT

echo "Waiting for servers..."

ready=0
for i in $(seq 1 30); do
    if curl -fsS http://localhost:18080/v1/spaces >/dev/null 2>&1 &&
       curl -fsS http://localhost:18081/v1/spaces >/dev/null 2>&1; then
        ready=1
        break
    fi
    sleep 0.2
done

if [[ "$ready" != 1 ]]; then
    echo "ERROR: Servers did not start."
    exit 1
fi

echo
echo "== Running Smart Parking REST tests =="

# The demo normally uses 8080 and 8081.
# Use different ports here to avoid interfering with
# manually started development servers.

sed \
  -e 's/localhost:8080/localhost:18080/g' \
  -e 's/localhost:8081/localhost:18081/g' \
  etc/demo.sh | bash

echo
echo "== Verifying cross-server gRPC forwarding =="

response=$(curl -fsS -X POST \
  -H "Content-Type: application/json" \
  -d '{"licensePlate":"1234567","hasAccessiblePermit":true,"spaceId":"P15"}' \
  http://localhost:18081/v1/reservations)

if [[ "$response" != *'"reservationId":"R1001-n1"'* ]]; then
    echo "FAIL: Server 2 did not return Server 1's node identifier."
    exit 1
fi

echo "PASS: Server 2 forwarded the request to Server 1."
echo
echo "ALL CHECKS PASSED"
