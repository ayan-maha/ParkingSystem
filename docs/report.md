# Smart Parking System
## Part I - Project Report

Distributed Systems | 16 November 2026

### Executive overview

Smart Parking models parking spaces and vehicle reservations. Its public REST API supports space management, manual and automatic reservation requests, cancellation, and statistics. Its internal gRPC interface provides typed server-to-server communication. The implementation demonstrates the complete path from an HTTP request to a gRPC method and back, including forwarding between two nodes.

The central design decision is to separate the public interface from the internal service. HTTP handlers validate and translate requests; the gRPC implementation supplies the responses. OpenAPI and protobuf are the two contracts describing these layers, and generators produce their Go interfaces.

{{architecture}}

### Delivered in Part I

Eleven REST operations are connected to eleven RPCs. ListSpaces uses server streaming; batch creation uses client streaming. Request validation, standardized error responses, browser CORS support, embedded Swagger tools, and automated verification support the communication infrastructure.

The service returns dummy data. No write modifies persistent parking state. The report distinguishes the intended domain rules from fixed demonstration cases throughout; it does not claim that a real parking allocator, expiration scheduler, or replicated system has been implemented.

### How to read this report

| Pages | Topic |
| --- | --- |
| 2-4 | Domain model, REST API, protobuf, and RPC shapes |
| 5-7 | Request flow, batch/list streams, errors, and CORS |
| 8 | Design rationale, presentation, and color choices |
| 9-11 | Explanation of each project file |
| 12-14 | Generation, verification, examples, and coverage |

This report is based on the inspected repository and the supplied Part I template/tutorial. A separate official assignment handout or grading rubric was not available. The coverage discussion therefore refers to the known template guidance rather than claiming compliance with an unseen rubric.

<!-- page -->

# 2. Service model and parking rules

### Why a parking service?

Parking offers more than independent key/value updates: a reservation connects a vehicle to a space, availability limits which requests should succeed, and accessible spaces require eligibility. These relationships make the service suitable for demonstrating a read-modify-write operation while retaining a small, understandable API.

### Main objects

| Object | Important fields | Meaning |
| --- | --- | --- |
| ParkingSpace | spaceId, type, status, optional reservationId | A regular or accessible space and its availability |
| Reservation | reservationId, spaceId, licensePlate, hasAccessiblePermit, createdAt, expiresAt | A vehicle's booking of a space |
| ParkingStats | totalSpaces, availableSpaces, reservedSpaces, regularSpaces, accessibleSpaces | Summary counts for the parking lot |

Space types are REGULAR and ACCESSIBLE. Space states are AVAILABLE and RESERVED. Timestamps are represented as date-time strings in JSON and as strings in protobuf; the HTTP adapter parses them using RFC3339 when building its typed REST response.

### Intended service semantics

Manual assignment uses the supplied space ID. Automatic assignment, as described by the API, prefers the available regular space with the lowest numerical ID. A permit holder may fall back to the lowest-numbered accessible space if no regular space is available. An accessible space requires a permit; a permit holder can also use a regular space.

The API describes one active reservation per license plate and a 30-minute reservation lifetime. Reservation creation checks eligibility and availability before allocating a space. Cancellation releases it. Deleting a reserved space and changing its type are prohibited by the domain model.

### What the current dummy methods actually do

| Fixed input / behavior | Demonstration result |
| --- | --- |
| P01 | Regular, available example |
| P02 | Reserved example and a conflict trigger |
| P15 | Accessible example; reservation requires permit |
| missing | Not-found trigger for relevant operations |
| PUT for P99 | Creation response with HTTP 201 |
| Automatic reservation | Always selects dummy P01 |
| Reservation creation | Returns R1001 plus the processing node ID |

There is no active-reservation lookup, real fallback search, timer, or saved status update. Statistics are fixed examples and are not derived from the three list items. In the current dummy PATCH method, P02 is rejected for any requested type; PUT permits the unchanged REGULAR type for P02. These are explicit demonstration cases, not a complete state machine.

<!-- page -->

# 3. The REST API

