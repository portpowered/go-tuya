// Package main demonstrates read-only device queries.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/portpowered/go-tuya/tuya"
)

func main() {
	accessToken := os.Getenv("TUYA_AUTH_TOKEN")
	refreshToken := os.Getenv("TUYA_REFRESH_TOKEN")
	expireTime, err := strconv.ParseInt(os.Getenv("TUYA_AUTH_TOKEN_EXPIRED"), 10, 64)
	if accessToken == "" || refreshToken == "" || err != nil {
		log.Fatal("set TUYA_AUTH_TOKEN, TUYA_REFRESH_TOKEN, and TUYA_AUTH_TOKEN_EXPIRED")
	}

	client := tuya.NewClient(&tuya.ClientConfig{
		AuthInformation: &tuya.AuthInformation{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpireTime:   expireTime,
		},
	})
	ctx := context.Background()

	homes, err := client.HomeService.QueryHomes(ctx, tuya.QueryHomesRequest{})
	if err != nil {
		log.Fatal("home query failed; redact credentials before inspecting diagnostics")
	}
	fmt.Printf("Found %d homes.\n", len(homes.Results))
	if len(homes.Results) == 0 {
		return
	}

	devices, err := client.DevicesService.QueryDevicesByHome(ctx, tuya.QueryDevicesByHomeRequest{
		HomeID: homes.Results[0].ID,
	})
	if err != nil {
		log.Fatal("device query failed; redact credentials before inspecting diagnostics")
	}
	fmt.Printf("Found %d devices in the first home.\n", len(devices.Results))
	if len(devices.Results) == 0 {
		return
	}

	deviceID := devices.Results[0].ID
	if _, err := client.DevicesService.QueryDeviceStatus(ctx, tuya.QueryDeviceStatusRequest{DeviceID: deviceID}); err != nil {
		log.Fatal("device status query failed; redact credentials before inspecting diagnostics")
	}
	if _, err := client.DevicesService.QueryDeviceSpecification(ctx, tuya.QueryDeviceSpecificationRequest{DeviceID: deviceID}); err != nil {
		log.Fatal("device specification query failed; redact credentials before inspecting diagnostics")
	}
	if _, err := client.DevicesService.GetDeviceDetails(ctx, tuya.GetDeviceDetailsRequest{DeviceID: deviceID}); err != nil {
		log.Fatal("device details query failed; redact credentials before inspecting diagnostics")
	}
	fmt.Println("Status, specification, and details queries succeeded; response values are suppressed.")

	allDevices, err := client.DevicesService.QueryDevices(ctx, tuya.QueryDevicesRequest{PageNo: 1, PageSize: 20})
	if err != nil {
		log.Fatal("global device query failed; redact credentials before inspecting diagnostics")
	}
	fmt.Printf("Found %d devices on the requested page.\n", len(allDevices.Devices))
}
