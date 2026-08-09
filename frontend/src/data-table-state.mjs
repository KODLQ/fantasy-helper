export function totalPages(total, pageSize) {
  if (!Number.isFinite(total) || total <= 0 || !Number.isFinite(pageSize) || pageSize <= 0) return 1;
  return Math.max(1, Math.ceil(total / pageSize));
}

export function pageWindow(page, pages, maximum = 5) {
  const safePages = Math.max(1, Math.trunc(pages) || 1);
  const safePage = Math.min(safePages, Math.max(1, Math.trunc(page) || 1));
  const size = Math.min(safePages, Math.max(1, Math.trunc(maximum) || 1));
  let start = Math.max(1, safePage - Math.floor(size / 2));
  let end = Math.min(safePages, start + size - 1);
  start = Math.max(1, end - size + 1);
  return Array.from({ length: end - start + 1 }, (_, index) => start + index);
}

export function resultRange(page, pageSize, total, returned) {
  if (total <= 0 || returned <= 0) return { start: 0, end: 0 };
  const start = Math.min(total, (Math.max(1, page) - 1) * pageSize + 1);
  return { start, end: Math.min(total, start + returned - 1) };
}

export function nextSort(current, key, defaultDirection = 'asc') {
  if (!current || current.key !== key) return { key, direction: defaultDirection };
  return { key, direction: current.direction === 'asc' ? 'desc' : 'asc' };
}

export function normalizeFilters(filters) {
  return Object.fromEntries(Object.entries(filters).map(([key, value]) => [key, String(value ?? '').trim()]));
}
