# Device queries and commands

Use the home and device query methods to obtain identifiers and inspect a
device's reported functions before sending commands. A typical read flow is:

```go
homes, err := client.HomeService.QueryHomes(ctx, tuya.QueryHomesRequest{})
if err != nil {
    return err
}
if len(homes.Results) == 0 {
    return errors.New("no homes found")
}

devices, err := client.DevicesService.QueryDevicesByHome(ctx, tuya.QueryDevicesByHomeRequest{
    HomeID: homes.Results[0].ID,
})
if err != nil {
    return err
}
```

The device service also exposes status, specification, detail, log, factory
information, sub-device, and user-related queries. Returned models can include
private fields such as local keys, account identifiers, location, or device
state. Avoid logging whole response structs; select only the fields your
application needs and redact identifiers in diagnostics.

## Send a command

Command codes are device-specific. Confirm that the device's specification
includes the requested code and value before using `SendCommands`. The
following example sends a live power-on command to one device:

```go
_, err := client.DevicesService.SendCommands(ctx, tuya.SendCommandsRequest{
    DeviceID: deviceID,
    Commands: []tuya.Command{{Code: "switch_led_1", Value: true}},
})
if err != nil {
    return err
}
```

This operation changes a real device when run with live credentials. Tests use
mock HTTP servers and do not exercise a Tuya account. See
[`examples/command`](../examples/command/command.go) for environment-based
configuration; review the command and target before running it.

The package does not implement local-network control. Region selection applies
to the cloud API base URL and can be configured with `WithRegion` or
`WithCloudAPIURL` when constructing the reusable client. Create a per-account
session with `Client.NewSession` before making requests.
