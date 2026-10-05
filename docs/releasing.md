# Releasing

## Provider evidence review

No sanitized Tuya account captures are checked in. The HTTP and MQTT schemas
label provider-documented and implementation-derived behavior separately.
The [provider evidence guide](https://portpowered.github.io/go-tuya/docs/guides/provider-evidence/)
explains those labels for SDK users.
See [verification and evidence](verification.md) and the generated
[wire-model inventory](wire-model-inventory.md) when reviewing route coverage.

Follow the pre-release review in the template's
[`docs/releasing.md`](https://github.com/portpowered/go-third-party-template/blob/05e93ff08899414207e9335717e7d7b0190ebd09/docs/releasing.md),
especially its requirement to replace or remove unverified example endpoints,
resources, and wire responses. Review the pending entries in
[`template-checklist.md`](template-checklist.md) as a separate sign-off. The
first clean-history release was `v0.1.0`; `v0.2.0` introduced `pkg/tuya`.
The v0.3.0 migration history below records the compatibility break and package
baseline used for that release.

## v0.3.0 compatibility history

The v0.3.0 release removed nine unused exported legacy wire structs:
`DeviceResponseResult`, `DeviceResponseResultElement`, `HomeResponse`,
`HomeResponseResult`, `HomeResponseResultElement`, `RawMQTTMessage`,
`MessageQueueConfigResponse`, `MessageQueueConfigTopicInfo`, and
`MessageQueueTopicSubscription`.
Use the named client operation results and event interfaces instead. The
`v0.3.0` compatibility policy permits this pre-v1 API break; a `v0.2.x` patch
release would not. Exported encrypted request helper signatures stay the same,
but now reject method/path pairs absent from `api/openapi.yaml`.

The module path is `github.com/portpowered/go-tuya`. The public client package
is `github.com/portpowered/go-tuya/pkg/tuya`. The first tag had no prior stable
release, so the API compatibility tool reported that it had no baseline. For
the `v0.2.0` package move, the compatibility check compares `pkg/tuya` with the
legacy `tuya` package in `v0.1.0`; it allows the resulting API break because
the v0 minor version increased. Later tags compare the public package with the
same path in the prior stable tag.

## Tag and verify

Use semantic version tags of the form `vMAJOR.MINOR.PATCH`; push a reviewed tag
to start `.github/workflows/release.yml`. Before v1, an API break requires a
minor or major increase. A v0 patch release must preserve compatibility. Once
the module is stable, breaking API changes require a major increase. A v2
release also needs the Go module's major-version suffix and matching import
paths updated before tagging.

The workflow validates the module path and public API version policy, builds,
runs race-enabled tests, vet, and replay checks, then fetches the exact tag
through `proxy.golang.org` in a temporary consumer module and compiles the
public `tuya` package. It creates GitHub release notes only after those checks
pass. The release job has `contents: write`; verification runs with read-only
contents permission. API documentation is built and published by the separate
Documentation workflow, so review its Pages status as part of release
readiness.

After publication, verify the package independently with the released tag:

```sh
mkdir module-check
cd module-check
go mod init example.com/tuya-module-check
GOPROXY=https://proxy.golang.org go get github.com/portpowered/go-tuya@vX.Y.Z
```

Then compile a consumer importing
`github.com/portpowered/go-tuya/pkg/tuya`. Replace `vX.Y.Z` with the exact
SDK release being verified.

## Standalone CLI module

`cmd/go-tuya` is a separate Go module at
`github.com/portpowered/go-tuya/cmd/go-tuya`. Release it independently with a
nested module tag of the form `cmd/go-tuya/vMAJOR.MINOR.PATCH`; do not create
that tag until the corresponding public SDK dependency is available from the
Go module proxy. The CLI uses only the public SDK APIs, so do not keep a local
`replace` directive in its released `go.mod`.

For the first CLI release, choose a reviewed version and verify the CLI from
the exact nested-module tag in a clean consumer. The workflow checks the
module path, confirms the SDK dependency resolves through `proxy.golang.org`,
builds and tests the nested module, then runs `go install` for that exact CLI
version and checks `go-tuya --help`. Publish release notes only after those
steps pass. The SDK and CLI tags are separate releases; publishing one does not
publish the other.

Before creating a CLI tag, run from the repository root:

```sh
make check
```

Then verify the installed consumer command with the exact CLI version:

```sh
mkdir cli-module-check
cd cli-module-check
go mod init example.com/tuya-cli-check
GOPROXY=https://proxy.golang.org go install github.com/portpowered/go-tuya/cmd/go-tuya@vX.Y.Z
go-tuya --help
```

The nested version `vX.Y.Z` is resolved from the repository tag
`cmd/go-tuya/vX.Y.Z`. Do not push either SDK or CLI tags as part of the code
change; release owners review and publish them separately.
