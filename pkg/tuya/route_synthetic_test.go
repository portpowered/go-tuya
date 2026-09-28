package tuya

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These hand-authored responses exercise the public operation mapping and
// request routes. They are synthetic and are not Tuya account captures.
func TestSyntheticDeviceAndHomeRoutes(t *testing.T) {
	ctx := context.Background()
	device := map[string]any{"id": "device-1", "name": "Desk lamp", "category": "dj"}
	tests := []struct {
		name   string
		method string
		path   string
		result any
		run    func(*testing.T, *Session)
	}{
		{"homes", http.MethodGet, "/v1.0/m/life/users/homes", []any{map[string]any{"ownerId": "home-1", "name": "Home"}}, func(t *testing.T, s *Session) {
			got, err := s.HomeService.QueryHomes(ctx, QueryHomesRequest{})
			if err != nil || len(got.Results) != 1 || got.Results[0].ID != "home-1" {
				t.Fatalf("QueryHomes = %+v, %v", got, err)
			}
		}},
		{"home devices", http.MethodGet, "/v1.0/m/life/ha/home/devices", []any{device}, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.QueryDevicesByHome(ctx, QueryDevicesByHomeRequest{HomeID: "home-1"})
			if err != nil || len(got.Results) != 1 || got.Results[0].ID != "device-1" {
				t.Fatalf("QueryDevicesByHome = %+v, %v", got, err)
			}
		}},
		{"home assistant devices", http.MethodGet, "/v1.0/m/life/ha/home/devices", []any{device}, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.QueryDevicesByHomeAssistantDevices(ctx, QueryDevicesByHomeRequest{HomeID: "home-1"})
			if err != nil || len(got.Results) != 1 || got.Results[0].ID != "device-1" {
				t.Fatalf("QueryDevicesByHomeAssistantDevices = %+v, %v", got, err)
			}
		}},
		{"devices by IDs", http.MethodGet, "/v1.0/m/life/ha/home/devices", []any{device}, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.QueryDevicesByIDs(ctx, QueryDevicesByIDsRequest{DeviceIDs: []string{"device-1"}})
			if err != nil || len(got.Results) != 1 || got.Results[0].ID != "device-1" {
				t.Fatalf("QueryDevicesByIDs = %+v, %v", got, err)
			}
		}},
		{"command", http.MethodPost, "/v1.1/m/thing/device-1/commands", true, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.SendCommands(ctx, SendCommandsRequest{DeviceID: "device-1", Commands: []Command{{Code: "switch", Value: true}}})
			if err != nil || !got.Result {
				t.Fatalf("SendCommands = %+v, %v", got, err)
			}
		}},
		{"device status", http.MethodGet, "/v1.0/m/life/devices/device-1/status", map[string]any{"dpStatusRelationDTOS": []any{}}, func(t *testing.T, s *Session) {
			if _, err := s.DevicesService.QueryDeviceStatus(ctx, QueryDeviceStatusRequest{DeviceID: "device-1"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"device specification", http.MethodGet, "/v1.1/m/life/device-1/specifications", map[string]any{}, func(t *testing.T, s *Session) {
			if _, err := s.DevicesService.QueryDeviceSpecification(ctx, QueryDeviceSpecificationRequest{DeviceID: "device-1"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"device details", http.MethodGet, "/v1.0/devices/device-1", device, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.GetDeviceDetails(ctx, GetDeviceDetailsRequest{DeviceID: "device-1"})
			if err != nil || got.Device.ID != "device-1" {
				t.Fatalf("GetDeviceDetails = %+v, %v", got, err)
			}
		}},
		{"devices by user", http.MethodGet, "/v1.0/users/user-1/devices", []any{device}, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.QueryDevicesByUser(ctx, QueryDevicesByUserRequest{UID: "user-1"})
			if err != nil || len(got.Devices) != 1 || got.Devices[0].ID != "device-1" {
				t.Fatalf("QueryDevicesByUser = %+v, %v", got, err)
			}
		}},
		{"global devices", http.MethodGet, "/v1.0/devices", map[string]any{"total": 1, "last_id": "next", "devices": []any{device}}, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.QueryDevices(ctx, QueryDevicesRequest{PageNo: 1, PageSize: 10})
			if err != nil || got.Total != 1 || len(got.Devices) != 1 || got.Devices[0].ID != "device-1" {
				t.Fatalf("QueryDevices = %+v, %v", got, err)
			}
		}},
		{"rename function", http.MethodPut, "/v1.0/devices/device-1/functions/switch", true, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.UpdateDeviceFunctionName(ctx, UpdateDeviceFunctionNameRequest{DeviceID: "device-1", FunctionCode: "switch", Name: "Power"})
			if err != nil || !got.Result {
				t.Fatalf("UpdateDeviceFunctionName = %+v, %v", got, err)
			}
		}},
		{"reset device", http.MethodPut, "/v1.0/devices/device-1/reset-factory", true, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.ResetDeviceFactoryDefaults(ctx, ResetDeviceFactoryDefaultsRequest{DeviceID: "device-1"})
			if err != nil || !got.Result {
				t.Fatalf("ResetDeviceFactoryDefaults = %+v, %v", got, err)
			}
		}},
		{"delete device", http.MethodDelete, "/v1.0/devices/device-1", true, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.DeleteDevice(ctx, DeleteDeviceRequest{DeviceID: "device-1"})
			if err != nil || !got.Result {
				t.Fatalf("DeleteDevice = %+v, %v", got, err)
			}
		}},
		{"rename device", http.MethodPut, "/v1.0/devices/device-1", true, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.UpdateDeviceName(ctx, UpdateDeviceNameRequest{DeviceID: "device-1", Name: "Desk lamp"})
			if err != nil || !got.Result {
				t.Fatalf("UpdateDeviceName = %+v, %v", got, err)
			}
		}},
		{"add device user", http.MethodPost, "/v1.0/devices/device-1/user", "user-2", func(t *testing.T, s *Session) {
			got, err := s.DevicesService.AddDeviceUser(ctx, AddDeviceUserRequest{DeviceID: "device-1", NickName: "Guest"})
			if err != nil || got.UserID != "user-2" {
				t.Fatalf("AddDeviceUser = %+v, %v", got, err)
			}
		}},
		{"remove device user", http.MethodDelete, "/v1.0/devices/device-1/users/user-2", true, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.DeleteDeviceUser(ctx, DeleteDeviceUserRequest{DeviceID: "device-1", UserID: "user-2"})
			if err != nil || !got.Result {
				t.Fatalf("DeleteDeviceUser = %+v, %v", got, err)
			}
		}},
		{"list device users", http.MethodGet, "/v1.0/devices/device-1/users", []any{}, func(t *testing.T, s *Session) {
			if _, err := s.DevicesService.ListDeviceUsers(ctx, ListDeviceUsersRequest{DeviceID: "device-1"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"device logs", http.MethodGet, "/v1.0/devices/device-1/logs", map[string]any{}, func(t *testing.T, s *Session) {
			if _, err := s.DevicesService.QueryDeviceLogs(ctx, QueryDeviceLogsRequest{DeviceID: "device-1", Types: []string{"report"}}); err != nil {
				t.Fatal(err)
			}
		}},
		{"sub devices", http.MethodGet, "/v1.0/devices/device-1/sub-devices", []any{}, func(t *testing.T, s *Session) {
			if _, err := s.DevicesService.QuerySubDevices(ctx, QuerySubDevicesRequest{DeviceID: "device-1"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"factory info", http.MethodGet, "/v1.0/devices/factory-infos", []any{}, func(t *testing.T, s *Session) {
			if _, err := s.DevicesService.QueryDeviceFactoryInfos(ctx, QueryDeviceFactoryInfosRequest{DeviceIDs: []string{"device-1"}}); err != nil {
				t.Fatal(err)
			}
		}},
		{"update device user", http.MethodPut, "/v1.0/devices/device-1/users/user-2", true, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.UpdateDeviceUser(ctx, UpdateDeviceUserRequest{DeviceID: "device-1", UserID: "user-2", NickName: "Guest"})
			if err != nil || !got.Result {
				t.Fatalf("UpdateDeviceUser = %+v, %v", got, err)
			}
		}},
		{"get device user", http.MethodGet, "/v1.0/devices/device-1/users/user-2", map[string]any{}, func(t *testing.T, s *Session) {
			if _, err := s.DevicesService.GetDeviceUser(ctx, GetDeviceUserRequest{DeviceID: "device-1", UserID: "user-2"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"rename outlet", http.MethodPut, "/v1.0/devices/device-1/multiple-name", true, func(t *testing.T, s *Session) {
			got, err := s.DevicesService.UpdateMultiOutletName(ctx, UpdateMultiOutletNameRequest{DeviceID: "device-1", Identifier: "outlet-1", Name: "Desk"})
			if err != nil || !got.Result {
				t.Fatalf("UpdateMultiOutletName = %+v, %v", got, err)
			}
		}},
		{"list outlets", http.MethodGet, "/v1.0/devices/device-1/multiple-names", []any{}, func(t *testing.T, s *Session) {
			if _, err := s.DevicesService.ListMultiOutletNames(ctx, ListMultiOutletNamesRequest{DeviceID: "device-1"}); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != test.method || r.URL.Path != test.path {
					t.Errorf("request = %s %s, want %s %s", r.Method, r.URL.Path, test.method, test.path)
				}
				if r.Header.Get("X-token") != "synthetic-access" || r.Header.Get("X-requestId") == "" {
					t.Errorf("missing synthetic auth or request ID headers")
				}
				result := test.result
				if value, ok := result.(string); ok {
					secretHash := md5.Sum([]byte(r.Header.Get("X-requestId") + "synthetic-refresh"))
					secret := secretGenerating(r.Header.Get("X-requestId"), "", hex.EncodeToString(secretHash[:]))
					plaintext, _ := json.Marshal(value)
					encrypted, err := aesGCMEncrypt(string(plaintext), secret)
					if err != nil {
						t.Errorf("encrypt synthetic response: %v", err)
						return
					}
					result = string(encrypted)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "code": 200, "result": result})
			}))
			defer server.Close()
			client, err := NewClient(WithCloudAPIURL(server.URL), WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatal(err)
			}
			session := client.NewSession(Tokens{AccessToken: "synthetic-access", RefreshToken: "synthetic-refresh"})
			test.run(t, session)
			if requests != 1 {
				t.Fatalf("request count = %d, want 1", requests)
			}
		})
	}
}
