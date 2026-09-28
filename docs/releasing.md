# Releasing

## Provider evidence review

The maintainer reports that the implemented API flows worked with their real
Tuya account(s). Those tests were not documented with sanitized exchanges or
route-by-route provenance, so the repository cannot independently reproduce
or inspect them. The README and [provider evidence review](provider-evidence.md)
record this limit. The checked-in provider schema covers the exact documented
`GET /v1.0/devices` route; the `/v1.0/m/...` routes remain private routes without
matching public wire specifications, despite the reported account-test success.
Keep that distinction in release notes and customer documentation.

Follow the pre-release review in the template's
[`docs/releasing.md`](https://github.com/portpowered/go-third-party-template/blob/main/docs/releasing.md),
especially its requirement to replace or remove unverified example endpoints,
resources, and wire responses. Review the pending entries in
[`template-checklist.md`](template-checklist.md) as a separate sign-off. The
first clean-history release is `v0.1.0`; checklist item 3 records the live
badge and Go Reference verification separately.

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
GOPROXY=https://proxy.golang.org go get github.com/portpowered/go-tuya@v0.1.0
```

Then compile a consumer importing
`github.com/portpowered/go-tuya/pkg/tuya`. Replace `v0.1.0` with the version that
was released.
