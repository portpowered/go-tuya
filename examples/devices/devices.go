// Package main demonstrates read-only device queries.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/portpowered/go-tuya/pkg/tuya"
)

const exampleDevicePageSize = 20

//nolint:cyclop,funlen // The runnable example keeps each read-only SDK call and its credential-safe error boundary visible.
func main() {
	accessToken := os.Getenv("TUYA_AUTH_TOKEN")
	refreshToken := os.Getenv("TUYA_REFRESH_TOKEN")

	expireTime, err := strconv.ParseInt(os.Getenv("TUYA_AUTH_TOKEN_EXPIRED"), 10, 64)

	if accessToken == "" || refreshToken == "" || err != nil {
		log.Fatal("set TUYA_AUTH_TOKEN, TUYA_REFRESH_TOKEN, and TUYA_AUTH_TOKEN_EXPIRED")
	}

	base, err := tuya.NewClient(tuya.WithClientID(os.Getenv("TUYA_CLIENT_ID")))
	if err != nil {
		log.Fatal("could not configure the Tuya client")
	}

	client := base.NewSession(tuya.Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpireTime:   expireTime,
	})
	ctx := context.Background()
	request := tuya.Request{AuthorizationContext: nil} //nolint:exhaustruct,exhaustruct_v5 // The deprecated refresh switch intentionally stays zero.

	homes, err := client.HomeService.QueryHomes(ctx, tuya.QueryHomesRequest{Request: request})
	if err != nil {
		log.Fatal("home query failed; redact credentials before inspecting diagnostics")
	}

	fmt.Printf("Found %d homes.\n", len(homes.Results))

	if len(homes.Results) == 0 {
		return
	}

	devices, err := client.DevicesService.QueryDevicesByHome(ctx, tuya.QueryDevicesByHomeRequest{
		Request: request,
		HomeID:  homes.Results[0].ID,
	})
	if err != nil {
		log.Fatal("device query failed; redact credentials before inspecting diagnostics")
	}

	fmt.Printf("Found %d devices in the first home.\n", len(devices.Results))

	if len(devices.Results) == 0 {
		return
	}

	deviceID := devices.Results[0].ID

	_, err = client.DevicesService.QueryDeviceStatus(ctx, tuya.QueryDeviceStatusRequest{
		Request:  request,
		DeviceID: deviceID,
	})
	if err != nil {
		log.Fatal("device status query failed; redact credentials before inspecting diagnostics")
	}

	_, err = client.DevicesService.QueryDeviceSpecification(ctx, tuya.QueryDeviceSpecificationRequest{
		Request:  request,
		DeviceID: deviceID,
	})
	if err != nil {
		log.Fatal("device specification query failed; redact credentials before inspecting diagnostics")
	}

	_, err = client.DevicesService.GetDeviceDetails(ctx, tuya.GetDeviceDetailsRequest{
		Request:  request,
		DeviceID: deviceID,
	})
	if err != nil {
		log.Fatal("device details query failed; redact credentials before inspecting diagnostics")
	}

	fmt.Println("Status, specification, and details queries succeeded; response values are suppressed.")

	allDevices, err := client.DevicesService.QueryDevices(ctx, tuya.QueryDevicesRequest{
		Request:  request,
		PageNo:   1,
		PageSize: exampleDevicePageSize,
		Schema:   "", ProductID: "", DeviceIDs: nil, StartTime: "", EndTime: "", LastID: "",
	})
	if err != nil {
		log.Fatal("global device query failed; redact credentials before inspecting diagnostics")
	}

	fmt.Printf("Found %d devices on the requested page.\n", len(allDevices.Devices))
}
