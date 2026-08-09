## Why

The player research table hard-codes its headers, rows, filters, sort indicator, and disabled pagination, making it difficult to reuse consistent tabular behavior in upcoming league, alert, and analysis screens. A shared controlled data-table component is needed now so large FPL datasets can use accessible server-side pagination, sorting, and relevant column filters without duplicating UI state or query logic.

## What Changes

- Add a reusable, typed React data-table component with controlled rows, column definitions, sort state, filter controls, pagination, loading, empty, and error states.
- Support text, select, minimum-number, and numeric-range filters only on columns that declare those controls.
- Make sortable headers keyboard-accessible and expose the active direction through `aria-sort` and a visible indicator.
- Integrate the component into player research with server-side page, page-size, sort, and filter query parameters.
- Return authoritative season-scoped team metadata with player research results so player rows, cards, and club filters never derive labels from demo mappings or numeric IDs.
- Preserve the player card view while sharing the same server-side query state and pagination.
- Add component-focused tests and Playwright coverage for filtering, bidirectional sorting, page-size changes, and page navigation.

## Capabilities

### New Capabilities

- `reusable-data-table`: Controlled, accessible tabular rendering with configurable columns, filters, sorting, pagination, and data states.

### Modified Capabilities

- `player-research`: Replace placeholder pagination and fixed sorting controls with functional server-backed table interactions and relevant per-column filters.

## Impact

- Adds a shared component and styles under `frontend/src/components`.
- Refactors `frontend/src/features/research.tsx` to use controlled table state and enriches the paginated `/api/v1/players` response with authoritative player-team relationships and the selected season's club catalogue.
- Extends backend, frontend unit, and Playwright coverage without adding a third-party table dependency; the player-list response gains additive team metadata and an explicit player-team item shape.