REST is the interface used by curl, Swagger UI, and other HTTP clients. Requests identify resources through paths and use HTTP methods to express operations. Request and success-response bodies use JSON. The contract is in `api/openapi.yaml` (OpenAPI 3.0.3).

### Complete REST-to-RPC mapping

All RPC names below belong to `parking.v1.ParkingService`.

| HTTP method and path | RPC | Success | RPC shape |
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

### Resources and request fields

`ParkingSpace` contains `spaceId`, `type`, `status`, and an optional `reservationId`. Types are REGULAR or ACCESSIBLE; statuses are AVAILABLE or RESERVED. Listing supports optional `type` and `status` filters.

`Reservation` contains identifiers, a license plate, the accessible-permit flag, and creation/expiration timestamps. A create request requires `licensePlate` and `hasAccessiblePermit`. Supplying `spaceId` requests manual assignment; omitting it requests automatic assignment.

`ParkingStats` contains total, available, reserved, regular, and accessible space counts. The Part I server returns fixed statistics; these are not calculated from the list endpoint.

### PUT and PATCH

Both currently accept a space type. PUT represents creation or replacement and can return 201 for creation or 200 for an update. PATCH represents changing an existing space and can return 404. In the dummy implementation, PUT returns its creation case for P99; PATCH returns its missing case for the identifier `missing`.

### Validation

Before the generated REST decoder runs, OpenAPI validation checks matching API requests. It rejects malformed JSON, missing required fields, invalid enum values, and strings constrained by `minLength: 1`. A required boolean may be false, but it must be present. Empty batches are rejected by `minItems: 1`.

<!-- page -->

# 4. gRPC, protobuf, and streaming

gRPC is the internal communication interface. Go code calls named RPC methods through a generated client. Protocol Buffers defines the typed messages carried by those calls. The contract lives in `proto/parking/v1/parking.proto`.

### The definition is not the stub

The `.proto` file declares the `ParkingService` methods and message structures. Generation produces `parking.pb.go` for Go message types and `parking_grpc.pb.go` for client stubs, server interfaces, stream interfaces, and registration code. The handwritten implementation is `parking_server.go`.

The protobuf package is `parking.v1`. The `go_package` option points generated Go imports at `github.com/your-team/kvs/gen/parking/v1` with alias `parkingv1`; the module path still comes from the template.

### Understanding fields

```proto
message ParkingSpace {
  string space_id = 1;
  SpaceType type = 2;
  SpaceStatus status = 3;
  optional string reservation_id = 4;
}
```

The numbers identify fields in the protobuf wire format. `space_id = 1` does not assign the value 1. Each field has its own number within the message. `optional` allows the implementation to distinguish an absent field from a supplied value. REST uses names such as `spaceId`; protobuf uses `space_id` and generated Go uses `SpaceId`.

### Unary: one request, one response

Nine methods are unary, including GetSpace, PutSpace, and CreateReservation. For example, GetSpace receives a GetSpaceRequest with an ID and returns a GetSpaceResponse containing one ParkingSpace. Each RPC has a separately named request and response message. Delete/cancel responses are empty messages.

### Server streaming: listing spaces

```proto
rpc ListSpaces(ListSpacesRequest)
    returns (stream ListSpacesResponse);
message ListSpacesResponse { ParkingSpace space = 1; }
```

One request produces zero or more response messages. The server calls `Send()` once per matching space; the client calls `Recv()` until `io.EOF`. A single field is sufficient because each message carries one space. A `repeated` field would instead put a list inside each message.

The REST handler collects this finite stream into one JSON array. No matching spaces produces `[]`. This is not a subscription to future changes, and the REST response itself is not streamed item by item.

### Client streaming: creating a batch

CreateSpacesBatch receives a stream of requests and returns one response. Each request contains one space ID and type. Its response uses `repeated ParkingSpace spaces = 1` because that single response contains all results. This demonstrates the template's streaming pattern for a variable number of items.

<!-- page -->

# 5. Following a request through the code

### Example A: get parking space P15

```http
GET /v1/spaces/P15
```

