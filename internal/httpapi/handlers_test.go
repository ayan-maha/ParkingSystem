package httpapi_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	parkingv1 "github.com/your-team/kvs/gen/parking/v1"
	"github.com/your-team/kvs/internal/grpcsvc"
	"github.com/your-team/kvs/internal/httpapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// Exercise the generated REST layer, validation, and real gRPC streams together.
func TestParkingAPI(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	parkingv1.RegisterParkingServiceServer(server, grpcsvc.NewParking("n1"))
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(func() { server.Stop(); _ = lis.Close() })
	conn, err := grpc.NewClient("passthrough:///parking", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	handler := httpapi.NewHTTPHandler(parkingv1.NewParkingServiceClient(conn), http.NewServeMux())

	cases := []struct {
		name, method, path, body string
		status                   int
		contains                 string
	}{
		{"list stream", "GET", "/v1/spaces", "", 200, `"P15"`},
		{"filter stream", "GET", "/v1/spaces?type=ACCESSIBLE", "", 200, `"P15"`},
		{"empty stream", "GET", "/v1/spaces?type=ACCESSIBLE&status=RESERVED", "", 200, `[]`},
		{"invalid filter", "GET", "/v1/spaces?type=INVALID", "", 400, ""},
		{"invalid status", "GET", "/v1/spaces?status=INVALID", "", 400, ""},
		{"missing space", "GET", "/v1/spaces/missing", "", 404, ""},
		{"create", "POST", "/v1/spaces", `{"spaceId":"P25","type":"REGULAR"}`, 201, `"P25"`},
		{"create conflict", "POST", "/v1/spaces", `{"spaceId":"P02","type":"REGULAR"}`, 409, ""},
		{"create invalid enum", "POST", "/v1/spaces", `{"spaceId":"P25","type":"INVALID"}`, 400, ""},
		{"create missing type", "POST", "/v1/spaces", `{"spaceId":"P25"}`, 400, ""},
		{"create empty id", "POST", "/v1/spaces", `{"spaceId":"","type":"REGULAR"}`, 400, ""},
		{"null body", "POST", "/v1/spaces", `null`, 400, ""},
		{"missing body", "POST", "/v1/spaces", "", 400, ""},
		{"malformed body", "POST", "/v1/spaces", `{`, 400, ""},
		{"batch stream", "POST", "/v1/spaces/batch", `{"spaces":[{"spaceId":"P30","type":"REGULAR"},{"spaceId":"P31","type":"ACCESSIBLE"}]}`, 201, `"P31"`},
		{"empty batch", "POST", "/v1/spaces/batch", `{"spaces":[]}`, 400, ""},
		{"duplicate batch", "POST", "/v1/spaces/batch", `{"spaces":[{"spaceId":"P30","type":"REGULAR"},{"spaceId":"P30","type":"ACCESSIBLE"}]}`, 400, ""},
		{"batch conflict", "POST", "/v1/spaces/batch", `{"spaces":[{"spaceId":"P02","type":"REGULAR"}]}`, 409, ""},
		{"put invalid enum", "PUT", "/v1/spaces/P01", `{"type":"INVALID"}`, 400, ""},
		{"put missing type", "PUT", "/v1/spaces/P01", `{}`, 400, ""},
		{"put success", "PUT", "/v1/spaces/P01", `{"type":"ACCESSIBLE"}`, 200, `"ACCESSIBLE"`},
		{"patch", "PATCH", "/v1/spaces/P01", `{"type":"ACCESSIBLE"}`, 200, `"ACCESSIBLE"`},
		{"patch missing", "PATCH", "/v1/spaces/missing", `{"type":"REGULAR"}`, 404, ""},
		{"delete", "DELETE", "/v1/spaces/P01", "", 204, ""},
		{"forwarded reservation", "POST", "/v1/reservations", `{"licensePlate":"123","hasAccessiblePermit":false}`, 201, `"R1001-n1"`},
		{"permit required field", "POST", "/v1/reservations", `{"licensePlate":"123"}`, 400, ""},
		{"empty plate", "POST", "/v1/reservations", `{"licensePlate":"","hasAccessiblePermit":false}`, 400, ""},
		{"permit forbidden", "POST", "/v1/reservations", `{"licensePlate":"123","hasAccessiblePermit":false,"spaceId":"P15"}`, 403, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.contains) {
				t.Fatalf("response missing %q: %s", tc.contains, w.Body.String())
			}
			if tc.status >= 400 {
				var problem struct {
					Title  string
					Status int
					Detail string
				}
				if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if w.Header().Get("Content-Type") != "application/problem+json" || problem.Status != tc.status || problem.Title == "" || problem.Detail == "" {
					t.Fatalf("invalid Problem Details response: %s", w.Body.String())
				}
			}
		})
	}
}
