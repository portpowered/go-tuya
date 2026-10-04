package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/portpowered/go-tuya/pkg/tuya"
)

const eventQueueBuffer = 16

type eventRecord struct {
	Type     string `json:"type"`
	DeviceID string `json:"device_id,omitempty"`
}

type eventTargets struct {
	homes   []tuya.Home
	devices map[string][]tuya.Device
}

func emptyEventTargets() eventTargets {
	var targets eventTargets

	return targets
}

func eventsCommand(ctx context.Context, args []string, config settings, out io.Writer, deps Dependencies) error {
	if len(args) == 0 || args[0] != "watch" {
		return usageError("usage: go-tuya [global flags] events watch [--duration duration] [--home id]")
	}

	flags := flag.NewFlagSet("events watch", flag.ContinueOnError)
	duration := flags.Duration("duration", defaultEventDuration, "how long to watch events")
	homeID := flags.String("home", "", "watch events for one home")

	_, help, err := parseCommandFlags(flags, args[1:], out, "Usage: go-tuya [global flags] events watch [--duration duration] [--home id]")
	if err != nil || help {
		return err
	}

	if len(flags.Args()) != 0 || *duration <= 0 {
		return usageError("events watch requires a positive duration and no positional arguments")
	}

	document, err := loadTokenDocument(config)
	if err != nil {
		return err
	}

	return withSession(ctx, config, document, deps, func(session *tuya.Session) error {
		return watchSession(ctx, session, *homeID, *duration, config, out)
	})
}

func watchSession(ctx context.Context, session *tuya.Session, homeID string, duration time.Duration, config settings, out io.Writer) (runErr error) {
	targets, err := queryEventTargets(ctx, session, homeID, config.requestTimeout)
	if err != nil {
		return err
	}

	queueConfig, err := queryEventQueueConfig(ctx, session, config.requestTimeout)
	if err != nil {
		return err
	}

	if queueConfig.OwnerTopic == "" || queueConfig.DeviceTopic == "" {
		return newCommandError("provider returned incomplete event topic configuration", "protocol")
	}

	queueEvents := make(chan eventRecord, eventQueueBuffer)
	publish := makeEventPublisher(ctx, queueEvents)

	err = registerEventListeners(ctx, session, targets, queueConfig, publish)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, config.requestTimeout)
	started, err := session.MessageQueue.Start(requestCtx, tuya.MessageQueueStartRequest{Request: newRequest()})

	cancel()

	if err != nil {
		return safeSDKError("start event queue", err)
	}

	if !started.Success {
		return newCommandError("event queue did not start", "transport")
	}

	defer func(parent context.Context) {
		closeCtx, closeCancel := context.WithTimeout(context.WithoutCancel(parent), closeTimeout)
		defer closeCancel()

		_, stopErr := session.MessageQueue.Stop(closeCtx, tuya.MessageQueueStopRequest{Request: newRequest()})
		if stopErr != nil && runErr == nil {
			runErr = safeSDKError("stop event queue", stopErr)
		}
	}(ctx)

	return waitForEvents(ctx, queueEvents, duration, config.jsonOutput, out)
}

func queryEventTargets(ctx context.Context, session *tuya.Session, homeID string, requestTimeout time.Duration) (eventTargets, error) {
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	homes, err := session.HomeService.QueryHomes(requestCtx, tuya.QueryHomesRequest{Request: newRequest()})

	cancel()

	if err != nil {
		return emptyEventTargets(), safeSDKError("list homes for event listeners", err)
	}

	targets := eventTargets{
		homes:   make([]tuya.Home, 0, len(homes.Results)),
		devices: make(map[string][]tuya.Device),
	}

	for _, home := range homes.Results {
		if homeID != "" && home.ID != homeID {
			continue
		}

		targets.homes = append(targets.homes, home)
	}

	if len(targets.homes) == 0 {
		return emptyEventTargets(), newCommandError("no matching homes are available for event listeners", "not_found")
	}

	for _, home := range targets.homes {
		requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		devices, queryErr := session.DevicesService.QueryDevicesByHome(requestCtx, tuya.QueryDevicesByHomeRequest{
			Request: newRequest(),
			HomeID:  home.ID,
		})

		cancel()

		if queryErr != nil {
			return emptyEventTargets(), safeSDKError("list devices for event listeners", queryErr)
		}

		targets.devices[home.ID] = devices.Results
	}

	return targets, nil
}

