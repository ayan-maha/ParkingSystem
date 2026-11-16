package httpapi

import (
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/your-team/kvs/api"
)

// Validate before decoding into Go structs: decoding alone cannot distinguish
// omitted required fields from zero values, and does not enforce enums.
func validateRequests(next http.Handler) http.Handler {
	doc, err := openapi3.NewLoader().LoadFromData(api.Spec)
	if err != nil {
		panic(err)
	}
	// Validation must work on both nodes regardless of their hostname or port.
	doc.Servers = nil
	router, err := legacy.NewRouter(doc)
	if err != nil {
		panic(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, params, err := router.FindRoute(r)
		if err != nil {
			// Documentation routes and unmatched paths belong to ServeMux.
			next.ServeHTTP(w, r)
			return
		}
		if err := openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: params,
			Route:      route,
		}); err != nil {
			writeParkingError(w, http.StatusBadRequest, err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}