1. `main.go` receives the request through its HTTP server. With CORS enabled, the outer helper first adds the appropriate browser headers.
2. `validation.go` matches the OpenAPI route and validates the request. Requests outside the API contract, such as documentation routes, pass through to ServeMux.
3. `rest.gen.go` routes the URL, decodes its parameters, and calls the handwritten GetSpace method through the generated strict interface.
4. `handlers.go` creates `GetSpaceRequest{SpaceId: req.SpaceId}` and calls `h.Parking.GetSpace(...)`. The generated gRPC client sends the call to the configured target.
5. `parking_server.go` receives it. P15 is a fixed dummy accessible space, so the method constructs an AVAILABLE ParkingSpace and wraps it in GetSpaceResponse.
6. The REST handler converts the protobuf space into the generated REST type and returns GetSpace200JSONResponse. The generated HTTP layer writes HTTP 200 with JSON.

```json
{"spaceId":"P15","type":"ACCESSIBLE","status":"AVAILABLE"}
```

### Example B: create a reservation through node 2

```http
POST /v1/reservations
Content-Type: application/json

{"licensePlate":"1234567",
 "hasAccessiblePermit":true,"spaceId":"P15"}
```

The REST handler forwards the plate, permit flag, and optional space ID in a CreateReservationRequest. If node 2 targets node 1, the dummy implementation on node 1 returns the reservation ID `R1001-n1`. Node 2 converts the timestamps to REST date-time values and returns HTTP 201.

The result demonstrates that the RPC ran on node 1, even though the HTTP request reached node 2. The HTTP layer does not generate the reservation ID or maintain a map of reservations.

### Contract semantics versus the dummy implementation

The API describes automatic assignment, one active reservation per plate, and a 30-minute expiration. These are service semantics; Part I does not implement a mutable allocator, uniqueness tracking, or expiration scheduling.

Today, automatic assignment returns P01. P02 triggers an occupied-space error. P15 requires a permit. The ID `missing` triggers not-found responses. Creation and expiration times are fixed examples, not current clock values. Reservation creation is the intended read-modify-write operation, but no real state transition occurs in this part.

<!-- page -->

# 6. Listing and batch creation in detail

### ListSpaces: adapting a stream to a JSON list

The REST client sends one GET request with optional type/status filters. The handler translates supplied filters into optional protobuf enum fields. The server considers the fixed P01, P02, and P15 examples and sends a ListSpacesResponse for each match.

```text
REST handler                     gRPC server
    | ---- ListSpaces request ------> |
    | <--- response: P01 ------------ |
    | <--- response: P02 ------------ |
    | <--- response: P15 ------------ |
    | <--- stream finished ---------- |
    |
    +---- HTTP 200: [P01, P02, P15] to REST client
```

The handler accumulates results before returning. If receiving a message fails, it returns the error rather than a successful partial array. This keeps the REST success body consistent with its array schema.

### CreateSpacesBatch: adapting a JSON list to a stream

The REST input remains one JSON object containing an array:

```json
{"spaces":[
  {"spaceId":"P30","type":"REGULAR"},
  {"spaceId":"P31","type":"ACCESSIBLE"}
]}
```

1. OpenAPI validation checks the JSON structure and item schemas. The handler also checks that a nonempty batch exists and converts the types to protobuf enums.
2. The handler opens CreateSpacesBatch and calls `Send()` for each item. P30 and P31 are separate messages on the same RPC stream.
3. The gRPC server receives messages until `io.EOF`. It rejects empty IDs, invalid types, duplicate IDs within the batch, and the fixed P02 existing-space conflict.
4. A request-local `seen` map detects duplicates. It is temporary validation data, not persistent parking state. The server builds a temporary slice of dummy results.
5. After a nonempty stream completes successfully, the server calls `SendAndClose()` with one response containing all spaces. It does not return a successful partial batch.
6. The client obtains this response through `CloseAndRecv()`. If the server rejects an item early and Send reports EOF, the handler receives the server's final status. Success becomes HTTP 201 with a `spaces` array.

### What the streaming choice demonstrates

