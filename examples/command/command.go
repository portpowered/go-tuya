// Package main demonstrates sending a command to a device.
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
	deviceID := os.Getenv("TUYA_DEVICE_ID")
	expireTime, err := strconv.ParseInt(os.Getenv("TUYA_AUTH_TOKEN_EXPIRED"), 10, 64)
	if accessToken == "" || refreshToken == "" || deviceID == "" || err != nil {
		log.Fatal("set TUYA_AUTH_TOKEN, TUYA_REFRESH_TOKEN, TUYA_AUTH_TOKEN_EXPIRED, and TUYA_DEVICE_ID")
	}

	client := tuya.NewClient(&tuya.ClientConfig{
		AuthInformation: &tuya.AuthInformation{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpireTime:   expireTime,
		},
	})
	_, err = client.DevicesService.SendCommands(context.Background(), tuya.SendCommandsRequest{
		DeviceID: deviceID,
		Commands: []tuya.Command{{Code: "switch_led_1", Value: true}},
	})
	if err != nil {
		log.Fatal("device command failed; redact credentials before inspecting diagnostics")
	}
	fmt.Println("Command sent successfully.")
}
