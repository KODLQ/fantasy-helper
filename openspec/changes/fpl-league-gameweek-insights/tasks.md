## 1. Contracts and calculations

- [ ] 1.1 Implement league summary/comparison and gameweek autopsy schemas using the common response contract.
- [ ] 1.2 Implement deterministic member selection and snapshot reuse with coverage metadata.
- [ ] 1.3 Implement overlap, differential, point-gap, captain, bench, transfer-hit, and substitution formulas with versioned tests.
- [ ] 1.4 Add completed, provisional, partial, stale, and unavailable state fixtures.

## 2. API and UI

- [ ] 2.1 Add scoped league summary/comparison/autopsy routes with authorization and input validation.
- [ ] 2.2 Build league overview, rival selection, comparison table, and gameweek autopsy pages.
- [ ] 2.3 Show selected/omitted members, exact source snapshots, formula/version details, and warnings.
- [ ] 2.4 Prevent different-season/gameweek snapshots from being compared.

## 3. Verification

- [ ] 3.1 Add unit/integration contract tests for all formulas and incomplete-input behavior.
- [ ] 3.2 Add Playwright tests for navigation, rival filters, member limits, overlap/difference displays, autopsy details, loading/empty/error/stale/provisional states, and keyboard-accessible controls.
- [ ] 3.3 Run the full browser suite against seeded public and manager snapshots with no network dependency.