func queryEventQueueConfig(ctx context.Context, session *tuya.Session, requestTimeout time.Duration) (tuya.MessageQueueConfig, error) {
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	config, err := session.MessageQueue.GetMessageQueueConfig(requestCtx)

	cancel()

	if err != nil {
		var emptyConfig tuya.MessageQueueConfig

		return emptyConfig, safeSDKError("get event queue configuration", err)
	}

	return config, nil
}

func makeEventPublisher(ctx context.Context, events chan<- eventRecord) func(eventRecord) {
	return func(record eventRecord) {
		select {
		case events <- record:
		case <-ctx.Done():
		}
	}
}

func registerEventListeners(ctx context.Context, session *tuya.Session, targets eventTargets, config tuya.MessageQueueConfig, publish func(eventRecord)) error {
	for _, home := range targets.homes {
		ownerTopic := strings.Replace(config.OwnerTopic, "{ownerId}", home.ID, 1)

		_, err := session.MessageQueue.AddMessageListener(ctx, tuya.AddMessageListenerRequest{
			Request: newRequest(),
			Topic:   ownerTopic,
			Callback: func(_ string, _ any) {
				publish(eventRecord{Type: "account_event", DeviceID: ""})
			},
		})
		if err != nil {
			return safeSDKError("add account event listener", err)
		}

		for _, device := range targets.devices[home.ID] {
			err = addDeviceEventListener(ctx, session, device.ID, publish)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func addDeviceEventListener(ctx context.Context, session *tuya.Session, deviceID string, publish func(eventRecord)) error {
	_, err := session.MessageQueue.AddDeviceListener(ctx, tuya.AddDeviceListenerRequest{
		DeviceID: deviceID,
		Callback: func(_ string, event tuya.Event) {
			if event == nil {
				publish(eventRecord{Type: "device_event", DeviceID: deviceID})

				return
			}

			publish(eventRecord{Type: event.GetEventType(), DeviceID: event.GetDeviceID()})
		},
	})
	if err != nil {
		return safeSDKError("add device event listener", err)
	}

	return nil
}

func waitForEvents(ctx context.Context, events <-chan eventRecord, duration time.Duration, jsonOutput bool, out io.Writer) error {
	if !jsonOutput {
		_, err := fmt.Fprintf(out, "Watching account and device event types for %s; event payloads are hidden.\n", duration)
		if err != nil {
			return newCommandError("could not write command output", "output")
		}
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return newCommandError("event watch interrupted", "interrupted", exitCodeInterrupted)
		case <-timer.C:
			return nil
		case record := <-events:
			err := writeEvent(out, jsonOutput, record)
			if err != nil {
				return err
			}
		}
	}
}

func writeEvent(out io.Writer, jsonOutput bool, record eventRecord) error {
	if jsonOutput {
		encoder := json.NewEncoder(out)

		err := encoder.Encode(record)
		if err != nil {
			return newCommandError("could not write event output", "output")
		}

		return nil
	}

	if record.DeviceID == "" {
		_, err := fmt.Fprintf(out, "Event: %s\n", record.Type)
		if err != nil {
			return newCommandError("could not write event output", "output")
		}

		return nil
	}

	_, err := fmt.Fprintf(out, "Event: %s device=%s\n", record.Type, record.DeviceID)
	if err != nil {
		return newCommandError("could not write event output", "output")
	}

	return nil
}