Unary calls with repeated fields could represent these operations too. The selected design exercises both stream directions for variable-size operations. HTTP clients keep simple JSON requests and responses while the internal interface demonstrates gRPC streams. The batch result is validation-before-success, not a database transaction: nothing is committed or rolled back in Part I.

<!-- page -->

# 7. Errors, CORS, and server lifecycle

### Error translation

The dummy gRPC methods return gRPC status errors instead of putting error fields in success messages. `httpStatusFromGRPC` is the shared translation point in the HTTP layer.

| gRPC status | HTTP status | Meaning |
| --- | --- | --- |
| NotFound | 404 | Requested resource is missing |
| InvalidArgument | 400 | Request content is invalid |
| PermissionDenied | 403 | Accessible permit required |
| AlreadyExists | 409 | Fixed existing/occupied conflict |
| FailedPrecondition | 409 | Reserved space cannot be changed/deleted |
| Unimplemented | 501 | RPC implementation is unavailable |
| Unavailable | 503 | gRPC service cannot serve the call |
| Other errors | 500 | Internal failure |

OpenAPI validation and strict decoding failures return 400. `writeParkingError` writes `application/problem+json`, including a title, numeric status, and detail. The current detail uses the error string, which can include the gRPC status description. OpenAPI documents each operation's expected application failures, plus 500 and 503. The 501 mapping is a fallback in the implementation.

```json
{"title":"Not Found","status":404,
 "detail":"rpc error: code = NotFound desc = parking space not found"}
```

### CORS: browser permission between origins

Origins include scheme, host, and port. Swagger UI at localhost:8091 and the API at localhost:8080 have different origins. The `withCORS` helper can add Access-Control-Allow-Origin and allowed-method/header responses so the browser permits the call. It handles OPTIONS preflight requests with HTTP 204 and allows GET, POST, PUT, PATCH, DELETE, and OPTIONS.

`-cors-origin` selects an allowed origin; empty disables CORS headers. `make run CORS='*'` permits any origin for development. The existing header lists also include If-Match and expose ETag, although parking handlers do not implement those features. CORS is not authentication and does not control gRPC calls or curl. The embedded /docs page uses the API's own origin.

### Starting a node

`main.go` reads -id, -http, -grpc, -target, and -cors-origin. It registers ParkingService on a gRPC server and enables reflection. It creates a gRPC client to -target; by default that is its own localhost gRPC port. Local demo connections use insecure transport credentials. It then registers documentation routes, attaches REST routing/validation, and starts the HTTP server.

Each node hosts both servers. Node 1 normally uses HTTP 8080 and gRPC 50051. Node 2 uses HTTP 8081 and gRPC 50052 but targets node 1 at 50051. On an interrupt/termination signal, the node attempts HTTP shutdown with a five-second timeout, gracefully stops gRPC, and closes its client connection.

<!-- page -->

# 8. Design decisions and visual presentation

### Why two contracts?

OpenAPI gives browser and HTTP clients an explicit description of paths, JSON, and status codes. Protobuf gives internal callers typed messages and RPC methods, including streams. The handwritten adapter is the boundary between them: it translates names, enum values, optional fields, timestamps, successful results, and errors.

This separation keeps the HTTP layer free of parking state. Later service behavior could change behind the RPC interface without moving application logic into HTTP handlers. Generated interfaces help expose mismatches when a contract changes, while tests check behavior across those interfaces.

### Why these resource boundaries?

Spaces and reservations are distinct resources because a space can exist without a reservation, and a reservation has its own ID and lifecycle. Statistics is a separate read operation. Batch creation is an explicit endpoint because it accepts multiple space definitions as one public request and demonstrates internal client streaming.

PUT and PATCH overlap in their current body shape because the editable space configuration has only a type. Their existence semantics differ: PUT can create, while PATCH requires an existing space. Separate named RPC requests/responses keep each method understandable even when fields are similar.

### The idea of a clear interface with meaningful colors

A future parking interface should make availability, eligibility, and conflicts easy to recognize. A dashboard could show total/available/reserved counts, filterable space cards, a manual or automatic reservation form, and a reservation summary. This is a presentation concept; Part I currently provides Swagger documentation, not a custom parking dashboard.

