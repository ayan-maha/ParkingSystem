{{cover}}

<!-- page -->

# 1. Overview and service design

Smart Parking is a Part I communication prototype for managing parking spaces and vehicle reservations. Clients use a REST API; HTTP handlers translate their requests into gRPC calls. Each node runs both an HTTP server and a gRPC server, so a REST request can be served locally or forwarded to another node.

{{architecture}}

### Domain model

| Resource | Main fields | Purpose |
| --- | --- | --- |
| ParkingSpace | spaceId, type, status, optional reservationId | A REGULAR or ACCESSIBLE space, AVAILABLE or RESERVED |
| Reservation | reservationId, spaceId, licensePlate, permit flag, timestamps | A vehicle's booking of a space |
| ParkingStats | total, available, reserved, regular, accessible counts | Parking-lot summary |

### Intended parking rules

Manual assignment uses a supplied space ID. Automatic assignment is designed to prefer the available regular space with the lowest numerical ID. A permit holder may fall back to an accessible space if regular spaces are unavailable. Accessible spaces require a permit; permit holders may also use regular spaces.

The API describes one active reservation per license plate and a 30-minute reservation lifetime. Reserved spaces cannot be deleted or have their type changed. Reservation creation is the intended read-modify-write operation: check eligibility and availability, then reserve a space as one service operation.

### What Part I implements

The implementation provides contracts, generated interfaces, HTTP validation, RPC forwarding, streaming examples, dummy responses, errors, CORS, and tests. It does not implement persistent parking state, actual allocation, active-reservation tracking, expiration scheduling, replication, or consensus.

The fixed examples are P01 (regular/available), P02 (regular/reserved), and P15 (accessible/available). The identifier `missing` triggers not-found cases. Automatic reservation returns P01; reservation creation returns `R1001-<node ID>` and fixed timestamps. Statistics are fixed and are not calculated from the list. Writes do not affect later reads.

<!-- page -->

# 2. REST API and operation mapping

REST is the public HTTP interface used by curl and Swagger UI. `api/openapi.yaml` defines paths, methods, JSON schemas, required fields, operation IDs, and status codes. Success bodies use JSON; errors use Problem Details. All RPCs below belong to `parking.v1.ParkingService`.

| HTTP operation | RPC | Success | RPC shape |
| --- | --- | --- | --- |
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

### Request validation and method semantics

Listing accepts optional type and status filters. Creating a space requires spaceId and type. PUT/PATCH accept a type: PUT can create or replace, while PATCH only modifies an existing space. In the dummy implementation, PUT uses P99 for its creation case and PATCH rejects `missing`.

Creating a reservation requires licensePlate and hasAccessiblePermit. The boolean may be false, but it must be present. spaceId is optional: present means manual assignment, absent means automatic assignment. Batch creation accepts a nonempty spaces array. DELETE success returns 204 with no body.

The OpenAPI validator runs before decoding into generated Go request types. It rejects malformed JSON, missing required fields, invalid enums, empty strings constrained by minLength, and empty batches. Conversion alone would not enforce all these rules or distinguish every missing field from a Go zero value.

### Example: reservation through node 2

```bash
curl -i -X POST http://localhost:8081/v1/reservations \
  -H 'Content-Type: application/json' \
  -d '{"licensePlate":"1234567","hasAccessiblePermit":true,"spaceId":"P15"}'
```

When node 2 targets node 1, the expected response is HTTP 201 with reservation ID R1001-n1. Without the accessible permit, this request returns HTTP 403. This tests communication and dummy domain checks; it does not save a booking.

<!-- page -->

# 3. gRPC, streaming, and request flow

The internal interface is defined in `parking.proto`. Protobuf declares typed messages; gRPC transports calls to named methods. Generated client stubs let HTTP handlers invoke those methods. The handwritten ParkingServer implements them and returns dummy results or gRPC status errors.

Each RPC has its own request and response message. Separate message types keep each operation explicit, even when some fields are similar. Optional fields represent filters and manual-assignment choices without adding parking state to the HTTP layer.

### Unary calls

Nine methods use one request and one reply. For example, GetSpace receives an ID and returns one ParkingSpace. DeleteSpace and CancelReservation return empty response messages on success. PutSpace returns a created flag so the HTTP layer can choose 201 or 200.

### Server streaming: ListSpaces

```proto
rpc ListSpaces(ListSpacesRequest)
    returns (stream ListSpacesResponse);
message ListSpacesResponse { ParkingSpace space = 1; }
```

The server filters its fixed examples and calls Send once per matching space. The HTTP handler receives messages until io.EOF, collects them, and returns one JSON array. No matches produces `[]`. The stream is finite; it does not watch future changes. A repeated field is unnecessary because each streamed reply already represents one space.

### Client streaming: CreateSpacesBatch

The REST client still sends one JSON array. Internally, the handler opens one RPC stream and sends one request per space, then uses CloseAndRecv to obtain the result. The server receives until EOF, validates IDs/types/duplicates, and uses SendAndClose to return one reply containing a repeated spaces list. P02 triggers an existing-space conflict; an empty stream is invalid.

The server returns success only after validating the batch. Its temporary slice and duplicate-detection map are request-local data, not persistent state. No database transaction is performed. Streaming demonstrates variable-size communication without changing the REST request/response shapes.

### A request through both layers

