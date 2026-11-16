{{cover}}

<!-- page -->
# 1. Service semantics and architecture

Smart Parking manages spaces and vehicle reservations. A ParkingSpace has an ID, type (REGULAR or ACCESSIBLE), availability (AVAILABLE or RESERVED), and an optional reservation ID. A Reservation links a vehicle's license plate to a space and includes permit eligibility and creation/expiration times. ParkingStats provides aggregate counts.

Manual reservations identify a space. Automatic assignment is designed to choose the lowest-numbered available regular space, falling back to an accessible space for a permit holder. Accessible spaces require a permit; permit holders may use regular spaces. The service design allows one active reservation per plate and a 30-minute reservation lifetime. Reserved spaces cannot be deleted or have their type changed. These are intended semantics; the Part I implementation uses dummy cases.

{{architecture}}

Every node hosts HTTP and gRPC servers. The stateless REST adapter validates HTTP input, translates it into protobuf, calls its configured target, and converts the reply back to JSON. Node 1 normally uses HTTP 8080 / gRPC 50051; node 2 uses HTTP 8081 / gRPC 50052 and targets node 1 at 50051. This is configured forwarding, not replication or leader discovery.

### Operations

The table states the intended stateful-service effects. In Part I, writes return dummy responses without modifying state. The missing consistency interface and guarantees are disclosed in Section 6.

| Operation | Intended effect |
| --- | --- |
| ListSpaces | Read filtered spaces |
| GetSpace | Read one space |
| AddSpace | Create an available space |
| CreateSpacesBatch | Create spaces as one batch |
| PutSpace | Create/replace space configuration |
| PatchSpace | Modify an existing space |
| DeleteSpace | Delete an available space |
| CreateReservation | Check eligibility/availability and reserve |
| GetReservation | Read an active reservation |
| CancelReservation | Cancel and release the space |
| GetParkingStats | Read summary counts |

Part I demonstrates request/reply communication; the table does not claim that real state transitions are implemented.

<!-- page -->
# 2. REST API: parking-space operations

The public contract is `api/openapi.yaml`. All JSON request/success bodies use application/json; errors use application/problem+json. Required path parameters are strings. Only ListSpaces declares query parameters. No operation declares a custom required header. Body/schema names below are defined fully on the next page. Every listed status comes from the current OpenAPI contract.

### GET /v1/spaces - listSpaces

Parameters: optional query type=REGULAR|ACCESSIBLE and status=AVAILABLE|RESERVED. Request body: none. Response: 200 with an array of ParkingSpace objects (including an empty array when no spaces match). Errors: 400 invalid query, 500 internal failure, 503 service unavailable.

### POST /v1/spaces - createSpace

Parameters: none. Required request body: CreateSpaceRequest. Response: 201 with ParkingSpace. Errors: 400 invalid input, 409 existing space, 500, 503. The dummy existing-space trigger is P02; successful results are not stored.

### POST /v1/spaces/batch - createSpacesBatch

Parameters: none. Required request body: CreateSpacesBatchRequest containing at least one item. Response: 201 with CreateSpacesBatchResponse. Errors: 400 invalid/empty batch or duplicate IDs, 409 existing-space conflict, 500, 503. HTTP remains a single array-based request/reply even though the internal RPC uses client streaming.

### GET /v1/spaces/{spaceId} - getSpace

Parameters: required path spaceId. Request body: none. Response: 200 with ParkingSpace. Errors: 400 invalid request, 404 missing space, 500, 503. The dummy identifier `missing` returns 404; P15 returns an accessible available space.

### PUT /v1/spaces/{spaceId} - putSpace

Parameters: required path spaceId. Required request body: UpdateSpaceRequest. Responses: 200 with ParkingSpace for replacement, or 201 with ParkingSpace for creation. Errors: 400 invalid input, 409 reserved-space type change, 500, 503. The intended operation is idempotent replacement. The dummy created flag is true only for P99.

