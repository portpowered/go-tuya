package tuya

import (
	"context"
	"crypto/md5" // #nosec G501 -- synthetic route tests reproduce Tuya's protocol-mandated request-key derivation.
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	routeFixtureCode                        = "code"
	routeFixtureDevice1                     = "device-1"
	routeFixtureHome1                       = "home-1"
	routeFixtureSwitch                      = "switch"
	routeFixtureSyntheticAccess             = "synthetic-access"
	routeFixtureUser2                       = "user-2"
	routeFixtureV10DevicesDevice1           = "/v1.0/devices/device-1"
	routeFixtureV10DevicesDevice1UsersUser2 = "/v1.0/devices/device-1/users/user-2"
	routeFixtureV10MLifeHaHomeDevices       = "/v1.0/m/life/ha/home/devices"
)

// These hand-authored responses exercise the public operation mapping and
// request routes. They are synthetic and are not Tuya account captures.
//
//nolint:cyclop,funlen,gocognit,gocyclo,maintidx // The exhaustive route matrix keeps each route paired with its response assertion.
func TestSyntheticDeviceAndHomeRoutes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	device := map[string]any{"id": routeFixtureDevice1, "name": "Desk lamp", "category": "dj"}

	tests := []struct {
		name   string
		method string
		path   string
		result any
		run    func(*testing.T, *Session)
	}{
		{"homes", http.MethodGet, "/v1.0/m/life/users/homes", []any{map[string]any{"ownerId": routeFixtureHome1, "name": "Home"}}, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.HomeService.QueryHomes(ctx, QueryHomesRequest{})
			if err != nil || len(got.Results) != 1 || got.Results[0].ID != routeFixtureHome1 {
				t.Fatalf("QueryHomes = %+v, %v", got, err)
			}
		}},
		{"home devices", http.MethodGet, routeFixtureV10MLifeHaHomeDevices, []any{device}, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.QueryDevicesByHome(ctx, QueryDevicesByHomeRequest{HomeID: routeFixtureHome1})
			if err != nil || len(got.Results) != 1 || got.Results[0].ID != routeFixtureDevice1 {
				t.Fatalf("QueryDevicesByHome = %+v, %v", got, err)
			}
		}},
		{"devices by IDs", http.MethodGet, routeFixtureV10MLifeHaHomeDevices, []any{device}, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.QueryDevicesByIDs(ctx, QueryDevicesByIDsRequest{DeviceIDs: []string{routeFixtureDevice1}})
			if err != nil || len(got.Results) != 1 || got.Results[0].ID != routeFixtureDevice1 {
				t.Fatalf("QueryDevicesByIDs = %+v, %v", got, err)
			}
		}},
		{"command", http.MethodPost, "/v1.1/m/thing/device-1/commands", true, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.SendCommands(ctx, SendCommandsRequest{
				DeviceID: routeFixtureDevice1,
				Commands: []Command{{Code: routeFixtureSwitch, Value: true}},
			})
			if err != nil || !got.Result {
				t.Fatalf("SendCommands = %+v, %v", got, err)
			}
		}},
		{"device status", http.MethodGet, "/v1.0/m/life/devices/device-1/status", map[string]any{"dpStatusRelationDTOS": []any{}}, func(t *testing.T, s *Session) {
			t.Helper()

			if _, err := s.DevicesService.QueryDeviceStatus(ctx, QueryDeviceStatusRequest{DeviceID: routeFixtureDevice1}); err != nil {
				t.Fatal(err)
			}
		}},
		{"device specification", http.MethodGet, "/v1.1/m/life/device-1/specifications", map[string]any{}, func(t *testing.T, s *Session) {
			t.Helper()

			if _, err := s.DevicesService.QueryDeviceSpecification(ctx, QueryDeviceSpecificationRequest{DeviceID: routeFixtureDevice1}); err != nil {
				t.Fatal(err)
			}
		}},
		{"device details", http.MethodGet, routeFixtureV10DevicesDevice1, device, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.GetDeviceDetails(ctx, GetDeviceDetailsRequest{DeviceID: routeFixtureDevice1})
			if err != nil || got.Device.ID != routeFixtureDevice1 {
				t.Fatalf("GetDeviceDetails = %+v, %v", got, err)
			}
		}},
		{"devices by user", http.MethodGet, "/v1.0/users/user-1/devices", []any{device}, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.QueryDevicesByUser(ctx, QueryDevicesByUserRequest{UID: "user-1"})
			if err != nil || len(got.Devices) != 1 || got.Devices[0].ID != routeFixtureDevice1 {
				t.Fatalf("QueryDevicesByUser = %+v, %v", got, err)
			}
		}},
		{"global devices", http.MethodGet, "/v1.0/devices", map[string]any{"total": 1, "last_id": "next", "devices": []any{device}}, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.QueryDevices(ctx, QueryDevicesRequest{PageNo: 1, PageSize: 10})
			if err != nil || got.Total != 1 || len(got.Devices) != 1 || got.Devices[0].ID != routeFixtureDevice1 {
				t.Fatalf("QueryDevices = %+v, %v", got, err)
			}
		}},
		{"rename function", http.MethodPut, "/v1.0/devices/device-1/functions/switch", true, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.UpdateDeviceFunctionName(ctx, UpdateDeviceFunctionNameRequest{
				DeviceID:     routeFixtureDevice1,
				FunctionCode: routeFixtureSwitch,
				Name:         "Power",
			})
			if err != nil || !got.Result {
				t.Fatalf("UpdateDeviceFunctionName = %+v, %v", got, err)
			}
		}},
		{"reset device", http.MethodPut, "/v1.0/devices/device-1/reset-factory", true, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.ResetDeviceFactoryDefaults(ctx, ResetDeviceFactoryDefaultsRequest{DeviceID: routeFixtureDevice1})
			if err != nil || !got.Result {
				t.Fatalf("ResetDeviceFactoryDefaults = %+v, %v", got, err)
			}
		}},
		{"delete device", http.MethodDelete, routeFixtureV10DevicesDevice1, true, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.DeleteDevice(ctx, DeleteDeviceRequest{DeviceID: routeFixtureDevice1})
			if err != nil || !got.Result {
				t.Fatalf("DeleteDevice = %+v, %v", got, err)
			}
		}},
		{"rename device", http.MethodPut, routeFixtureV10DevicesDevice1, true, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.UpdateDeviceName(ctx, UpdateDeviceNameRequest{DeviceID: routeFixtureDevice1, Name: "Desk lamp"})
			if err != nil || !got.Result {
				t.Fatalf("UpdateDeviceName = %+v, %v", got, err)
			}
		}},
		{"add device user", http.MethodPost, "/v1.0/devices/device-1/user", routeFixtureUser2, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.AddDeviceUser(ctx, AddDeviceUserRequest{DeviceID: routeFixtureDevice1, NickName: "Guest"})
			if err != nil || got.UserID != routeFixtureUser2 {
				t.Fatalf("AddDeviceUser = %+v, %v", got, err)
			}
		}},
		{"remove device user", http.MethodDelete, routeFixtureV10DevicesDevice1UsersUser2, true, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.DeleteDeviceUser(ctx, DeleteDeviceUserRequest{DeviceID: routeFixtureDevice1, UserID: routeFixtureUser2})
			if err != nil || !got.Result {
				t.Fatalf("DeleteDeviceUser = %+v, %v", got, err)
			}
		}},
		{"list device users", http.MethodGet, "/v1.0/devices/device-1/users", []any{}, func(t *testing.T, s *Session) {
			t.Helper()

			if _, err := s.DevicesService.ListDeviceUsers(ctx, ListDeviceUsersRequest{DeviceID: routeFixtureDevice1}); err != nil {
				t.Fatal(err)
			}
		}},
		{"device logs", http.MethodGet, "/v1.0/devices/device-1/logs", map[string]any{}, func(t *testing.T, s *Session) {
			t.Helper()

			if _, err := s.DevicesService.QueryDeviceLogs(ctx, QueryDeviceLogsRequest{DeviceID: routeFixtureDevice1, Types: []string{"report"}}); err != nil {
				t.Fatal(err)
			}
		}},
		{"sub devices", http.MethodGet, "/v1.0/devices/device-1/sub-devices", []any{}, func(t *testing.T, s *Session) {
			t.Helper()

			if _, err := s.DevicesService.QuerySubDevices(ctx, QuerySubDevicesRequest{DeviceID: routeFixtureDevice1}); err != nil {
				t.Fatal(err)
			}
		}},
		{"factory info", http.MethodGet, "/v1.0/devices/factory-infos", []any{}, func(t *testing.T, s *Session) {
			t.Helper()

			if _, err := s.DevicesService.QueryDeviceFactoryInfos(ctx, QueryDeviceFactoryInfosRequest{DeviceIDs: []string{routeFixtureDevice1}}); err != nil {
				t.Fatal(err)
			}
		}},
		{"update device user", http.MethodPut, routeFixtureV10DevicesDevice1UsersUser2, true, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.UpdateDeviceUser(ctx, UpdateDeviceUserRequest{DeviceID: routeFixtureDevice1, UserID: routeFixtureUser2, NickName: "Guest"})
			if err != nil || !got.Result {
				t.Fatalf("UpdateDeviceUser = %+v, %v", got, err)
			}
		}},
		{"get device user", http.MethodGet, routeFixtureV10DevicesDevice1UsersUser2, map[string]any{}, func(t *testing.T, s *Session) {
			t.Helper()

			if _, err := s.DevicesService.GetDeviceUser(ctx, GetDeviceUserRequest{DeviceID: routeFixtureDevice1, UserID: routeFixtureUser2}); err != nil {
				t.Fatal(err)
			}
		}},
		{"rename outlet", http.MethodPut, "/v1.0/devices/device-1/multiple-name", true, func(t *testing.T, s *Session) {
			t.Helper()

			got, err := s.DevicesService.UpdateMultiOutletName(ctx, UpdateMultiOutletNameRequest{DeviceID: routeFixtureDevice1, Identifier: "outlet-1", Name: "Desk"})
			if err != nil || !got.Result {
				t.Fatalf("UpdateMultiOutletName = %+v, %v", got, err)
			}
		}},
		{"list outlets", http.MethodGet, "/v1.0/devices/device-1/multiple-names", []any{}, func(t *testing.T, s *Session) {
			t.Helper()

			if _, err := s.DevicesService.ListMultiOutletNames(ctx, ListMultiOutletNamesRequest{DeviceID: routeFixtureDevice1}); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			requests := 0

			server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
				requests++

				if request.Method != test.method || request.URL.Path != test.path {
					t.Errorf("request = %s %s, want %s %s", request.Method, request.URL.Path, test.method, test.path)
				}

				if request.Header.Get("X-Token") != routeFixtureSyntheticAccess || request.Header.Get("X-Requestid") == "" {
					t.Errorf("missing synthetic auth or request ID headers")
				}

				result := test.result
				if value, ok := result.(string); ok {
					// #nosec G401 -- synthetic response encryption mirrors Tuya's MD5 request key.
					secretHash := md5.Sum([]byte(request.Header.Get("X-Requestid") + syntheticRefreshFixture))
					secret := secretGenerating(request.Header.Get("X-Requestid"), "", hex.EncodeToString(secretHash[:]))
					plaintext, _ := json.Marshal(value)

					encrypted, err := aesGCMEncrypt(string(plaintext), secret)
					if err != nil {
						t.Errorf("encrypt synthetic response: %v", err)

						return
					}

					result = string(encrypted)
				}

				_ = json.NewEncoder(responseWriter).Encode(map[string]any{"success": true, routeFixtureCode: 200, "result": result})
			}))
			defer server.Close()

			client, err := newSyntheticClient(WithCloudAPIURL(server.URL), WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatal(err)
			}

			session := client.NewSession(Tokens{AccessToken: routeFixtureSyntheticAccess, RefreshToken: syntheticRefreshFixture})
			test.run(t, session)

			if requests != 1 {
				t.Fatalf("request count = %d, want 1", requests)
			}
		})
	}
}
