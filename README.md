# Smart Parking System - Distributed Systems, Part I

**Authors:** Hala Atamna (213620651), Ayan Mahameed (214556532).

Smart Parking demonstrates a parking service's REST-to-gRPC communication infrastructure. It uses OpenAPI-generated HTTP interfaces, protobuf messages, generated gRPC stubs, handwritten adapters, and dummy service methods. Each node hosts both servers; its REST layer can call its own gRPC server or forward to another node.

**Part I uses dummy responses. Successful writes do not save state or affect later reads.** The current version also omits the required selectable read-consistency interface; see [Known omissions and issues](#known-omissions-and-issues).

The submission report is [Smart_Parking_Part_I_Report.pdf](docs/Smart_Parking_Part_I_Report.pdf). Its editable content is [report_submission.md](docs/report_submission.md).

## Architecture

```text
Client (curl / Swagger)
    | HTTP + JSON
    v
OpenAPI validation -> generated REST routing -> handwritten HTTP adapter
    | gRPC + protobuf, through the generated client stub
    v
ParkingService on the configured target node
    |
    v
Dummy response -> HTTP status and JSON / Problem Details
```

The HTTP layer has no parking database. The gRPC implementation uses fixed inputs and temporary request-local collections. Forwarding between nodes does not replicate state or discover a leader.

## Prerequisites and build

- Go 1.26 or later.
- `protoc`, installed separately, plus the Go generation tools.
- Make and a Bash-compatible environment with standard Unix utilities and curl for the supplied recipes/scripts.
- Internet access to download dependencies/generators. Docker is optional, only for additional API documentation tools.

From the project root:

```bash
make setup
make build
```

`make setup` checks for protoc and installs/records the Go generators. It uses `@latest`, so running it can update generator versions in go.mod/go.sum. To install the already-recorded versions without selecting newer ones, use `go install tool` instead; protoc must still be installed separately. The protobuf recipe expects generator executables under `$(go env GOPATH)/bin`.

`make build` regenerates REST/protobuf code, runs `go mod tidy`, `go vet ./...`, and `go test ./...`, then builds `bin/server`. Do not edit generated Go files manually.

On Windows, the Make/Bash recipes need a suitable shell and utilities; they are not plain PowerShell commands. The underlying Go/protoc commands can also be run directly, producing a Windows executable such as `bin/server.exe`.

## Run and demonstrate two nodes

In terminal 1:

```bash
make run
```

In terminal 2, after the first build completes:

```bash
make run2
```

| Node | REST | gRPC listener | REST layer's gRPC target |
|---|---|---|---|
| n1 | localhost:8080 | localhost:50051 | localhost:50051 |
| n2 | localhost:8081 | localhost:50052 | localhost:50051 |

Open `http://localhost:8080/docs` to try the API. A useful demo is: list/filter spaces, create a batch, reserve P15 with and without a permit, request `missing`, then demonstrate forwarding through node 2.

```bash
curl -i -X POST http://localhost:8081/v1/reservations \
  -H 'Content-Type: application/json' \
  -d '{"licensePlate":"1234567","hasAccessiblePermit":true,"spaceId":"P15"}'
```

Expected: HTTP 201 and reservation ID `R1001-n1`. The HTTP request reached n2, but n1 processed the RPC. Timestamps are fixed illustrative values. Repeating requests does not create real reservations.

## REST and gRPC operations

The complete REST contract, including bodies, parameters, and status codes, is [api/openapi.yaml](api/openapi.yaml). The internal contract is [parking.proto](proto/parking/v1/parking.proto).

| REST operation | RPC | Success | RPC shape |
|---|---|---|---|
| GET /v1/spaces | ListSpaces | 200 | Server stream |
| POST /v1/spaces | AddSpace | 201 | Unary |
| POST /v1/spaces/batch | CreateSpacesBatch | 201 | Client stream |
| GET /v1/spaces/{spaceId} | GetSpace | 200 | Unary |
| PUT /v1/spaces/{spaceId} | PutSpace | 200 / 201 | Unary |
| PATCH /v1/spaces/{spaceId} | PatchSpace | 200 | Unary |
| DELETE /v1/spaces/{spaceId} | DeleteSpace | 204 | Unary |
| POST /v1/reservations | CreateReservation | 201 | Unary |
| GET /v1/reservations/{reservationId} | GetReservation | 200 | Unary |
| DELETE /v1/reservations/{reservationId} | CancelReservation | 204 | Unary |
| GET /v1/parking/stats | GetParkingStats | 200 | Unary |

ListSpaces accepts optional `type` and `status` filters. The server sends one space per RPC response; the adapter collects them into one REST JSON array. Batch creation accepts one JSON `spaces` array, sends one RPC request per item, then returns one response containing the results. An empty batch, duplicate IDs, or invalid items fail validation; no persistent transaction occurs.

## Parking semantics and dummy behavior

The service design separates spaces (REGULAR/ACCESSIBLE; AVAILABLE/RESERVED) from vehicle reservations. Manual assignment supplies a space ID. Intended automatic assignment prefers the lowest-numbered available regular space, with accessible fallback for permit holders. The design describes one active reservation per plate and a 30-minute lifetime. Reservation creation is the intended read-modify-write operation.

The current methods only demonstrate fixed cases:

- P01 is regular/available; P02 represents a reserved/existing-space conflict; P15 is accessible/available and requires a permit for reservation.
- The identifier `missing` triggers not-found responses in the relevant methods.
- Automatic reservation always selects P01. Active-plate uniqueness, capacity search/fallback, and real expiration are not implemented.
- PUT reports creation for P99. Other successful changes return dummy objects without storing them.
- Statistics are fixed examples, not totals calculated from the three listed spaces.

No persistent state machine, real allocation, replicated log, or consensus is implemented.

## Validation and errors

REST middleware validates matching requests against the embedded OpenAPI document before generated decoding. Missing required fields, invalid enums, malformed JSON, and empty constrained strings return HTTP 400. Required booleans may be false but must be supplied.

Errors use `application/problem+json` with `title`, `status`, and `detail`; the detail may include the gRPC error text.

| gRPC status | HTTP status |
|---|---|
| NotFound | 404 |
| InvalidArgument | 400 |
| PermissionDenied | 403 |
| AlreadyExists / FailedPrecondition | 409 |
| Unimplemented | 501 |
| Unavailable | 503 |
| Other failures | 500 |

## Verification

With both default-port nodes running:

```bash
make demo
```

This runs 40 curl status cases against each node: **80 checks** covering every REST operation and common validation/error paths. Its primary assertions are HTTP status codes.

For a build and automatic two-process integration check:

```bash
make check
```

This builds, starts temporary nodes on HTTP 18080/18081 and gRPC 15051/15052, runs the demo with adapted ports, checks the node-1 reservation ID returned through node 2, and stops its processes. Logs go to `/tmp/parking-n1.log` and `/tmp/parking-n2.log`.

Go tests exercise the HTTP adapter against an in-memory gRPC server, streaming, request validation, Problem Details, and the CORS helper. Previous verification passed generation, vet, tests, build, all 80 actual two-node demo checks, and forwarding. Separate live checks covered CORS preflight and documentation routes. These results establish communication and dummy behavior, not real booking correctness.

## API documentation and CORS

Each HTTP server serves:

- `/docs`: Swagger UI for viewing/testing the API.
- `/editor`: Swagger Editor; export YAML manually to update the repository.
- `/openapi.yaml`: the embedded contract.

The browser pages load assets from a CDN. Rebuild after editing embedded HTML/YAML. Optional Docker tools are managed with `make tools-up` / `make tools-down`: Editor on 8090, UI on 8091, Scalar on 8092.

For browser tools hosted at another origin, enable CORS:

```bash
make run CORS='*'
```

Or pass `-cors-origin http://localhost:8091` directly to the executable. The helper handles OPTIONS preflights and permits PUT alongside the other API methods. CORS is unnecessary for same-origin `/docs`, curl, or gRPC and is not authentication.

## Known omissions and issues

- **Consistency interface:** read-consistency options were removed from REST/protobuf. No sequential or linearizable guarantee is implemented or claimed. The required selectable interface remains missing; the report distinguishes proposed semantics from actual behavior.
- **Reserved-space PATCH:** P02 is rejected even when its requested type remains REGULAR. The contract only prohibits changing a reserved space's type; PUT permits its unchanged type.
- **Direct gRPC validation:** several unary methods rely on REST validation and do not fully validate direct RPC inputs. Batch validation is more complete.
- **HTTP fallback documentation:** the mapper supports 501, but the operation response lists do not document it.
- Some template naming remains in the Go module path and HTML tab titles. The OpenAPI overview's word "replicated" describes a future goal, not current state replication.

## Implementation map

```text
api/                    REST contract, generator config, embedded Swagger tools
proto/parking/v1/       Internal protobuf contract
gen/rest/               Generated REST models/routing/interfaces
gen/parking/v1/         Generated protobuf types and gRPC stubs
internal/httpapi/       Validation, protocol adapter, error mapping, integration tests
internal/grpcsvc/       Dummy ParkingService implementation
cmd/server/             Startup, target configuration, CORS, and CORS tests
etc/                    Demo and automatic two-node verification scripts
docker/                 Optional API documentation tools
docs/                   Submission report and document sources
Makefile                Build, generation, run, and verification commands
```
