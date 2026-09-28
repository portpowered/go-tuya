# Releasing

## Provider evidence review

The maintainer reports that the implemented API flows worked with real Tuya
accounts. Those tests were not documented with sanitized exchanges or
route-by-route provenance, so the repository cannot independently reproduce
or inspect them. The README and [provider evidence guide](https://portpowered.github.io/go-tuya/docs/guides/provider-evidence)
record this limit. The OpenAPI inventory includes the routes used by the client
and labels each operation as provider-documented or implementation-derived.
Keep that distinction in release notes and customer documentation.

Follow the pre-release review in the template's
[`docs/releasing.md`](https://github.com/portpowered/go-third-party-template/blob/main/docs/releasing.md),
especially its requirement to replace or remove unverified example endpoints,
resources, and wire responses. Review the pending entries in
[`template-checklist.md`](template-checklist.md) as a separate sign-off. The
first clean-history release was `v0.1.0`; `v0.2.0` introduced `pkg/tuya`.
Checklist item 3 records the latest badge and Go Reference checks.

## v0.3.0 migration

The next release removes nine unused exported legacy wire structs:
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
GOPROXY=https://proxy.golang.org go get github.com/portpowered/go-tuya@v0.2.0
```

Then compile a consumer importing
`github.com/portpowered/go-tuya/pkg/tuya`. Replace `v0.2.0` with a later
version when checking a subsequent release.