### PATCH /v1/spaces/{spaceId} - patchSpace

Parameters: required path spaceId. Required request body: UpdateSpaceRequest. Response: 200 with ParkingSpace. Errors: 400 invalid input, 404 missing space, 409 reserved-space conflict, 500, 503. Unlike PUT, PATCH is not intended to create a missing space. Both methods currently edit only the type field.

### DELETE /v1/spaces/{spaceId} - deleteSpace

Parameters: required path spaceId. Request body: none. Response: 204 with no body. Errors: 400 invalid request, 404 missing space, 409 reserved space, 500, 503. Only available spaces may be deleted in the domain design; Part I returns a dummy acknowledgement.

### Common validation

OpenAPI middleware runs before generated request decoding. It checks required-field presence, JSON shape, enum values, nonempty constrained strings, and nonempty batch arrays. A required boolean may be false but cannot be omitted. Per-operation application errors and infrastructure failures share the Problem schema.

<!-- page -->
# 3. REST API: reservations, statistics, and bodies

### POST /v1/reservations - createReservation

Parameters: none. Required body: CreateReservationRequest. Response: 201 with Reservation. Errors: 400 invalid request, 403 missing accessible permit, 404 requested space absent, 409 occupied space/no eligible space/vehicle already reserved, 500, 503. The current dummy code implements the P02 occupied case, P15 permit check, and `missing` case; it does not implement capacity search or active-plate tracking.

### GET /v1/reservations/{reservationId} - getReservation

Parameters: required string path reservationId. Request body: none. Response: 200 with Reservation. Errors: 400 invalid request, 404 missing/expired reservation, 500, 503. The implementation uses `missing` as its 404 trigger, not a real expiration lookup.

### DELETE /v1/reservations/{reservationId} - cancelReservation

Parameters: required string path reservationId. Request body: none. Response: 204 with no body. Errors: 400 invalid request, 404 missing/expired reservation, 500, 503. Cancellation and releasing the space are intended effects; no state is modified in Part I.

### GET /v1/parking/stats - getParkingStats

Parameters: none. Request body: none. Response: 200 with ParkingStats. Errors documented: 400 invalid query parameters, 500, 503. No query parameter is currently declared for this endpoint. Returned counts are fixed examples.

### Complete body definitions

All fields are required unless marked optional. The notation [] means an array, not an omitted schema. SpaceType is the string enum REGULAR|ACCESSIBLE; SpaceStatus is AVAILABLE|RESERVED.

| Schema | Fields and constraints |
| --- | --- |
| ParkingSpace | spaceId: nonempty string; type: SpaceType; status: SpaceStatus; reservationId: optional string. |
| Reservation | reservationId: string; spaceId: nonempty string; licensePlate: nonempty string; hasAccessiblePermit: boolean; createdAt and expiresAt: date-time strings. |
| ParkingStats | totalSpaces, availableSpaces, reservedSpaces, regularSpaces, accessibleSpaces: integers. |
| CreateSpaceRequest | spaceId: nonempty string; type: SpaceType. |
| UpdateSpaceRequest | type: SpaceType. |
| CreateReservationRequest | licensePlate: nonempty string; hasAccessiblePermit: boolean; spaceId: optional nonempty string. |
| CreateSpacesBatchRequest | spaces: array of CreateSpaceRequest; minimum one item. |
| CreateSpacesBatchResponse | spaces: array of ParkingSpace. |
| Problem | title: string; status: integer; optional type: string (schema default about:blank); optional detail: string. |

The error writer actually supplies title, status, and detail. Example success: `{"spaceId":"P15","type":"ACCESSIBLE","status":"AVAILABLE"}`. Example error: `{"title":"Not Found","status":404,"detail":"rpc error: code = NotFound desc = parking space not found"}`.

<!-- page -->
# 4. Internal API and status translation

