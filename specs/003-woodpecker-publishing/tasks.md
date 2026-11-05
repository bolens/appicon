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
