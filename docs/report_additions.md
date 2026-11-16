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
# Service model and parking rules

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
# Design decisions and visual presentation

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
# Worked examples and observable behavior

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
# Coverage, evidence, and scope

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