The protobuf package is parking.v1 and the service is ParkingService. Each RPC has its own named request/response message. Generated message types live in parking.pb.go; client stubs and server/stream interfaces live in parking_grpc.pb.go. ParkingServer is the handwritten implementation.

| REST operation | RPC | Shape |
| --- | --- | --- |
| GET /v1/spaces | ListSpaces | Server stream |
| POST /v1/spaces | AddSpace | Unary |
| POST /v1/spaces/batch | CreateSpacesBatch | Client stream |
| GET /v1/spaces/{spaceId} | GetSpace | Unary |
| PUT /v1/spaces/{spaceId} | PutSpace | Unary |
| PATCH /v1/spaces/{spaceId} | PatchSpace | Unary |
| DELETE /v1/spaces/{spaceId} | DeleteSpace | Unary |
| POST /v1/reservations | CreateReservation | Unary |
| GET /v1/reservations/{reservationId} | GetReservation | Unary |
| DELETE /v1/reservations/{reservationId} | CancelReservation | Unary |
| GET /v1/parking/stats | GetParkingStats | Unary |

### RPC message design and streams

ListSpacesRequest has optional type/status enum filters. Each streamed ListSpacesResponse contains one space. The server sends each match; the HTTP adapter receives until EOF and builds one JSON array. This is a finite result stream, not a subscription.

Each CreateSpacesBatchRequest contains one space_id/type pair. The client sends all items, closes its sending side, and receives CreateSpacesBatchResponse with repeated spaces. The server checks every received item before returning success; its duplicate-ID map and result slice are request-local.

Get/DeleteSpace requests identify a space; Add/Put/Patch carry its ID and type. PutSpaceResponse also carries a created flag. Reservation requests carry a plate/permit/optional space ID or a reservation ID. GetParkingStatsRequest is empty. Success wrappers contain a space, reservation, or stats; delete/cancel replies are empty. Errors are gRPC statuses, not fields in these wrappers.

### Mapping gRPC failures to HTTP

| gRPC status | HTTP status |
| --- | --- |
| NotFound | 404 |
| InvalidArgument | 400 |
| PermissionDenied | 403 |
| AlreadyExists / FailedPrecondition | 409 |
| Unimplemented | 501 |
| Unavailable | 503 |
| Other failures | 500 |

The common httpStatusFromGRPC function performs this mapping. Validation/decoding errors return 400 directly. Problem Details provides the HTTP body. The 501 fallback exists in code but is not documented in the operation response lists; this is a known contract gap. The implemented RPCs normally avoid that fallback.

<!-- page -->
# 5. Design choices and implementation locations

### Separating contracts and responsibilities

We describe one service through two interfaces: OpenAPI for public HTTP/JSON and protobuf for internal RPC calls. The adapter translates between them without retaining parking state. This allows the communication structure to remain stable when actual stateful service behavior is introduced. Generated interfaces expose signature mismatches during compilation.

Spaces and reservations are separate resources because a space exists without a booking and reservations have their own identities and lifecycle. Reservation creation is the domain's read-modify-write operation: eligibility and availability must eventually be checked together with allocation. Part I only demonstrates fixed branches of that behavior.

### Features we consider strongest

**Bidirectional use of streaming patterns.** ListSpaces demonstrates server streaming and batch creation demonstrates client streaming. These handle variable-size operations while preserving familiar JSON arrays for REST clients. Batch validation rejects duplicates and conflicts before returning a successful batch, without claiming database atomicity.

**End-to-end validation and error handling.** OpenAPI middleware checks requests before typed decoding. Domain errors returned by gRPC reach HTTP clients through a common status mapping and Problem Details writer. This keeps transport conversion consistent across operations.

**A visible forwarding demonstration.** The second node forwards HTTP requests through its gRPC client to the first node. A reservation ID ending in n1 gives an observable processing-node marker; the demo asserts it rather than relying on server logs alone.

### Where to find the essential implementation

Line numbers are approximate for the inspected version; function names identify stable navigation points.

