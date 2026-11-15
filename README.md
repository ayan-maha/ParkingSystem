# Smart Parking System — Distributed Systems, Part I

## Overview

Smart Parking is a distributed-systems application designed to manage parking spaces and reservations through a REST API.

Part I implements the communication infrastructure using OpenAPI 3.0, generated Go HTTP interfaces, Protocol Buffers, and gRPC. The application supports multiple server nodes and forwards REST requests to a designated gRPC server.

The current implementation uses predefined dummy responses. Persistent application state, replicated logs, and consensus are outside the scope of Part I.

## Features

- Create, retrieve, update, delete, and list parking spaces.
- Support REGULAR and ACCESSIBLE parking spaces.
- Create manual reservations for specific spaces.
- Request automatic space assignment.
- Restrict accessible parking reservations to permit holders.
- Retrieve and cancel reservations.
- Retrieve parking statistics.
- Request sequential or linearizable read consistency.
- Map gRPC errors to HTTP status codes.
- Create multiple parking spaces in one batch request.
- Partially update existing parking spaces using PATCH.

## Technologies

- Go 1.26 or later
- OpenAPI 3.0
- oapi-codegen
- Protocol Buffers
- gRPC
- Make
- curl

## Project Structure

```text
api/                     OpenAPI specification and generator configuration
proto/parking/v1/        Parking gRPC service definition
gen/rest/                Generated REST interfaces
gen/parking/v1/          Generated protobuf messages and gRPC stubs
internal/httpapi/        REST handlers and gRPC forwarding
internal/grpcsvc/        Dummy parking service implementation
cmd/server/              Application entry point
etc/demo.sh              REST demonstration script
etc/check.sh             Automated build and integration verification
Makefile                 Build, generation, and execution commands
```

## Build

From the project root:

```bash
make setup
make build
```

`make setup` installs the Go code-generation tools. `make build` regenerates the REST and gRPC sources, runs `go vet`, and builds `bin/server`.

## Run a Single Server

```bash
make run
```

This starts node `n1` with:

- REST: `localhost:8080`
- gRPC: `localhost:50051`

## Run Two Servers

In terminal 1:

```bash
make run
```

In terminal 2:

```bash
make run2
```

Node `n2` listens on REST port `8081` and gRPC port `50052`. Its REST handlers forward requests to node `n1` through gRPC on port `50051`.

## REST API
Method	Path	Description
GET	/v1/spaces	List parking spaces
POST	/v1/spaces	Create a parking space
POST	/v1/spaces/batch	Create multiple parking spaces
GET	/v1/spaces/{spaceId}	Retrieve a parking space
PUT	/v1/spaces/{spaceId}	Create or replace a parking space
PATCH	/v1/spaces/{spaceId}	Partially update a parking space
DELETE	/v1/spaces/{spaceId}	Delete a parking space
POST	/v1/reservations	Create a reservation
GET	/v1/reservations/{reservationId}	Retrieve a reservation
DELETE	/v1/reservations/{reservationId}	Cancel a reservation
GET	/v1/parking/stats	Retrieve parking statistics

The full REST contract is defined in `api/openapi.yaml`.

## Example Request

```bash
curl -i -X POST http://localhost:8080/v1/reservations \
  -H "Content-Type: application/json" \
  -d '{"licensePlate":"1234567","hasAccessiblePermit":true,"spaceId":"P15"}'
```

A successful dummy reservation returns HTTP `201 Created` with a reservation identifier such as `R1001-n1`, identifying the gRPC node that processed the request.

## Verification

With both servers running:

```bash
make demo
```

This performs 66 REST HTTP status checks across both servers.

For a self-contained build and integration check:

```bash
make check
```

This builds the project, starts two temporary server processes, runs the REST demonstration against both nodes, verifies forwarding to node `n1`, and stops the servers.

## Architecture

```text
Client
  |
  | HTTP / JSON
  v
REST Server (OpenAPI-generated interface)
  |
  | gRPC / Protocol Buffers
  v
ParkingService
  |
  v
Dummy responses
```

When using two servers, the REST layer of node `n2` calls the ParkingService gRPC endpoint of node `n1`.

## Consistency and Part I Limitations

The API exposes sequential and linearizable read-consistency options for future implementation. Part I accepts these parameters but does not enforce distributed consistency.

All write requests return predefined dummy responses and do not change persistent state. Reservation IDs and timestamps are illustrative rather than dynamically generated. Automatic expiration, actual parking allocation, concurrency control, state-machine execution, replicated logs, and consensus are deferred to later assignment parts.

## HTTP Error Mapping

| gRPC status | HTTP status |
|---|---|
| NotFound | 404 |
| InvalidArgument | 400 |
| PermissionDenied | 403 |
| AlreadyExists | 409 |
| FailedPrecondition | 409 |
| Unimplemented | 501 |
| Unavailable | 503 |
| Other internal errors | 500 |

## API Documentation

With the REST server running:

- `/docs` — Swagger UI
- `/editor` — Swagger Editor
- `/openapi.yaml` — OpenAPI specification

The project also includes optional Docker-based API documentation tools, configured in `docker/api-tools.compose.yaml`.