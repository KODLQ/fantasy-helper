import { ReactNode } from 'react';
import { nextSort, pageWindow, resultRange, totalPages } from '../data-table-state.mjs';

export type SortDirection = 'asc' | 'desc';
export type DataTableSort = { key: string; direction: SortDirection };
export type DataTableFilters = Record<string, string>;

type FilterBase = { label: string };
export type TextFilter = FilterBase & { type: 'text'; id: string; placeholder?: string };
export type SelectFilter = FilterBase & { type: 'select'; id: string; options: { value: string; label: string }[] };
export type NumberFilter = FilterBase & { type: 'number'; id: string; min?: number; step?: number; placeholder?: string };
export type NumberRangeFilter = FilterBase & { type: 'number-range'; minId: string; maxId: string; min?: number; step?: number };
export type DataTableFilter = TextFilter | SelectFilter | NumberFilter | NumberRangeFilter;

export type DataTableColumn<T> = {
  id: string;
  header: ReactNode;
  cell: (row: T, index: number) => ReactNode;
  className?: string;
  sortKey?: string;
  defaultSortDirection?: SortDirection;
  filter?: DataTableFilter;
};

export type DataTablePaginationProps = {
  page: number;
  pageSize: number;
  total: number;
  returned: number;
  pageSizeOptions?: number[];
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
};

type DataTableProps<T> = Omit<DataTablePaginationProps, 'returned'> & {
  caption: string;
  columns: DataTableColumn<T>[];
  rows: T[];
  rowKey: (row: T) => string | number;
  sort?: DataTableSort;
  filters: DataTableFilters;
  loading?: boolean;
  error?: string;
  loadingMessage?: string;
  emptyMessage?: string;
  onSortChange: (sort: DataTableSort) => void;
  onFilterChange: (id: string, value: string) => void;
  onClearFilters: () => void;
  testId?: string;
};

export function DataTable<T>({ caption, columns, rows, rowKey, sort, filters, loading = false, error = '', loadingMessage = 'Loading results…', emptyMessage = 'No results match those filters.', onSortChange, onFilterChange, onClearFilters, testId, ...pagination }: DataTableProps<T>) {
  const filterColumns = columns.filter((column) => column.filter);
  const activeFilters = filterColumns.some((column) => filterIDs(column.filter!).some((id) => Boolean(filters[id])));
  const stateMessage = loading ? loadingMessage : error || (rows.length === 0 ? emptyMessage : '');

  return <div className="data-table" data-testid={testId}>
    {filterColumns.length > 0 && <div className="data-table-toolbar"><span>{activeFilters ? 'Filters applied' : 'Filter relevant columns'}</span><button className="text-button" disabled={!activeFilters} onClick={onClearFilters}>Clear filters</button></div>}
    <div className="table-wrap data-table-scroll"><table><caption className="visually-hidden">{caption}</caption><thead><tr>{columns.map((column) => {
      const active = Boolean(column.sortKey && sort?.key === column.sortKey);
      const ariaSort = active ? (sort!.direction === 'asc' ? 'ascending' : 'descending') : undefined;
      return <th key={column.id} className={column.className} aria-sort={ariaSort}>{column.sortKey ? <button className={`data-table-sort ${active ? 'active' : ''}`} onClick={() => onSortChange(nextSort(sort, column.sortKey!, column.defaultSortDirection ?? 'asc'))}>{column.header}<span aria-hidden="true">{active ? (sort!.direction === 'asc' ? '↑' : '↓') : '↕'}</span></button> : column.header}</th>;
    })}</tr>{filterColumns.length > 0 && <tr className="data-table-filters">{columns.map((column) => <th key={column.id}>{column.filter ? <FilterControl filter={column.filter} values={filters} onChange={onFilterChange} /> : null}</th>)}</tr>}</thead><tbody>{stateMessage ? <tr><td colSpan={columns.length} className={`table-state ${error ? 'error-state' : ''}`} role={error ? 'alert' : 'status'}>{stateMessage}</td></tr> : rows.map((row, index) => <tr key={rowKey(row)}>{columns.map((column) => <td key={column.id} className={column.className}>{column.cell(row, index)}</td>)}</tr>)}</tbody></table></div>
    <DataTablePagination {...pagination} returned={rows.length} />
  </div>;
}

export function DataTablePagination({ page, pageSize, total, returned, pageSizeOptions = [10, 25, 50, 100], onPageChange, onPageSizeChange }: DataTablePaginationProps) {
  const pages = totalPages(total, pageSize);
  const current = Math.min(pages, Math.max(1, page));
  const range = resultRange(current, pageSize, total, returned);
  return <div className="table-footer data-table-pagination"><span>Showing {range.start}–{range.end} of {total} results</span><label>Rows<select aria-label="Rows per page" value={pageSize} onChange={(event) => onPageSizeChange(Number(event.target.value))}>{pageSizeOptions.map((size) => <option key={size} value={size}>{size}</option>)}</select></label><nav aria-label="Table pagination"><button disabled={current === 1} aria-label="First page" onClick={() => onPageChange(1)}>«</button><button disabled={current === 1} aria-label="Previous page" onClick={() => onPageChange(current - 1)}>←</button>{pageWindow(current, pages).map((candidate) => <button key={candidate} className={candidate === current ? 'page-selected' : ''} aria-label={`Page ${candidate}`} aria-current={candidate === current ? 'page' : undefined} disabled={candidate === current} onClick={() => onPageChange(candidate)}>{candidate}</button>)}<button disabled={current === pages} aria-label="Next page" onClick={() => onPageChange(current + 1)}>→</button><button disabled={current === pages} aria-label="Last page" onClick={() => onPageChange(pages)}>»</button></nav></div>;
}

function FilterControl({ filter, values, onChange }: { filter: DataTableFilter; values: DataTableFilters; onChange: (id: string, value: string) => void }) {
  if (filter.type === 'select') return <select aria-label={filter.label} value={values[filter.id] ?? ''} onChange={(event) => onChange(filter.id, event.target.value)}><option value="">All</option>{filter.options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>;
  if (filter.type === 'number-range') return <div className="range-filter"><input aria-label={`${filter.label} minimum`} type="number" min={filter.min} step={filter.step} placeholder="Min" value={values[filter.minId] ?? ''} onChange={(event) => onChange(filter.minId, event.target.value)} /><span>–</span><input aria-label={`${filter.label} maximum`} type="number" min={filter.min} step={filter.step} placeholder="Max" value={values[filter.maxId] ?? ''} onChange={(event) => onChange(filter.maxId, event.target.value)} /></div>;
  if (filter.type === 'number') return <input aria-label={filter.label} type="number" min={filter.min} step={filter.step} placeholder={filter.placeholder ?? 'Min'} value={values[filter.id] ?? ''} onChange={(event) => onChange(filter.id, event.target.value)} />;
  return <input aria-label={filter.label} type="search" placeholder={filter.placeholder ?? 'Filter…'} value={values[filter.id] ?? ''} onChange={(event) => onChange(filter.id, event.target.value)} />;
}

function filterIDs(filter: DataTableFilter) {
  return filter.type === 'number-range' ? [filter.minId, filter.maxId] : [filter.id];
}
