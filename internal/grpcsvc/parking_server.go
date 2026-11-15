package grpcsvc

import (
	"context"

	parkingv1 "github.com/your-team/kvs/gen/parking/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ParkingServer implements our Smart Parking gRPC service.
type ParkingServer struct {
	parkingv1.UnimplementedParkingServiceServer

	NodeID string
}

// NewParking creates a parking server with a node identifier.
func NewParking(nodeID string) *ParkingServer {
	return &ParkingServer{NodeID: nodeID}
}

// identifyServer sends the responding server's ID as gRPC metadata.
func (s *ParkingServer) identifyServer(ctx context.Context) {
	_ = grpc.SetHeader(ctx, metadata.Pairs("x-node-id", s.NodeID))
}

// GetSpace returns dummy parking-space information.
func (s *ParkingServer) GetSpace(
	ctx context.Context,
	req *parkingv1.GetSpaceRequest,
) (*parkingv1.GetSpaceResponse, error) {

	s.identifyServer(ctx)

	// Simulate a nonexistent parking space.
	if req.GetSpaceId() == "missing" {
		return nil, status.Error(
			codes.NotFound,
			"parking space not found",
		)
	}

	// Return dummy information.
	spaceType := parkingv1.SpaceType_SPACE_TYPE_REGULAR
	spaceStatus := parkingv1.SpaceStatus_SPACE_STATUS_AVAILABLE

	if req.GetSpaceId() == "P15" {
		spaceType = parkingv1.SpaceType_SPACE_TYPE_ACCESSIBLE
	}

	if req.GetSpaceId() == "P02" {
		spaceStatus = parkingv1.SpaceStatus_SPACE_STATUS_RESERVED
	}

	space := &parkingv1.ParkingSpace{
		SpaceId: req.GetSpaceId(),
		Type:    spaceType,
		Status:  spaceStatus,
	}

	return &parkingv1.GetSpaceResponse{
		Space: space,
	}, nil
}

// ListSpaces returns a dummy list of parking spaces.
func (s *ParkingServer) ListSpaces(
	ctx context.Context,
	req *parkingv1.ListSpacesRequest,
) (*parkingv1.ListSpacesResponse, error) {
	s.identifyServer(ctx)

	spaces := []*parkingv1.ParkingSpace{
		{
			SpaceId: "P01",
			Type:    parkingv1.SpaceType_SPACE_TYPE_REGULAR,
			Status:  parkingv1.SpaceStatus_SPACE_STATUS_AVAILABLE,
		},
		{
			SpaceId: "P02",
			Type:    parkingv1.SpaceType_SPACE_TYPE_REGULAR,
			Status:  parkingv1.SpaceStatus_SPACE_STATUS_RESERVED,
		},
		{
			SpaceId: "P15",
			Type:    parkingv1.SpaceType_SPACE_TYPE_ACCESSIBLE,
			Status:  parkingv1.SpaceStatus_SPACE_STATUS_AVAILABLE,
		},
	}

	// Apply optional filters to dummy data.
	filtered := make([]*parkingv1.ParkingSpace, 0)

	for _, space := range spaces {
		if req.Type != nil && space.Type != req.GetType() {
			continue
		}

		if req.Status != nil && space.Status != req.GetStatus() {
			continue
		}

		filtered = append(filtered, space)
	}

	return &parkingv1.ListSpacesResponse{
		Spaces: filtered,
	}, nil
}

// AddSpace returns a dummy newly created parking space.
func (s *ParkingServer) AddSpace(
	ctx context.Context,
	req *parkingv1.AddSpaceRequest,
) (*parkingv1.AddSpaceResponse, error) {
	s.identifyServer(ctx)

	if req.GetSpaceId() == "P02" {
		return nil, status.Error(
			codes.AlreadyExists,
			"parking space already exists",
		)
	}

	space := &parkingv1.ParkingSpace{
		SpaceId: req.GetSpaceId(),
		Type:    req.GetType(),
		Status:  parkingv1.SpaceStatus_SPACE_STATUS_AVAILABLE,
	}

	return &parkingv1.AddSpaceResponse{
		Space: space,
	}, nil
}

func (s *ParkingServer) CreateSpacesBatch(
	ctx context.Context,
	req *parkingv1.CreateSpacesBatchRequest,
) (*parkingv1.CreateSpacesBatchResponse, error) {
	s.identifyServer(ctx)

	// A batch must contain at least one space.
	if len(req.GetSpaces()) == 0 {
		return nil, status.Error(
			codes.InvalidArgument,
			"batch must contain at least one parking space",
		)
	}

	seen := make(map[string]bool)
	spaces := make([]*parkingv1.ParkingSpace, 0, len(req.GetSpaces()))

	// Validate every item before returning a successful response.
	for _, item := range req.GetSpaces() {
		id := item.GetSpaceId()

		if id == "" {
			return nil, status.Error(
				codes.InvalidArgument,
				"parking space ID is required",
			)
		}

		if item.GetType() != parkingv1.SpaceType_SPACE_TYPE_REGULAR &&
			item.GetType() != parkingv1.SpaceType_SPACE_TYPE_ACCESSIBLE {
			return nil, status.Error(
				codes.InvalidArgument,
				"type must be REGULAR or ACCESSIBLE",
			)
		}

		if seen[id] {
			return nil, status.Error(
				codes.InvalidArgument,
				"duplicate parking space ID in batch",
			)
		}
		seen[id] = true

		// Dummy conflict for an existing parking space.
		if id == "P02" {
			return nil, status.Error(
				codes.AlreadyExists,
				"parking space already exists",
			)
		}

		spaces = append(spaces, &parkingv1.ParkingSpace{
			SpaceId: id,
			Type:    item.GetType(),
			Status:  parkingv1.SpaceStatus_SPACE_STATUS_AVAILABLE,
		})
	}

	// Part I: return dummy objects without saving them.
	return &parkingv1.CreateSpacesBatchResponse{
		Spaces: spaces,
	}, nil
}

// PutSpace returns a dummy created or updated parking space.
func (s *ParkingServer) PutSpace(
	ctx context.Context,
	req *parkingv1.PutSpaceRequest,
) (*parkingv1.PutSpaceResponse, error) {
	s.identifyServer(ctx)

	// Simulate a conflict when changing a reserved space.
	if req.GetSpaceId() == "P02" &&
		req.GetType() != parkingv1.SpaceType_SPACE_TYPE_REGULAR {
		return nil, status.Error(
			codes.FailedPrecondition,
			"cannot change the type of a reserved space",
		)
	}

	created := req.GetSpaceId() == "P99"

	space := &parkingv1.ParkingSpace{
		SpaceId: req.GetSpaceId(),
		Type:    req.GetType(),
		Status:  parkingv1.SpaceStatus_SPACE_STATUS_AVAILABLE,
	}

	if req.GetSpaceId() == "P02" {
		space.Status = parkingv1.SpaceStatus_SPACE_STATUS_RESERVED
	}

	return &parkingv1.PutSpaceResponse{
		Space:   space,
		Created: created,
	}, nil
}

func (s *ParkingServer) PatchSpace(
	ctx context.Context,
	req *parkingv1.PatchSpaceRequest,
) (*parkingv1.PatchSpaceResponse, error) {
	s.identifyServer(ctx)

	// Part I: dummy responses only; no persistent changes.
	if req.GetSpaceId() == "missing" {
		return nil, status.Error(codes.NotFound, "parking space not found")
	}

	if req.GetSpaceId() == "P02" {
		return nil, status.Error(
			codes.FailedPrecondition,
			"cannot modify a reserved parking space",
		)
	}

	if req.GetType() == parkingv1.SpaceType_SPACE_TYPE_UNSPECIFIED {
		return nil, status.Error(
			codes.InvalidArgument,
			"parking space type is required",
		)
	}

	space := &parkingv1.ParkingSpace{
		SpaceId: req.GetSpaceId(),
		Type:    req.GetType(),
		Status:  parkingv1.SpaceStatus_SPACE_STATUS_AVAILABLE,
	}

	return &parkingv1.PatchSpaceResponse{
		Space: space,
	}, nil
}

// DeleteSpace returns a dummy successful deletion.
func (s *ParkingServer) DeleteSpace(
	ctx context.Context,
	req *parkingv1.DeleteSpaceRequest,
) (*parkingv1.DeleteSpaceResponse, error) {
	s.identifyServer(ctx)

	if req.GetSpaceId() == "missing" {
		return nil, status.Error(
			codes.NotFound,
			"parking space not found",
		)
	}

	if req.GetSpaceId() == "P02" {
		return nil, status.Error(
			codes.FailedPrecondition,
			"cannot delete a reserved parking space",
		)
	}

	return &parkingv1.DeleteSpaceResponse{}, nil
}

// CreateReservation returns a dummy reservation.
// No actual parking state is changed in Part I.
func (s *ParkingServer) CreateReservation(
	ctx context.Context,
	req *parkingv1.CreateReservationRequest,
) (*parkingv1.CreateReservationResponse, error) {
	s.identifyServer(ctx)

	spaceID := "P01"

	// Manual reservation: use the requested space.
	if req.SpaceId != nil {
		spaceID = req.GetSpaceId()
	}

	// Simulate a nonexistent parking space.
	if spaceID == "missing" {
		return nil, status.Error(
			codes.NotFound,
			"parking space not found",
		)
	}

	// P02 is reserved in our dummy dataset.
	if spaceID == "P02" {
		return nil, status.Error(
			codes.AlreadyExists,
			"parking space already reserved",
		)
	}

	// P15 represents an accessible parking space.
	if spaceID == "P15" && !req.GetHasAccessiblePermit() {
		return nil, status.Error(
			codes.PermissionDenied,
			"accessible parking permit required",
		)
	}

	// Fixed dummy values, not real reservation state.
	reservation := &parkingv1.Reservation{
		ReservationId:       "R1001-" + s.NodeID,
		SpaceId:             spaceID,
		LicensePlate:        req.GetLicensePlate(),
		HasAccessiblePermit: req.GetHasAccessiblePermit(),
		CreatedAt:           "2026-11-14T10:00:00Z",
		ExpiresAt:           "2026-11-14T10:30:00Z",
	}

	return &parkingv1.CreateReservationResponse{
		Reservation: reservation,
	}, nil
}

// GetReservation returns dummy reservation information.
func (s *ParkingServer) GetReservation(
	ctx context.Context,
	req *parkingv1.GetReservationRequest,
) (*parkingv1.GetReservationResponse, error) {
	s.identifyServer(ctx)

	if req.GetReservationId() == "missing" {
		return nil, status.Error(
			codes.NotFound,
			"reservation not found",
		)
	}

	reservation := &parkingv1.Reservation{
		ReservationId:       req.GetReservationId(),
		SpaceId:             "P15",
		LicensePlate:        "1234567",
		HasAccessiblePermit: true,
		CreatedAt:           "2026-11-14T10:00:00Z",
		ExpiresAt:           "2026-11-14T10:30:00Z",
	}

	return &parkingv1.GetReservationResponse{
		Reservation: reservation,
	}, nil
}

// CancelReservation returns a dummy successful cancellation.
func (s *ParkingServer) CancelReservation(
	ctx context.Context,
	req *parkingv1.CancelReservationRequest,
) (*parkingv1.CancelReservationResponse, error) {
	s.identifyServer(ctx)

	if req.GetReservationId() == "missing" {
		return nil, status.Error(
			codes.NotFound,
			"reservation not found",
		)
	}

	return &parkingv1.CancelReservationResponse{}, nil
}

// GetParkingStats returns fixed dummy statistics.
func (s *ParkingServer) GetParkingStats(
	ctx context.Context,
	req *parkingv1.GetParkingStatsRequest,
) (*parkingv1.GetParkingStatsResponse, error) {
	s.identifyServer(ctx)

	return &parkingv1.GetParkingStatsResponse{
		Stats: &parkingv1.ParkingStats{
			TotalSpaces:      100,
			AvailableSpaces:  25,
			ReservedSpaces:   75,
			RegularSpaces:    90,
			AccessibleSpaces: 10,
		},
	}, nil
}
