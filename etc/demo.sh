
#!/usr/bin/env bash
set -euo pipefail

# Smart Parking System — Part I Demonstration
# Server 1: REST 8080, gRPC 50051
# Server 2: REST 8081, forwards to Server 1 gRPC

run_test() {
    local title="$1"
    local expected="$2"
    local base="$3"
    local method="$4"
    local path="$5"
    local body="${6:-}"

    echo
    echo "============================================"
    echo "$title"
    echo "Expected HTTP status: $expected"

    local response
    if [[ -n "$body" ]]; then
        response=$(curl -sS -X "$method" \
            -H "Content-Type: application/json" \
            -d "$body" \
            -w '\n%{http_code}' \
            "$base$path")
    else
        response=$(curl -sS -X "$method" \
            -w '\n%{http_code}' \
            "$base$path")
    fi

    local status="${response##*$'\n'}"
    local content="${response%$'\n'*}"

    echo "Actual HTTP status: $status"
    echo "Response: $content"

    if [[ "$status" != "$expected" ]]; then
        echo "FAIL"
        return 1
    fi

    echo "PASS"
}

run_suite() {
    local base="$1"
    local label="$2"

    echo
    echo "############################################"
    echo "Testing $label ($base)"
    echo "############################################"

    # GET — list spaces
    run_test "List parking spaces" 200 "$base" \
        GET "/v1/spaces"

    # GET — one space
    run_test "Get accessible space P15" 200 "$base" \
        GET "/v1/spaces/P15?consistency=linearizable"

    # GET — filtered list
    run_test "Filter accessible spaces" 200 "$base" \
        GET "/v1/spaces?type=ACCESSIBLE"

    # GET — missing space
    run_test "Get missing space" 404 "$base" \
        GET "/v1/spaces/missing"

    # POST — create regular space
    run_test "Create regular space" 201 "$base" \
        POST "/v1/spaces" \
        '{"spaceId":"P25","type":"REGULAR"}'

    # POST — create accessible space
    run_test "Create accessible space" 201 "$base" \
        POST "/v1/spaces" \
        '{"spaceId":"P26","type":"ACCESSIBLE"}'

    # POST — duplicate
    run_test "Create existing space" 409 "$base" \
        POST "/v1/spaces" \
        '{"spaceId":"P02","type":"REGULAR"}'

    # BATCH — create multiple parking spaces
    run_test "Create spaces batch" 201 "$base" \
        POST "/v1/spaces/batch" \
        '{"spaces":[{"spaceId":"P30","type":"REGULAR"},{"spaceId":"P31","type":"ACCESSIBLE"}]}'

    # BATCH — empty request
    run_test "Empty batch" 400 "$base" \
        POST "/v1/spaces/batch" \
        '{"spaces":[]}'

    # BATCH — duplicate IDs
    run_test "Duplicate IDs in batch" 400 "$base" \
        POST "/v1/spaces/batch" \
        '{"spaces":[{"spaceId":"P30","type":"REGULAR"},{"spaceId":"P30","type":"ACCESSIBLE"}]}'

    # BATCH — invalid type
    run_test "Invalid type in batch" 400 "$base" \
        POST "/v1/spaces/batch" \
        '{"spaces":[{"spaceId":"P30","type":"INVALID"}]}'

    # BATCH — existing parking space
    run_test "Existing space in batch" 409 "$base" \
        POST "/v1/spaces/batch" \
        '{"spaces":[{"spaceId":"P30","type":"REGULAR"},{"spaceId":"P02","type":"ACCESSIBLE"}]}'

    # PUT — update space
    run_test "Update space type" 200 "$base" \
        PUT "/v1/spaces/P01" \
        '{"type":"ACCESSIBLE"}'

    # PUT — create new space
    run_test "Create using PUT" 201 "$base" \
        PUT "/v1/spaces/P99" \
        '{"type":"REGULAR"}'

    # PUT — reserved space conflict
    run_test "Update reserved space" 409 "$base" \
        PUT "/v1/spaces/P02" \
        '{"type":"ACCESSIBLE"}'

    # PATCH — successfully update an existing space
    run_test "Patch available space" 200 "$base" \
        PATCH "/v1/spaces/P01" \
        '{"type":"ACCESSIBLE"}'

    # PATCH — nonexistent space
    run_test "Patch missing space" 404 "$base" \
        PATCH "/v1/spaces/missing" \
        '{"type":"REGULAR"}'

    # PATCH — cannot modify reserved space
    run_test "Patch reserved space" 409 "$base" \
        PATCH "/v1/spaces/P02" \
        '{"type":"ACCESSIBLE"}'

    # PATCH — invalid space type
    run_test "Patch invalid type" 400 "$base" \
        PATCH "/v1/spaces/P01" \
        '{"type":"INVALID"}'

    # DELETE — available space
    run_test "Delete available space" 204 "$base" \
        DELETE "/v1/spaces/P01"

    # DELETE — reserved space
    run_test "Delete reserved space" 409 "$base" \
        DELETE "/v1/spaces/P02"

    # DELETE — nonexistent space
    run_test "Delete missing space" 404 "$base" \
        DELETE "/v1/spaces/missing"

    # POST — manual reservation
    run_test "Manual reservation" 201 "$base" \
        POST "/v1/reservations" \
        '{"licensePlate":"1234567","hasAccessiblePermit":false,"spaceId":"P01"}'

    # POST — automatic reservation
    run_test "Automatic reservation" 201 "$base" \
        POST "/v1/reservations" \
        '{"licensePlate":"7654321","hasAccessiblePermit":false}'

    # POST — accessible reservation
    run_test "Accessible reservation with permit" 201 "$base" \
        POST "/v1/reservations" \
        '{"licensePlate":"5555555","hasAccessiblePermit":true,"spaceId":"P15"}'

    # POST — no accessible permit
    run_test "Accessible reservation without permit" 403 "$base" \
        POST "/v1/reservations" \
        '{"licensePlate":"4444444","hasAccessiblePermit":false,"spaceId":"P15"}'

    # POST — occupied space
    run_test "Reserve occupied space" 409 "$base" \
        POST "/v1/reservations" \
        '{"licensePlate":"3333333","hasAccessiblePermit":false,"spaceId":"P02"}'

    # POST — missing space
    run_test "Reserve missing space" 404 "$base" \
        POST "/v1/reservations" \
        '{"licensePlate":"2222222","hasAccessiblePermit":false,"spaceId":"missing"}'

    # GET — reservation
    run_test "Get reservation" 200 "$base" \
        GET "/v1/reservations/R1001"

    # GET — missing reservation
    run_test "Get missing reservation" 404 "$base" \
        GET "/v1/reservations/missing"

    # DELETE — cancel reservation
    run_test "Cancel reservation" 204 "$base" \
        DELETE "/v1/reservations/R1001"

    # DELETE — missing reservation
    run_test "Cancel missing reservation" 404 "$base" \
        DELETE "/v1/reservations/missing"

    # GET — parking statistics
    run_test "Get parking statistics" 200 "$base" \
        GET "/v1/parking/stats?consistency=linearizable"
}

run_suite "http://localhost:8080" "Server 1"
run_suite "http://localhost:8081" "Server 2 (forwarded)"

echo
echo "============================================"
echo "ALL HTTP STATUS CHECKS PASSED"
echo "============================================"