| Color | Proposed role | How the interface should use it |
| --- | --- | --- |
| Navy #173147 | Structure and headings | Navigation, page titles, table headers |
| Teal #087E8B | Primary actions | Reserve/search actions and selected filters |
| Green #166534 | Available / successful | AVAILABLE label with check icon |
| Amber #92400E | Reserved / attention | RESERVED label with lock icon |
| Red #B91C1C | Error / conflict | Explain failed requests next to the affected field |
| Pale blue #EEF5F8 | Background grouping | Table stripes, panels, and code examples |

Color should reinforce text rather than replace it. Use explicit AVAILABLE/RESERVED labels and an accessible-space label/icon. A permit field must remain visible and understandable without interpreting color. Maintain readable contrast, consistent spacing, clear headings, and visible keyboard focus; avoid using red versus green as the only distinction.

This report uses navy, teal, and pale blue to separate headings, explanations, and reference material. Diagrams show communication direction; tables support comparison; worked examples connect concepts to the actual endpoint names. Visual design supports understanding while leaving implementation details to technical documentation.

<!-- page -->

# 9. File reference: contracts and generated code

### api/openapi.yaml

The public API contract. Defines all 11 HTTP operations, operationId names, JSON schemas, required fields, enums, query/path parameters, status codes, and Problem Details errors. Edit this file before regenerating the REST code. It describes intended parking semantics as well as the shapes demonstrated in Part I.

### api/oapi-codegen.yaml

Configuration for oapi-codegen. Selects package `rest`, output `gen/rest/rest.gen.go`, and generation of models, the standard-library HTTP server, and the strict server interface. A strict response interface gives handlers typed response alternatives rather than handwritten success JSON.

### api/spec.go

Uses Go's embed facility to put openapi.yaml, docs.html, and editor.html into the server executable as Spec, DocsHTML, and EditorHTML. Servers can serve those resources without loading external project files at runtime. Rebuild after changing embedded files.

### api/docs.html

Hosts Swagger UI for viewing the OpenAPI document and sending test requests. Defaults to /openapi.yaml and supports a `url` query override. JavaScript/CSS load from a CDN, so the browser needs internet access. The browser-tab title still contains the template label "KV Store API"; the actual displayed API contract is the parking specification.

### api/editor.html

Hosts Swagger Editor with editing and preview. It loads the specification and supports a `url` override. Browser edits do not save to the repository: export YAML and overwrite api/openapi.yaml, then regenerate. This page also retains a template browser-tab title and loads assets from a CDN.

### proto/parking/v1/parking.proto

The internal API contract. Declares ParkingService, its 11 RPCs, SpaceType/SpaceStatus enums, resource messages, and a dedicated request/response message for each method. Declares server streaming for ListSpaces and client streaming for CreateSpacesBatch. This is source code for generation, not the generated stub itself.

### gen/rest/rest.gen.go (generated)

Contains REST models, enums, parameter binding, HTTP routing, StrictServerInterface, request objects, response objects, and JSON response writers. `Handlers` implements the required methods. Generated interfaces connect operationId values to Go methods. Do not hand-edit this file.

### gen/parking/v1/parking.pb.go (generated)

Contains protobuf Go structs, enum definitions, field getters, serialization/reflection support, and descriptors. Used on both sides of the gRPC connection. Generated from parking.proto; do not hand-edit it.

### gen/parking/v1/parking_grpc.pb.go (generated)

Contains ParkingServiceClient, ParkingServiceServer, the unimplemented server embedding, unary/streaming transport glue, method descriptors, and RegisterParkingServiceServer. Calling the client stub invokes an RPC; registering an implementation makes methods available to incoming calls. Do not hand-edit it.

<!-- page -->

# 10. File reference: application and tests

### cmd/server/main.go

The executable entry point and composition root. Creates the listeners, registers the gRPC implementation, creates the outbound gRPC client, wires HTTP handlers, serves the documentation, and handles shutdown. `portOf` extracts the port when constructing the default local target. `withCORS` wraps browser-facing HTTP requests.

### cmd/server/main_test.go

