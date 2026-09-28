//go:build integration

package tuya

import (
	"context"
	"os"
	"strconv"
	"testing"
)

// TestIntegration_DeviceManagementWorkflow tests the complete device management workflow
// This opt-in test requires real Tuya credentials and makes live cloud requests.
func TestIntegration_DeviceManagementWorkflow(t *testing.T) {
	// Check required environment variables
	requiredEnvVars := []string{
		"TUYA_AUTH_TOKEN",
		"TUYA_REFRESH_TOKEN",
		"TUYA_AUTH_TOKEN_EXPIRED",
	}

	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			t.Skipf("set %s to run this live integration test", envVar)
		}
	}

	ctx := context.Background()

	// Parse expire time
	expireTimeStr := os.Getenv("TUYA_AUTH_TOKEN_EXPIRED")
	expireTime, err := strconv.ParseInt(expireTimeStr, 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse TUYA_AUTH_TOKEN_EXPIRED: %v", err)
	}

	// Create client with real credentials
	client := NewClient(&ClientConfig{
		AuthInformation: &AuthInformation{
			AccessToken:  os.Getenv("TUYA_AUTH_TOKEN"),
			RefreshToken: os.Getenv("TUYA_REFRESH_TOKEN"),
			ExpireTime:   expireTime,
		},
	})

	// Test 1: Query Homes
	t.Run("QueryHomes", func(t *testing.T) {
		homes, err := client.HomeService.QueryHomes(ctx, QueryHomesRequest{})
		if err != nil {
			t.Fatalf("Failed to query homes: %v", err)
		}

		if len(homes.Results) == 0 {
			t.Skip("No homes found, skipping device tests")
		}

		t.Logf("Found %d homes", len(homes.Results))
		for i, home := range homes.Results {
			t.Logf("Home %d: ID=%s, Name=%s", i+1, home.ID, home.Name)
		}
	})

	// Get homes for subsequent tests
	homes, err := client.HomeService.QueryHomes(ctx, QueryHomesRequest{})
	if err != nil {
		t.Fatalf("Failed to query homes for setup: %v", err)
	}

	if len(homes.Results) == 0 {
		t.Skip("No homes available for device tests")
	}

	homeID := homes.Results[0].ID

	// Test 2: Query Devices by Home
	t.Run("QueryDevicesByHome", func(t *testing.T) {
		devices, err := client.DevicesService.QueryDevicesByHome(ctx, QueryDevicesByHomeRequest{
			HomeID: homeID,
		})
		if err != nil {
			t.Fatalf("Failed to query devices by home: %v", err)
		}

		t.Logf("Found %d devices in home %s", len(devices.Results), homeID)
		for i, device := range devices.Results {
			t.Logf("Device %d: ID=%s, Name=%s, Category=%s, Online=%t",
				i+1, device.ID, device.Name, device.Category, device.Online)
		}
	})

	// Get devices for subsequent tests
	devices, err := client.DevicesService.QueryDevicesByHome(ctx, QueryDevicesByHomeRequest{
		HomeID: homeID,
	})
	if err != nil {
		t.Fatalf("Failed to query devices for setup: %v", err)
	}

	if len(devices.Results) == 0 {
		t.Skip("No devices available for device-specific tests")
	}

	deviceID := devices.Results[0].ID

	t.Run("QueryDeviceStatus", func(t *testing.T) {
		deviceStatus, err := client.DevicesService.QueryDeviceStatus(ctx, QueryDeviceStatusRequest{
			DeviceID: deviceID,
		})
		if err != nil {
			t.Fatalf("Failed to query device status: %v", err)
		}
		t.Logf("Device status: %+v", deviceStatus)
	})
}
