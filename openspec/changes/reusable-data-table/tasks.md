## 1. Shared component

- [x] 1.1 Define typed row, column, controlled sort, filter, and pagination contracts.
- [x] 1.2 Implement accessible headers, declared filter controls, data-state rows, and row rendering.
- [x] 1.3 Implement result ranges, page-size selection, bounded page numbers, and boundary navigation.
- [x] 1.4 Add reusable table, filter, sorting, pagination, focus, and responsive-overflow styles.

## 2. Player research integration

- [x] 2.1 Move player query state to page, page size, bidirectional sort, and all supported relevant column filters.
- [x] 2.2 Integrate the shared data table while preserving player cells, actions, cards, debounce, cache, stale-request cancellation, and freshness behavior.
- [x] 2.3 Reset page one for filter, sort, and page-size changes and reconcile server-resolved pagination.

## 3. Verification

- [x] 3.1 Add component/unit tests for page windows, result ranges, boundary states, sort transitions, and filter normalization.
- [x] 3.2 Add Playwright coverage for per-column filters, ascending/descending sorting, page-size changes, and page navigation.
- [x] 3.3 Run frontend verification, full Playwright regression, strict OpenSpec validation, and portfolio ownership/dependency validation.