Tests the CORS helper. An OPTIONS preflight must receive 204, include the configured origin and PUT permission, and stop before the underlying application handler. An actual PUT must reach that handler. This tests the helper; earlier live two-node verification also exercised its wiring in the executable.

### internal/httpapi/handlers.go

The handwritten adapter between HTTP and gRPC. `Handlers` holds a ParkingServiceClient, not a parking database. NewHTTPHandler creates the strict generated server and wraps its mux with validation. Every operation translates its REST input, invokes an RPC, and returns the corresponding generated success response.

ListSpaces receives a stream and builds a JSON array. CreateSpacesBatch sends a stream and receives a result array. PutSpace checks the RPC's `created` flag to select HTTP 200 or 201. DeleteSpace and CancelReservation return 204 with no success body. Conversion helpers translate protobuf enums, optional reservation IDs, and RFC3339 timestamps to REST models. The error mapping and Problem Details writer live here too.

### internal/httpapi/validation.go

Loads the embedded OpenAPI document, removes its server-address restrictions for route matching, and creates a kin-openapi router. For matching API operations, ValidateRequest enforces the schema before Go decoding. This catches required-field presence, which ordinary decoding into zero-valued Go fields cannot reliably distinguish. Invalid requests return Problem Details with HTTP 400. Unmatched routes pass to ServeMux. An invalid embedded contract causes startup to fail rather than run with an unusable validator.

### internal/httpapi/handlers_test.go

Exercises the full HTTP adapter against a real in-memory gRPC server using bufconn. Tests include list filtering and empty streams, batch streaming, conflicts, invalid enums, missing/null/malformed bodies, required permit presence, empty identifiers/plates, successful reservation origin, and Problem Details shape/media type. These are integration tests within one test process; they are distinct from the two-process demo.

### internal/grpcsvc/parking_server.go

The handwritten ParkingService implementation. ParkingServer embeds the generated UnimplementedParkingServiceServer and stores only NodeID. Methods use fixed dummy cases and return protobuf results or gRPC status errors. identifyServer attaches `x-node-id` gRPC metadata; REST handlers do not copy that header into HTTP responses. The reservation ID embeds NodeID, which supports the forwarding demonstration.

Listing returns P01/P02/P15 and applies filters. Batch creation checks all received items before replying. P02 represents a conflict/reserved space, P15 represents accessible parking, and `missing` represents an absent resource. All collections in these methods are local temporary data. There is no shared parking map, persistent store, or actual reservation lifecycle.

<!-- page -->

# 11. File reference: tooling and supporting files

### Makefile

Coordinates setup, generation, checks, and running. `setup` records/installs the Go generators and checks for protoc; it cannot install protoc itself. `gen-rest` runs oapi-codegen and `gen-proto` runs protoc with Go plugins. `build` generates code, tidies dependencies, runs vet/tests, and builds the server. `run` builds and starts node 1; `run2` uses the existing binary and targets node 1. Other targets run demos, optional Docker tools, and clean generated/build outputs. The recipes assume Unix-style shell utilities.

### go.mod and go.sum

go.mod declares module `github.com/your-team/kvs`, Go 1.26.0, generator tools, and dependency versions. Main libraries include gRPC, protobuf, the oapi-codegen runtime, and kin-openapi. go.sum stores dependency checksums so fetched module content can be checked. Generated tools and application dependencies are managed together by Go modules.

### etc/demo.sh

A Bash script containing 40 request cases, run against both default HTTP nodes for 80 status checks. Covers every REST operation plus common validation, permission, missing-resource, and conflict paths. Each case uses curl, prints the body/status, and fails if the status differs. It expects both nodes to be running; its principal assertions are status codes.

### etc/check.sh

A self-contained Bash integration runner. Calls make build, starts two nodes on HTTP 18080/18081 and gRPC 15051/15052, waits for readiness, adapts the demo URLs, runs the suite, and verifies node 2 returns `R1001-n1`. A cleanup trap stops its server processes. Logs go to /tmp. The alternative ports avoid clashing with ordinary development servers.

### docker/api-tools.compose.yaml

