# Releasing

## Before the first release

The first version tag remains gated on verifying the provider behavior described
by the library. Follow the pre-release review in the template's
[`docs/releasing.md`](https://github.com/portpowered/go-third-party-template/blob/main/docs/releasing.md),
especially its requirement to replace or remove unverified example endpoints,
resources, and wire responses. Review the pending entries in
[`template-checklist.md`](template-checklist.md) as a separate sign-off. Adding
this workflow does not sign off checklist item 3 or any other open item; leave
each item unchecked until its evidence is reviewed.

The module path is `github.com/portpowered/go-tuya`. The public client package
is `github.com/portpowered/go-tuya/tuya`, so the release workflow compares
package `tuya`. The repository currently has no release tags. On the first tag,
the API compatibility tool reports that there is no prior stable release and
skips the comparison. Later tags compare the public package with the prior
stable tag.

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
`github.com/portpowered/go-tuya/tuya`. Replace `v0.1.0` with the version that
was released.
