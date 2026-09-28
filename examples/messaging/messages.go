// Package main demonstrates subscribing to Tuya account and device events.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/portpowered/go-tuya/tuya"
)

func main() {
	accessToken := os.Getenv("TUYA_AUTH_TOKEN")
	refreshToken := os.Getenv("TUYA_REFRESH_TOKEN")
	expireTime, err := strconv.ParseInt(os.Getenv("TUYA_AUTH_TOKEN_EXPIRED"), 10, 64)
	if accessToken == "" || refreshToken == "" || err != nil {
		log.Fatal("set TUYA_AUTH_TOKEN, TUYA_REFRESH_TOKEN, and TUYA_AUTH_TOKEN_EXPIRED")
	}

	ctx := context.Background()
	client := tuya.NewClient(&tuya.ClientConfig{
		AuthInformation: &tuya.AuthInformation{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpireTime:   expireTime,
		},
	})
	queueConfig, err := client.MessageQueue.GetMessageQueueConfig(ctx)
	if err != nil {
		log.Fatal("could not get message queue configuration; redact credentials before inspecting diagnostics")
	}

	homes, err := client.HomeService.QueryHomes(ctx, tuya.QueryHomesRequest{})
	if err != nil {
		log.Fatal("home query failed; redact credentials before inspecting diagnostics")
	}
	if len(homes.Results) == 0 {
		log.Fatal("no homes are available for message listeners")
	}
	devices, err := client.DevicesService.QueryDevicesByHome(ctx, tuya.QueryDevicesByHomeRequest{HomeID: homes.Results[0].ID})
	if err != nil {
		log.Fatal("device query failed; redact credentials before inspecting diagnostics")
	}

	_, err = client.MessageQueue.AddMessageListener(ctx, tuya.AddMessageListenerRequest{
		Topic: strings.Replace(queueConfig.OwnerTopic, "{ownerId}", homes.Results[0].ID, 1),
		Callback: func(_ string, _ interface{}) {
			fmt.Println("Received an account event.")
		},
	})
	if err != nil {
		log.Fatal("could not add account event listener; redact credentials before inspecting diagnostics")
	}

	for _, device := range devices.Results {
		listener := &tuya.TypedEventListener{
			OnStateChange: func(_ *tuya.DeviceStateChangeEvent) { fmt.Println("Received a device state change.") },
			OnManagement:  func(_ *tuya.DeviceManagementEvent) { fmt.Println("Received a device management event.") },
			OnOnline:      func(_ *tuya.DeviceOnlineEvent) { fmt.Println("A device came online.") },
			OnOffline:     func(_ *tuya.DeviceOfflineEvent) { fmt.Println("A device went offline.") },
			OnNameUpdate:  func(_ *tuya.DeviceNameUpdateEvent) { fmt.Println("A device name changed.") },
			OnDelete:      func(_ *tuya.DeviceDeleteEvent) { fmt.Println("A device was removed.") },
		}
		_, err := client.MessageQueue.AddDeviceListener(ctx, tuya.AddDeviceListenerRequest{
			DeviceID: device.ID,
			Callback: func(_ string, event tuya.Event) { listener.HandleEvent(event) },
		})
		if err != nil {
			log.Fatal("could not add device event listener; redact credentials before inspecting diagnostics")
		}
	}

	if _, err := client.MessageQueue.Start(ctx, tuya.MessageQueueStartRequest{}); err != nil {
		log.Fatal("could not start message queue; redact credentials before inspecting diagnostics")
	}
	fmt.Println("Listening for events for up to 15 minutes; event payloads are suppressed.")
	select {
	case <-ctx.Done():
	case <-time.After(15 * time.Minute):
	}
	if _, err := client.MessageQueue.Stop(ctx, tuya.MessageQueueStopRequest{}); err != nil {
		log.Fatal("could not stop message queue; redact credentials before inspecting diagnostics")
	}
}