Optional browser API tools: Swagger Editor on 8090, Swagger UI on 8091, and Scalar API Reference on 8092. They mount the API specification read-only. They do not run the parking servers or save edits into the repository. Their different origins may require enabling CORS for requests to the parking API. Current container tags are `latest`.

### README.md and .gitignore

README.md provides the project overview, commands, API mapping, examples, error table, and Part I limitations. .gitignore excludes gen/ and bin/, which are recreated by generation/build. Git repository metadata under .git/ is version-control internals rather than application design.

### bin/server.exe (generated build artifact)

The Windows executable produced during verification. It includes the application, generated interfaces, and embedded API documents. It is ignored by Git. The Makefile's usual output path is bin/server on its target shell/platform.

### This document's files

`docs/design.md` and `docs/Smart_Parking_Part_I_Design.pdf` are the original code guide. `docs/report_additions.md` supplies the extended report sections, and `docs/report.md` is the assembled report source. `docs/generate_design_pdf.py` formats the source and draws the architecture diagram. `docs/Smart_Parking_Part_I_Report.pdf` is this report. The PDF libraries are document-generation tools, not Go application dependencies.

<!-- page -->

# 12. Generation, verification, and Part I boundaries

### How changes move through the project

```text
api/openapi.yaml + api/oapi-codegen.yaml
    -- make gen-rest --> gen/rest/rest.gen.go

proto/parking/v1/parking.proto
    -- make gen-proto --> gen/parking/v1/parking.pb.go
                     + gen/parking/v1/parking_grpc.pb.go

generated interfaces + handwritten implementations
    -- make build --> executable
```

Change a contract first, regenerate its Go code, then update the handwritten adapter or service to match. Generated interfaces provide compile-time checks: an outdated handler or RPC signature fails to build. Embedding also means OpenAPI/HTML edits require a rebuild before the running server serves the new content.

### Typical development commands

```bash
make setup       # install/record Go generators; protoc required
make build       # generate, tidy, vet, test, build
make run         # node 1: HTTP 8080, gRPC 50051
make run2        # node 2: HTTP 8081, target node 1
make demo        # run the 80 default-port status checks
make check       # build + temporary two-node verification
```

These Make/Bash workflows need a suitable shell and utilities. On Windows, the same Go/protoc commands and server flags can be run directly; verification in this workspace used a temporary Windows toolchain and PowerShell for process-level checks.

### Verification completed for the current code

REST/protobuf generation, go vet, go test ./..., and executable build passed. All 80 demo cases passed against two actual server processes after the API cleanup. A separate reservation request through node 2 confirmed the node-1 ID. CORS preflight, embedded documentation routes, and Problem Details responses were also checked during the earlier verification of those changes. These checks demonstrate communication and dummy response behavior, not durable state correctness.

### What Part I deliberately leaves unimplemented

There is no persistent parking state, actual creation/deletion/allocation, active-reservation tracking, expiration scheduler, replicated log, consensus algorithm, leader election, or replicated state machine. Node 2's target is configured through a flag, not discovered dynamically. A write followed by a read need not reflect the write because the service uses fixed examples.

### Short glossary

**REST contract:** HTTP paths, methods, models, and statuses. **RPC contract:** named methods and protobuf messages. **Client stub:** generated proxy that sends RPCs. **Server implementation:** handwritten methods producing dummy results.

**Unary:** one request and one reply. **Server stream:** one request and multiple replies. **Client stream:** multiple requests and one reply. **repeated:** a list inside one message. **io.EOF:** no more messages to receive.

The design separates public HTTP concerns from internal service behavior while keeping the entire implementation within Part I.

<!-- page -->

# 13. Worked examples and observable behavior

These examples illustrate the current dummy methods. They are independent requests, not a scenario that creates durable state. The commands use node 1; changing the HTTP port to 8081 sends requests through node 2 when both nodes are configured as documented.

### Read one accessible parking space

```bash
curl -i http://localhost:8080/v1/spaces/P15
```

Expected status: 200. The JSON response includes spaceId P15, type ACCESSIBLE, and status AVAILABLE. The request follows the generated HTTP route, the GetSpace handler, the client stub, and the dummy GetSpace RPC implementation.