| Feature | Approximate location |
| --- | --- |
| Public API and schema constraints | api/openapi.yaml: paths; components/schemas |
| RPC contract and streams | proto/parking/v1/parking.proto:8, ParkingService |
| Server setup and target forwarding | cmd/server/main.go:31, main |
| HTTP adapter setup and error mapping | internal/httpapi/handlers.go:23, NewHTTPHandler; :52, httpStatusFromGRPC |
| Request validation | internal/httpapi/validation.go:14, validateRequests |
| List response stream | internal/grpcsvc/parking_server.go:72, ListSpaces; handlers.go:108 |
| Batch request stream | internal/grpcsvc/parking_server.go:142, CreateSpacesBatch; handlers.go:402 |
| Reservation dummy rules | internal/grpcsvc/parking_server.go:303, CreateReservation |
| Browser CORS support | cmd/server/main.go:116, withCORS; -cors-origin flag |
| Integration and CORS tests | internal/httpapi/handlers_test.go:22; cmd/server/main_test.go:10 |
| Two-process verification | etc/demo.sh; etc/check.sh; Makefile build/check targets |

Swagger UI/Editor are embedded through api/spec.go. Optional Docker tools use separate origins; configurable CORS handles browser preflights, including PUT. The custom parking dashboard is not part of the implementation.

<!-- page -->
# 6. Verification, omissions, and known issues

### Verification performed

REST/protobuf generation, go vet, Go tests, and executable build passed. HTTP integration tests use an in-memory gRPC server and cover validation, list/batch streams, common dummy failures, and Problem Details. Separate process checks ran the 40 demo cases against each of two servers: all 80 HTTP status checks passed. A reservation through node 2 returned R1001-n1, confirming forwarding to node 1.

Earlier live checks also exercised PUT CORS preflight, Problem Details media types/fields, and /docs, /editor, and /openapi.yaml. These are programmatic results; the report does not present them as screenshots or evidence of real booking correctness.

### Required-interface omission: consistency

The current API does not expose read-consistency selection, and protobuf requests have no consistency fields. The selectable read-consistency interface described by the template is not implemented in this version. No sequential or linearizable guarantee is claimed for any operation. The report records this omission instead of treating dummy responses as an implementation of consistency.

### Expected Part I limitations

No persistent state, real allocation, expiration scheduler, active-plate uniqueness tracking, replicated log, or consensus is implemented. Every write returns a dummy result. P01/P02/P15, the `missing` trigger, the P99 PUT-created case, fixed statistics, node-derived reservation IDs, and illustrative timestamps are fixtures. In particular, the no-capacity and duplicate-plate reservation errors are documented service cases but are not simulated by the current dummy server.

These limitations explain why a later GET need not reflect an earlier successful write. They also explain why the list contains three spaces while statistics report a larger fixed lot. This is dummy behavior, not evidence of a working state machine.

### Known discrepancies and likely causes

**Reserved-space PATCH behavior:** PatchSpace rejects P02 even when the requested type remains REGULAR, whereas the contract prohibits changing a reserved space's type. PutSpace allows that unchanged type. The likely cause is the unconditional P02 rejection in PatchSpace rather than a comparison with its dummy current type (parking_server.go:241).

**Validation differs by entry point:** REST middleware validates schemas, but several unary RPC methods do not repeat those checks for direct gRPC callers. For example, AddSpace can echo an invalid enum/empty ID from a direct RPC request. Validation was concentrated in the REST adapter, with additional explicit checks mainly in the batch method. Complete internal validation remains missing.

**Fallback contract gap:** Unimplemented maps to HTTP 501, but OpenAPI does not list 501 for these operations. That mapper was retained as a fallback. The normal implemented-call checks did not trigger it.

No major transport crash was observed in the executed checks. Those checks do not establish correctness under concurrent booking, server failure, or adversarial direct RPC inputs. The API description's word "replicated" is aspirational; current cross-node forwarding does not replicate state.
