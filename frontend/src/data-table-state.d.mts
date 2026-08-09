import type { DataTableFilters, DataTableSort, SortDirection } from './components/data-table';

export function totalPages(total: number, pageSize: number): number;
export function pageWindow(page: number, pages: number, maximum?: number): number[];
export function resultRange(page: number, pageSize: number, total: number, returned: number): { start: number; end: number };
export function nextSort(current: DataTableSort | undefined, key: string, defaultDirection?: SortDirection): DataTableSort;
export function normalizeFilters(filters: Record<string, unknown>): DataTableFilters;
