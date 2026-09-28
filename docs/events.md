# Message queue events

`MessageQueue` obtains the broker configuration, registers account or device
listeners, then starts and stops the sharing message queue. Use callbacks to
handle events without logging the full payload; messages can contain device
identifiers and state values.

```go
queue := client.MessageQueue
config, err := queue.GetMessageQueueConfig(ctx)
if err != nil {
    return err
}

_, err = queue.AddDeviceListener(ctx, tuya.AddDeviceListenerRequest{
    DeviceID: deviceID,
    Callback: func(_ string, event tuya.Event) {
        switch event.(type) {
        case *tuya.DeviceStateChangeEvent:
            // Handle the state change without logging private event data.
        case *tuya.DeviceManagementEvent:
            // Handle a management event.
        }
    },
})
if err != nil {
    return err
}

if _, err := queue.Start(ctx, tuya.MessageQueueStartRequest{}); err != nil {
    return err
}
defer queue.Stop(context.Background(), tuya.MessageQueueStopRequest{})
```

The parser recognizes device-state protocol 4 messages and device-management
protocol 20 messages; unknown management event codes are returned as generic
management events. This parsing behavior is covered by synthetic unit tests.
It does not establish the provider's complete event catalog or guarantee that
all accounts and devices publish each event. The caller should stop the queue
when its listener lifetime ends and should handle callback concurrency in its
own application.

Queue connection and listener state belongs to the account's `Session`. Reuse
the session's `MessageQueue` for that account and call `Session.Close` when its
lifetime ends.
