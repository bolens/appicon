# Tasks and acceptance evidence

- [x] Inventory existing jobs, publication consumers and host-specific exceptions.
- [x] Draft native validation, build, guarded publication and Pages branch paths.
- [x] Keep existing Actions enabled by default with explicit migration flags.
- [x] Complete local trust-boundary tests, workflow/security lint and source contract checks.
- [ ] Complete independent review and resolve findings.
- [ ] Verify exact-head native GitHub PR and main pipelines.
- [ ] Root provisions scoped credentials, migrates settings and selects one publisher.
- [ ] Verify signing/publication only during a real authorized release.

Unchecked tasks are not claimed as completed by source review or YAML parsing.

- [x] Prepare the generated validation prefix, drift gate and bounded denial tests.
- [x] Prove empty-secret validation compilation and unchanged workflow identity
  using the actual Woodpecker 3.20.1 compiler; retain full-profile credential denial.
- [ ] Deliver the final profile and broker-client source through current-head checks.
- [ ] Verify live profile/flag/cancellation readbacks and a new genuine main push
  before retiring duplicate validation; do not use cached restart as acceptance.

## Linux development-environment parity follow-up

- [x] Add the owning locked Linux development gate to native validation and its generated profile.
- [ ] Verify local hooks, workflow lint and actual compiler/event/profile fixtures.
- [ ] Complete independent review and exact-head native PR proof.
- [x] Add a separate default-off hosted Linux guard without changing macOS or Docker gates.
- [ ] Root enables and reads back `WOODPECKER_NIX_CI` only after exact PR and merged-main proof.
- [ ] Verify merged-main native execution and reconcile accepted source to Gitea.

Cold Nix storage and runtime acceptance remain required; no source review claims
that the native development environment has executed successfully.
