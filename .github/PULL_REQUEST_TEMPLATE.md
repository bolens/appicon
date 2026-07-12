---
name: Pull request
about: Checklist for contributors
---

## Summary

<!-- Why this change exists (1–3 bullets). -->

## Test plan

- [ ] `make check-fast`
- [ ] `make check` (if tools available)
- [ ] Docs / consumer-contract updated when behavior changes
- [ ] No live-network-only tests; fixtures / `httptest` preferred