GET /v1/spaces/P15 passes through CORS (if enabled), OpenAPI validation, generated HTTP routing, and the GetSpace handler. The handler builds a protobuf request and calls its client stub. ParkingServer constructs the dummy response; the adapter translates it to REST types and returns HTTP 200 JSON.

Node 1 normally listens on HTTP 8080 and gRPC 50051. Node 2 listens on HTTP 8081 and gRPC 50052, with its outbound target set to node 1 at 50051. The node-1 reservation identifier proves which gRPC server processed a request received by node 2.

<!-- page -->

# 4. Implementation structure and design decisions

The implementation separates API contracts, generated interfaces, the HTTP adapter, and the dummy service. The HTTP layer holds a ParkingServiceClient, not a parking database. Generated interfaces connect both contracts to their handwritten implementations.

| Main file or group | Responsibility |
| --- | --- |
| api/openapi.yaml | Public paths, operations, schemas, validation constraints, and error responses. |
| api/oapi-codegen.yaml | Configures generated REST models, routing, and the strict handler interface. |
| proto/parking/v1/parking.proto | Internal RPC methods, message types, enums, and stream directions. |
| cmd/server/main.go | Starts both servers, registers ParkingService, configures the target client, and wires CORS/documentation/shutdown. |
| internal/httpapi/handlers.go | Converts REST requests to RPCs and results to HTTP responses; handles streams, enum/time conversion, and errors. |
| internal/httpapi/validation.go | Validates matching HTTP requests against the embedded OpenAPI document. |
| internal/grpcsvc/parking_server.go | Implements fixed dummy results and gRPC status errors; attaches processing-node metadata. |
| gen/rest/rest.gen.go | Generated HTTP models, routes, request binding, strict interface, and response writers. |
| gen/parking/v1/parking.pb.go | Generated protobuf message types, enums, and serialization support. |
| gen/parking/v1/parking_grpc.pb.go | Generated RPC client stubs, server/stream interfaces, and registration code. |
| internal/httpapi/handlers_test.go | HTTP-to-gRPC integration checks using an in-memory gRPC transport. |
| cmd/server/main_test.go | CORS preflight and actual-request forwarding checks. |
| etc/demo.sh; etc/check.sh | HTTP status suite and automated two-process forwarding verification. |
| Makefile | Generation, build, tests, execution, and optional API-tool commands. |

### Supporting components

api/spec.go embeds the contract and Swagger HTML; docs.html and editor.html expose /docs and /editor. Docker Compose optionally serves API tools on separate ports. go.mod/go.sum manage dependencies; README.md documents usage. Generated files and binaries are recreated by the build rather than edited manually.

### Design rationale

OpenAPI describes public HTTP communication, while protobuf describes internal service calls. The adapter translates between them and keeps browser concerns outside the service. Generated interfaces catch signature mismatches; integration tests exercise the complete boundary.

Spaces and reservations are separate resources because spaces exist independently of bookings. PUT may create or replace a space, while PATCH requires an existing space. Batch creation and list streaming demonstrate variable-size RPC communication. All temporary batch data remains local to the request.

<!-- page -->

# 5. Errors, CORS, build, and verification

### Errors

The server reports gRPC status errors. The HTTP adapter maps them centrally and writes application/problem+json with title, status, and detail. OpenAPI validation/decoding failures also return HTTP 400. The current detail may contain the gRPC error string.

| gRPC status | HTTP result |
| --- | --- |
| NotFound | 404 - missing resource |
| InvalidArgument | 400 - invalid input |
| PermissionDenied | 403 - accessible permit required |
| AlreadyExists / FailedPrecondition | 409 - existing, occupied, or reserved-space conflict |
| Unimplemented / Unavailable | 501 / 503 - fallback or unavailable service |
| Other errors | 500 - internal failure |

### CORS and startup

CORS lets browser JavaScript call an API at another origin. Swagger UI on port 8091 and the API on 8080 have different origins. The -cors-origin flag enables the helper; `make run CORS='*'` permits any origin for development. It answers OPTIONS preflights with 204 and permits PUT along with the other API methods. Same-origin /docs, curl, and internal gRPC calls do not need CORS.

Each node reads -id, -http, -grpc, -target, and -cors-origin. An omitted target points to its own localhost gRPC port. Reflection is enabled for inspection; local demo calls use insecure gRPC credentials. Shutdown attempts to stop HTTP gracefully with a timeout, stops gRPC, and closes the client connection.

### Generation and commands

```bash
make setup       # install Go generators; protoc must be available
make build       # generate REST/protobuf, tidy, vet, test, build
make run         # start node 1
make run2        # start node 2, forwarding to node 1
make demo        # 80 status checks on default ports
make check       # temporary two-node build/integration checks
```

OpenAPI produces rest.gen.go; protoc produces the protobuf messages and gRPC interfaces. A contract change requires regeneration and matching handwritten methods. Embedded API/HTML changes require rebuilding. Make/Bash commands need a suitable shell; the same Go/protoc tools and flags can be used directly on Windows.

### Verification and limits

Generation, go vet, Go tests, and the build passed. All 80 demo status checks passed across two actual processes; a request through node 2 returned R1001-n1. Separate checks exercised Problem Details, live PUT CORS preflight, and documentation routes. These establish communication and dummy behavior, not durable booking correctness.

These checks validate API communication, request conversion, fixed error cases, and cross-node forwarding. Persistent booking correctness, actual expiration, replication, and consensus remain outside the implemented Part I scope.
