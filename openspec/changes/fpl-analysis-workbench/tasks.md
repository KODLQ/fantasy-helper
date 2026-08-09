## 1. Roadmap and ownership

- [ ] 1.1 Record the four child changes as the authoritative owners of their domain specs, APIs, formulas, persistence, UI, and Playwright tests.
- [ ] 1.2 Add repository-level dependency metadata and a documented delivery order for warehouse, authentication, manager/league, analysis children, and optimal-team learning.
- [ ] 1.3 Remove or reject duplicate parent domain specs during OpenSpec review.
- [ ] 1.4 Add and maintain `openspec/formulas.yaml` with formula IDs, versions, normalization rules, owners, consumers, and compatibility tests.

## 2. Cross-workbench compatibility

- [ ] 2.1 Add contract tests proving every child uses the common `{data,meta}` success and `{error,meta}` error envelopes.
- [ ] 2.2 Add compatibility tests for freshness states, snapshot provenance, coverage/omissions, request IDs, algorithm versions, and ruleset versions.
- [ ] 2.3 Add cross-child authorization tests proving user-owned manager, planning, alert, and analysis records cannot cross accounts.
- [ ] 2.4 Add handoff tests for manager snapshot → league/autopsy, transfer simulation → planning confirmation, and analysis → optimal-team comparison.
- [ ] 2.5 Add cross-child formula-contract tests proving fixture ease, points differences, recommendation metrics, alert thresholds, and storyline ranking resolve to registry versions.

## 3. Cross-workbench browser acceptance

- [ ] 3.1 Add Playwright navigation coverage for all child pages, authenticated entry, deep links, back/forward behavior, and session-expired recovery.
- [ ] 3.2 Add Playwright coverage for shared loading, empty, stale, partial, unavailable, validation, and API-error states.
- [ ] 3.3 Add an accessibility/button matrix covering every cross-child navigation and handoff control.
- [ ] 3.4 Run the seeded, network-isolated browser suite with deterministic clocks and verify no private data leaks between users.

## 4. Release documentation

- [ ] 4.1 Document the dependency graph, authoritative ownership, release gates, and rollback boundaries.
- [ ] 4.2 Publish one workbench glossary linking each domain metric to its child specification.
