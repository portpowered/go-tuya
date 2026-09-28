# API schema and generated wire models

[`api/openapi.yaml`](../api/openapi.yaml) contains the reviewed subset for
Tuya's documented `GET /v1.0/devices` route. The path and query parameters are
documented by Tuya; the result object fields and the sample record are recorded
with optional fields because the reference does not define their requiredness
or an exhaustive record shape. The route is used by `QueryDevices`, but this
does not verify the library's Device Sharing signing, encryption, or response
handling. The `/v1.0/m/...` routes remain outside the schema.

The internal Go wire models are generated from that file with
[`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen) v2.8.0. From the
repository root, use Go 1.25 or later:

```sh
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 \
  -config tuya/internal/wire/config.yaml api/openapi.yaml
```

The generated result model is consumed by `DevicesService.QueryDevices`. The
adapter preserves additional response properties before converting into the
existing public device result. The Documentation workflow regenerates the
models and fails if the checked-in output is stale. It also builds the
Fumadocs site from this schema and `docs/guides/`. The existing public request
keeps `StartTime` and `EndTime` as strings even though Tuya documents these
parameters as `Long`; their representation inside the custom encrypted request
has not been verified.