### Request a manual accessible reservation

```bash
curl -i -X POST http://localhost:8081/v1/reservations \
  -H 'Content-Type: application/json' \
  -d '{"licensePlate":"1234567","hasAccessiblePermit":true,"spaceId":"P15"}'
```

Expected status: 201. The reservation ID is R1001-n1 when node 2 targets node 1. The request echoes the plate and permit flag, uses P15, and returns fixed creation/expiration examples. This demonstrates forwarding without persisting a reservation.

### Request the same accessible space without a permit

With hasAccessiblePermit set to false, the server returns PermissionDenied. The HTTP adapter translates it to 403 and writes a Problem Details response. Its detail currently includes the gRPC error text.

```json
{"title":"Forbidden","status":403,
 "detail":"rpc error: code = PermissionDenied desc = accessible parking permit required"}
```

### Submit a batch

```bash
curl -i -X POST http://localhost:8080/v1/spaces/batch \
  -H 'Content-Type: application/json' \
  -d '{"spaces":[{"spaceId":"P30","type":"REGULAR"},{"spaceId":"P31","type":"ACCESSIBLE"}]}'
```

Expected status: 201 with a spaces array. The HTTP body is one JSON document; internally the handler sends two messages on a client stream. Duplicate IDs within a batch cause 400, and including P02 triggers 409.

### Other useful checks

GET /v1/spaces/missing returns 404. An invalid type on POST/PUT/PATCH returns 400. GET /v1/spaces?type=ACCESSIBLE&status=RESERVED returns an empty array for the fixed dataset. DELETE /v1/spaces/P02 returns 409. A successful delete returns 204 without a success body, but a later read does not prove deletion because no state changes are saved.

<!-- page -->

# 14. Coverage, evidence, and scope

### Coverage of the supplied template guidance

| Topic | Where it is implemented / explained |
| --- | --- |
| Own service instead of key-value example | Parking spaces, reservations, permits, and statistics |
| OpenAPI contract and operation IDs | api/openapi.yaml; generated REST interface |
| Internal protobuf contract | proto/parking/v1/parking.proto |
| Dedicated request and response messages | Separate messages for every RPC |
| Variable-size operations using streams | ListSpaces and CreateSpacesBatch |
| Dummy gRPC responses | internal/grpcsvc/parking_server.go |
| Stateless HTTP forwarding | internal/httpapi/handlers.go |
| gRPC status errors and HTTP mapping | Shared error mapping and Problem Details writer |
| REST-to-RPC mapping in report | Complete operation table in the REST section |
| Read-modify-write service operation | Reservation creation, described as intended semantics |
| Forwarding through a second node | Node 2 -target points to node 1 |
| Demonstration and verification | Go tests, demo.sh, check.sh, and live process checks |

### Verification evidence and its limits

The current application passed code generation, vet, tests, and build. The HTTP/gRPC integration tests use an in-memory gRPC transport. Separately, the 40 demo requests were exercised against each of two actual server processes: 80 HTTP status checks passed, and a reservation through node 2 returned node 1's ID.

Earlier checks of the error/CORS changes also verified Problem Details media types and fields, a live PUT preflight, and the embedded documentation routes. This report records those results from the work session. It does not contain fabricated screenshots or claim that a screenshot was captured for a check that only ran programmatically.

These checks establish request routing, translation, validation, dummy status behavior, and cross-node communication. They do not demonstrate durable allocation, concurrent reservation correctness, actual expiration, crash recovery, failover, replication, or consensus.

### Current implementation boundaries

All server methods use fixed examples or request-local temporary collections. NodeID is the only service-instance field besides the generated server embedding. The reservation uniqueness and expiration rules exist in the API design, while real enforcement remains unimplemented. The target node is configured manually rather than selected by a leader-discovery mechanism.

Swagger documentation is implemented. The custom dashboard and status colors are future presentation ideas. Some module and browser-tab naming still comes from the template.

Submission-specific author details, screenshots, and formatting requirements should follow the course's actual instructions.
