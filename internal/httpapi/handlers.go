package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	parkingv1 "github.com/your-team/kvs/gen/parking/v1"
	"github.com/your-team/kvs/gen/rest"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Handlers implements the generated REST server interface.
// Each handler forwards requests to the parking gRPC service.
type Handlers struct {
	Parking parkingv1.ParkingServiceClient
}

func NewHTTPHandler(
	parking parkingv1.ParkingServiceClient,
	mux *http.ServeMux,
) http.Handler {
	strict := rest.NewStrictHandlerWithOptions(
		&Handlers{Parking: parking},
		nil,
		rest.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(
				w http.ResponseWriter,
				r *http.Request,
				err error,
			) {
				writeParkingError(w, http.StatusBadRequest, err.Error())
			},
			ResponseErrorHandlerFunc: func(
				w http.ResponseWriter,
				r *http.Request,
				err error,
			) {
				writeParkingError(w, httpStatusFromGRPC(err), err.Error())
			},
		},
	)

	return rest.HandlerFromMux(strict, mux)
}

// Convert gRPC errors into HTTP status codes.
func httpStatusFromGRPC(err error) int {
	switch status.Code(err) {
	case codes.NotFound:
		return http.StatusNotFound
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.AlreadyExists, codes.FailedPrecondition:
		return http.StatusConflict
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Write errors in the format specified by ParkingError.
func writeParkingError(
	w http.ResponseWriter,
	code int,
	message string,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(rest.ParkingError{
		Code:    code,
		Message: message,
	})
}

func (h *Handlers) GetSpace(
	ctx context.Context,
	req rest.GetSpaceRequestObject,
) (rest.GetSpaceResponseObject, error) {

	// Convert the requested consistency level to protobuf.
	consistency := parkingv1.Consistency_CONSISTENCY_SEQUENTIAL
	if req.Params.Consistency != nil &&
		string(*req.Params.Consistency) == "linearizable" {
		consistency = parkingv1.Consistency_CONSISTENCY_LINEARIZABLE
	}

	// Forward the REST request to the gRPC service.
	result, err := h.Parking.GetSpace(ctx, &parkingv1.GetSpaceRequest{
		SpaceId:     req.SpaceId,
		Consistency: consistency,
	})
	if err != nil {
		return nil, err
	}

	// Convert the protobuf response into the REST response.
	space := result.GetSpace()

	return rest.GetSpace200JSONResponse(
		parkingSpaceFromProto(space),
	), nil
}

func (h *Handlers) ListSpaces(
	ctx context.Context,
	req rest.ListSpacesRequestObject,
) (rest.ListSpacesResponseObject, error) {

	consistency := parkingv1.Consistency_CONSISTENCY_SEQUENTIAL
	if req.Params.Consistency != nil &&
		string(*req.Params.Consistency) == "linearizable" {
		consistency = parkingv1.Consistency_CONSISTENCY_LINEARIZABLE
	}

	grpcReq := &parkingv1.ListSpacesRequest{
		Consistency: consistency,
	}

	// Optional space-type filter.
	if req.Params.Type != nil {
		t := parkingv1.SpaceType_SPACE_TYPE_REGULAR
		if string(*req.Params.Type) == "ACCESSIBLE" {
			t = parkingv1.SpaceType_SPACE_TYPE_ACCESSIBLE
		}
		grpcReq.Type = &t
	}

	// Optional status filter.
	if req.Params.Status != nil {
		st := parkingv1.SpaceStatus_SPACE_STATUS_AVAILABLE
		if string(*req.Params.Status) == "RESERVED" {
			st = parkingv1.SpaceStatus_SPACE_STATUS_RESERVED
		}
		grpcReq.Status = &st
	}

	result, err := h.Parking.ListSpaces(ctx, grpcReq)
	if err != nil {
		return nil, err
	}

	spaces := make(rest.ListSpaces200JSONResponse, 0)

	for _, space := range result.GetSpaces() {
		spaces = append(spaces, parkingSpaceFromProto(space))
	}

	return spaces, nil
}

// POST /v1/spaces
// Creates a new REGULAR or ACCESSIBLE parking space.
func (h *Handlers) CreateSpace(
	ctx context.Context,
	req rest.CreateSpaceRequestObject,
) (rest.CreateSpaceResponseObject, error) {

	spaceType := parkingv1.SpaceType_SPACE_TYPE_REGULAR
	if req.Body.Type == rest.CreateSpaceRequestTypeACCESSIBLE {
		spaceType = parkingv1.SpaceType_SPACE_TYPE_ACCESSIBLE
	}

	result, err := h.Parking.AddSpace(ctx, &parkingv1.AddSpaceRequest{
		SpaceId: req.Body.SpaceId,
		Type:    spaceType,
	})
	if err != nil {
		return nil, err
	}

	return rest.CreateSpace201JSONResponse(
		parkingSpaceFromProto(result.GetSpace()),
	), nil
}

// PUT /v1/spaces/{spaceId}
// Creates a space or replaces its configuration.
func (h *Handlers) PutSpace(
	ctx context.Context,
	req rest.PutSpaceRequestObject,
) (rest.PutSpaceResponseObject, error) {

	spaceType := parkingv1.SpaceType_SPACE_TYPE_REGULAR
	if req.Body.Type == rest.UpdateSpaceRequestTypeACCESSIBLE {
		spaceType = parkingv1.SpaceType_SPACE_TYPE_ACCESSIBLE
	}

	result, err := h.Parking.PutSpace(ctx, &parkingv1.PutSpaceRequest{
		SpaceId: req.SpaceId,
		Type:    spaceType,
	})
	if err != nil {
		return nil, err
	}

	space := parkingSpaceFromProto(result.GetSpace())

	if result.GetCreated() {
		return rest.PutSpace201JSONResponse(space), nil
	}

	return rest.PutSpace200JSONResponse(space), nil
}

func (h *Handlers) PatchSpace(
	ctx context.Context,
	request rest.PatchSpaceRequestObject,
) (rest.PatchSpaceResponseObject, error) {

	// Convert the REST space type to the protobuf enum.

	var spaceType parkingv1.SpaceType

	switch request.Body.Type {
	case rest.UpdateSpaceRequestTypeREGULAR:
		spaceType = parkingv1.SpaceType_SPACE_TYPE_REGULAR

	case rest.UpdateSpaceRequestTypeACCESSIBLE:
		spaceType = parkingv1.SpaceType_SPACE_TYPE_ACCESSIBLE

	default:
		return nil, status.Error(
			codes.InvalidArgument,
			"type must be REGULAR or ACCESSIBLE",
		)
	}

	// Forward the PATCH request to the gRPC server.
	response, err := h.Parking.PatchSpace(
		ctx,
		&parkingv1.PatchSpaceRequest{
			SpaceId: request.SpaceId,
			Type:    spaceType,
		},
	)

	if err != nil {
		return nil, err
	}

	// Convert the protobuf response to the REST response.
	return rest.PatchSpace200JSONResponse(
		parkingSpaceFromProto(response.GetSpace()),
	), nil
}

// DELETE /v1/spaces/{spaceId}
// Removes an available parking space.
func (h *Handlers) DeleteSpace(
	ctx context.Context,
	req rest.DeleteSpaceRequestObject,
) (rest.DeleteSpaceResponseObject, error) {

	_, err := h.Parking.DeleteSpace(ctx, &parkingv1.DeleteSpaceRequest{
		SpaceId: req.SpaceId,
	})
	if err != nil {
		return nil, err
	}

	return rest.DeleteSpace204Response{}, nil
}

// Convert a protobuf ParkingSpace into a REST ParkingSpace.
func parkingSpaceFromProto(
	space *parkingv1.ParkingSpace,
) rest.ParkingSpace {

	result := rest.ParkingSpace{
		SpaceId: space.GetSpaceId(),
		Type:    rest.ParkingSpaceTypeREGULAR,
		Status:  rest.ParkingSpaceStatusAVAILABLE,
	}

	if space.GetType() == parkingv1.SpaceType_SPACE_TYPE_ACCESSIBLE {
		result.Type = rest.ParkingSpaceTypeACCESSIBLE
	}

	if space.GetStatus() == parkingv1.SpaceStatus_SPACE_STATUS_RESERVED {
		result.Status = rest.ParkingSpaceStatusRESERVED
	}

	if space.ReservationId != nil {
		id := space.GetReservationId()
		result.ReservationId = &id
	}

	return result
}

// Converts a protobuf Reservation to a REST Reservation.

func reservationFromProto(r *parkingv1.Reservation) (rest.Reservation, error) {
	createdAt, err := time.Parse(time.RFC3339, r.GetCreatedAt())
	if err != nil {
		return rest.Reservation{}, err
	}

	expiresAt, err := time.Parse(time.RFC3339, r.GetExpiresAt())
	if err != nil {
		return rest.Reservation{}, err
	}

	return rest.Reservation{
		ReservationId:       r.GetReservationId(),
		SpaceId:             r.GetSpaceId(),
		LicensePlate:        r.GetLicensePlate(),
		HasAccessiblePermit: r.GetHasAccessiblePermit(),
		CreatedAt:           createdAt,
		ExpiresAt:           expiresAt,
	}, nil
}

// POST /v1/reservations
func (h *Handlers) CreateReservation(
	ctx context.Context,
	req rest.CreateReservationRequestObject,
) (rest.CreateReservationResponseObject, error) {

	grpcReq := &parkingv1.CreateReservationRequest{
		LicensePlate:        req.Body.LicensePlate,
		HasAccessiblePermit: req.Body.HasAccessiblePermit,
		SpaceId:             req.Body.SpaceId,
	}

	result, err := h.Parking.CreateReservation(ctx, grpcReq)
	if err != nil {
		return nil, err
	}

	reservation, err := reservationFromProto(result.GetReservation())
	if err != nil {
		return nil, err
	}

	return rest.CreateReservation201JSONResponse(reservation), nil
}

// GET /v1/reservations/{reservationId}
func (h *Handlers) GetReservation(
	ctx context.Context,
	req rest.GetReservationRequestObject,
) (rest.GetReservationResponseObject, error) {

	consistency := parkingv1.Consistency_CONSISTENCY_SEQUENTIAL
	if req.Params.Consistency != nil &&
		string(*req.Params.Consistency) == "linearizable" {
		consistency = parkingv1.Consistency_CONSISTENCY_LINEARIZABLE
	}

	result, err := h.Parking.GetReservation(ctx, &parkingv1.GetReservationRequest{
		ReservationId: req.ReservationId,
		Consistency:   consistency,
	})
	if err != nil {
		return nil, err
	}

	reservation, err := reservationFromProto(result.GetReservation())
	if err != nil {
		return nil, err
	}

	return rest.GetReservation200JSONResponse(reservation), nil
}

// DELETE /v1/reservations/{reservationId}
func (h *Handlers) CancelReservation(
	ctx context.Context,
	req rest.CancelReservationRequestObject,
) (rest.CancelReservationResponseObject, error) {

	_, err := h.Parking.CancelReservation(ctx, &parkingv1.CancelReservationRequest{
		ReservationId: req.ReservationId,
	})
	if err != nil {
		return nil, err
	}

	return rest.CancelReservation204Response{}, nil
}

// GET /v1/parking/stats
func (h *Handlers) GetParkingStats(
	ctx context.Context,
	req rest.GetParkingStatsRequestObject,
) (rest.GetParkingStatsResponseObject, error) {

	consistency := parkingv1.Consistency_CONSISTENCY_SEQUENTIAL
	if req.Params.Consistency != nil &&
		string(*req.Params.Consistency) == "linearizable" {
		consistency = parkingv1.Consistency_CONSISTENCY_LINEARIZABLE
	}

	result, err := h.Parking.GetParkingStats(ctx, &parkingv1.GetParkingStatsRequest{
		Consistency: consistency,
	})
	if err != nil {
		return nil, err
	}

	stats := result.GetStats()

	return rest.GetParkingStats200JSONResponse{
		TotalSpaces:      int(stats.GetTotalSpaces()),
		AvailableSpaces:  int(stats.GetAvailableSpaces()),
		ReservedSpaces:   int(stats.GetReservedSpaces()),
		RegularSpaces:    int(stats.GetRegularSpaces()),
		AccessibleSpaces: int(stats.GetAccessibleSpaces()),
	}, nil
}

func (h *Handlers) CreateSpacesBatch(
	ctx context.Context,
	request rest.CreateSpacesBatchRequestObject,
) (rest.CreateSpacesBatchResponseObject, error) {

	if request.Body == nil || len(request.Body.Spaces) == 0 {
		return nil, status.Error(
			codes.InvalidArgument,
			"batch must contain at least one parking space",
		)
	}

	// Convert REST items to protobuf messages.
	items := make([]*parkingv1.CreateSpacesBatchItem, 0, len(request.Body.Spaces))

	for _, space := range request.Body.Spaces {
		var spaceType parkingv1.SpaceType

		switch space.Type {
		case rest.CreateSpaceRequestTypeREGULAR:
			spaceType = parkingv1.SpaceType_SPACE_TYPE_REGULAR

		case rest.CreateSpaceRequestTypeACCESSIBLE:
			spaceType = parkingv1.SpaceType_SPACE_TYPE_ACCESSIBLE

		default:
			return nil, status.Error(
				codes.InvalidArgument,
				"type must be REGULAR or ACCESSIBLE",
			)
		}

		items = append(items, &parkingv1.CreateSpacesBatchItem{
			SpaceId: space.SpaceId,
			Type:    spaceType,
		})
	}

	// Forward the complete batch in ONE gRPC request.
	response, err := h.Parking.CreateSpacesBatch(
		ctx,
		&parkingv1.CreateSpacesBatchRequest{
			Spaces: items,
		},
	)

	if err != nil {
		return nil, err
	}

	// Convert protobuf results back to REST objects.
	createdSpaces := make([]rest.ParkingSpace, 0, len(response.GetSpaces()))

	for _, space := range response.GetSpaces() {
		createdSpaces = append(createdSpaces, parkingSpaceFromProto(space))
	}

	return rest.CreateSpacesBatch201JSONResponse{
		Spaces: createdSpaces,
	}, nil
}
