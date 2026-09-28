# Tuya wire-contract evidence review

This matrix compares this client's routes with Tuya's published documentation. It was reviewed on 2026-09-28. A related public API is not evidence for a different Device Sharing route or response shape. Offline tests use synthetic responses and establish only what this implementation sends and accepts.

| Client operation | Client route | Official documentation | Status |
| --- | --- | --- | --- |
| `QueryDevices` | `GET /v1.0/devices` | [Get Device List](https://developer.tuya.com/en/docs/cloud/7ccd629a6c?id=Kconjb15xpvd3) documents this path, its query parameters, `result.total`, `result.devices`, and `result.last_id`. | Exact documented route. `api/openapi.yaml` models only the documented query/result subset and fields shown in Tuya's sample; response-field requiredness and exhaustive device shape are unspecified. This does not verify the library's Device Sharing encryption/signing transport. |
| QR login | `/v1.0/m/life/home-assistant/qrcode/tokens` | [QR Code-Based Login Authorization](https://developer.tuya.com/en/docs/app-development/userqrlogin?id=Kceugtdh6y3h9) describes SDK calls and QR payloads, not this HTTP wire contract. | Unverified route and response. |
| Token refresh | `/v1.0/m/token/{refresh_token}` | [Refresh Token](https://developer.tuya.com/en/docs/cloud/80bb968f1d?id=Ka7kjv3j8jgvr) describes `GET /v1.0/token/{refresh_token}` and a different result shape. | Related API; no exact match. |
| Home list | `/v1.0/m/life/users/homes` | [Query Home List](https://developer.tuya.com/en/docs/cloud/f5dd40ed14?id=Kawfjh9hpov1n) describes `GET /v1.0/users/{uid}/homes`. | Related API; no exact match. |
| Home devices | `/v1.0/m/life/ha/home/devices` | [Query Devices in Home](https://developer.tuya.com/en/docs/cloud/d7ee73aadb?id=Kawfjer0wkt2a) describes `GET /v1.0/homes/{home_id}/devices`. | Related API; no exact match. |
| Device status | `/v1.0/m/life/devices/{device_id}/status` | [Get the status of a single device](https://developer.tuya.com/en/docs/cloud/1ef1a3044b?id=Kconf2usgnfwo) describes `GET /v1.0/iot-03/devices/{device_id}/status` and a different result shape. | Related API; no exact match. |
| RTC session | `/v1.0/m/life/ipc/{device_id}/webrtc/session` | [WebRTC guide](https://developer.tuya.com/en/docs/iot/webrtc?id=Kacsd4x2hl0se) describes a different access and signaling flow. | Unverified route and response. |

The documented list API types `start_time` and `end_time` as `Long`; the
existing public `QueryDevicesRequest` exposes them as strings. The library sends
them through its custom encrypted parameter path, so their on-wire encoding in
that flow remains unverified.

The checked-in OpenAPI subset documents only `QueryDevices`; its generated result type is used by that method, and CI checks for generated-file drift. This does not sign off the client's authentication/encryption envelope or any other operation. For every other claimed operation, collect a matching current official wire specification or a sanitized live exchange before adding it to the provider schema. Record request and response fields, statuses, redactions, and provenance. Keep unsupported and placeholder operations out of the published provider schema.
