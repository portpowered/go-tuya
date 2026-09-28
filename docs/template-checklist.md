# Template migration checklist: go-tuya

Source: `go-third-party-template/docs/library-standards.md` and its release
and verification guides. Sign-off records below are specific to this library.

- [x] **1.** Keep the public client, examples, README, and site independent of any consuming application. Put application adapters and rollout plans in the consuming repository. **Signed off:** package, examples, README, and authored guides contain no backend adapter or rollout instructions. The documentation sources are provider-library focused.
- [x] **2.** Document supported operations, authentication, errors, and transport injection with examples that match the exported API. Add customer-facing operation guides for important workflows, and distinguish verified behavior from synthetic examples and historical references. **Signed off:** README and authentication, device, events, testing, and provider notes cover the public surface; the auth example uses the exported methods. Documentation states that tests are synthetic and makes no live-capture claim.
- [ ] **3.** Show Go version, CI, coverage, release, Go Reference, license, and documentation badges in the README. Replace every example repository value and point badges to live reports. **Partial:** all seven badges use this repository/module. CI uploads coverage to Codecov, but a published coverage report and first GitHub release are not available to verify yet.
- [ ] **4.** Generate the API reference in CI with the shared Fumadocs action and publish it to GitHub Pages. Keep schemas checked in and reviewed, generate schema-defined Go models from them, and check for stale generated output in CI. Publish authored Markdown or MDX operation guides alongside the reference. Replace the template's clearly synthetic widget schema before presenting a provider API as supported behavior. **Not signed off:** this package has no reviewed OpenAPI schema, schema-generated models, stale-generation check, Fumadocs workflow, or Pages site. The request/response surface is broad and partly unverified; creating a synthetic schema would misstate provider behavior. Authored Markdown guides are present.
- [ ] **5.** Run offline build, lint, race, and replay checks before release. Keep real captures sanitized and separate from synthetic fixtures. **Partial:** offline `make lint` and `make check` cover vet, build, and race-enabled tests; tests and event examples are synthetic and no private captures are included. A replay fixture corpus and replay check do not exist.

## Release and history sign-off

- [x] Inspect the exported API and compile a separate consumer example. **Signed off:** reviewed the exported `tuya` API and compiled `examples/auth`, a separate module importing the library through a local replacement.
- [x] Run `make lint` and `make check`. **Signed off:** both commands passed after the migration changes.
- [x] Review the new tree for credentials and private captures. **Signed off:** removed the embedded partially redacted device response and token-printing examples; a filename-and-line-only credential-pattern scan found no remaining non-synthetic values. Tests use synthetic mock values; no capture directory or private capture is present.
- [x] Replace `main` history and remove old tags that retain the old commits. **Signed off:** a new root commit replaced `main`; v1.0.8 through v1.0.10 were deleted. A remote ref check found only the rewritten `main`, and its CI run passed.

Unfinished items remain unchecked until verified. This file is a review record,
not a claim that incomplete checklist items are complete.
