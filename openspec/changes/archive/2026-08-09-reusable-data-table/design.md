## Context

Player research already calls a server endpoint that accepts page, page size, sort direction, and a broad set of filters, but the React view hard-codes its table and always requests page one with 25 rows. Its visible pagination buttons are permanently disabled and sorting is controlled by a separate select rather than the columns users are inspecting. Upcoming league and analysis screens will need the same interaction pattern.

The shared component must remain compatible with the current dependency-light React application, preserve server-side query semantics for large datasets, and be usable with domain-specific cell renderers and actions.

## Goals / Non-Goals

**Goals:**

- Provide a generic TypeScript `DataTable<T>` with typed column definitions and row identity.
- Keep sorting, filters, page, and page size controlled by the consuming feature.
- Render only explicitly declared filter controls and sortable headers.
- Provide accessible sort state, labels, loading/error/empty rows, result counts, and pagination controls.
- Integrate player research with the existing server-side query contract and retain the card view.

**Non-Goals:**

- Client-side sorting or filtering of partial server result pages.
- Column resizing, reordering, pinning, selection, virtualization, or CSV export.
- A third-party grid dependency or backend API redesign.
- Migrating every existing list in this change.

## Decisions

### 1. Use a controlled, headless-data component

`DataTable<T>` receives rows plus `sort`, `filters`, `page`, `pageSize`, and callback props. It renders the interaction surface but never fetches or silently transforms rows. This prevents a reusable component from accidentally sorting only the current server page and lets URL/API state be introduced later without replacing the component.

An internal stateful grid library was considered, but it would duplicate existing query state and add substantial dependency and styling weight for the required feature set.

### 2. Declare behavior per column

Each column declares an ID, header, cell renderer, optional sort key, and optional filter descriptor. Supported descriptors are text, select, minimum number, and number range. Action and display-only columns omit sort and filter declarations, so irrelevant controls never appear.

### 3. Keep server query keys in the feature adapter

The generic component works with string sort keys and filter values. Player research maps those values to the established API names (`search`, `position`, `minPrice`, `maxPrice`, `minForm`, `minPoints`, `minMinutes`, `minValue`, and `status`). Any filter, sort, or page-size change resets the page to one; page navigation alone preserves the current query.

### 4. Use authoritative season team metadata

The player endpoint returns each paged item as its source player record paired with its source team record, plus the complete selected-season team catalogue for filter options. The frontend does not translate IDs through fixed arrays, invent generic club names, or assume the same clubs across seasons. A missing player-team relationship is a server data-integrity error rather than a plausible-looking UI fallback.

Player names, positions, prices, form, points, minutes, value, availability, and club identity are rendered directly from the selected season's API response. Dynamic player counts use response totals rather than demo constants. The retained demo dataset remains valid test data, but no production presentation logic knows its names.

### 5. Make pagination deterministic

The component computes total pages from server-provided `total` and `pageSize`, disables boundary actions, provides first/previous/next/last controls, and renders a bounded page-number window around the current page. The feature accepts the API's resolved `page` and `pageSize`, preventing UI drift if the server clamps values.

### 6. Preserve accessibility and explicit data states

Sortable headers are buttons inside `th` elements with `aria-sort`. Every filter has a column-specific accessible label. Loading, errors, and empty results use one full-width row. Pagination exposes textual result range and labelled controls; the table receives an accessible caption.

## Risks / Trade-offs

- [Many filter inputs can make narrow layouts dense] → Render compact controls only for relevant columns and allow horizontal table overflow.
- [Rapid filters can issue excessive requests] → Preserve the existing debounce and abort stale requests.
- [A page can become invalid after filtering] → Reset to page one for query changes and clamp against the returned total-page count.
- [Generic cell renderers can reduce consistency] → Centralize table structure and controls while leaving domain formatting intentionally feature-owned.
- [A player references missing team metadata] → Reject the inconsistent response with a non-disclosing data-integrity error instead of displaying a fabricated club label.

## Migration Plan

1. Add the shared component and styles without changing existing screens.
2. Replace the player research table and footer while preserving its domain cell renderers and card view.
3. Enable real server pagination and column query controls.
4. Verify keyboard sorting, filtering, pagination, cards, and existing player actions.

Rollback restores the former research markup and removes the unused component; no data or API migration is required.

## Open Questions

None for this delivery. Virtualization and URL-persisted table state can be evaluated when result sizes and navigation requirements justify them.
