import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';

const roots = ['src', 'tests', 'e2e'];
const extensions = new Set(['.ts', '.tsx', '.mjs']);
const failures = [];

async function walk(path) {
  for (const entry of await readdir(path, { withFileTypes: true })) {
    const fullPath = join(path, entry.name);
    if (entry.isDirectory()) await walk(fullPath);
    else if (extensions.has(entry.name.slice(entry.name.lastIndexOf('.')))) {
      const content = await readFile(fullPath, 'utf8');
      if (content.includes('\r\n')) failures.push(`${fullPath}: CRLF line endings`);
      if (content.length > 0 && !content.endsWith('\n')) failures.push(`${fullPath}: missing final newline`);
      if (content.split('\n').some((line) => /[ \t]+$/.test(line))) failures.push(`${fullPath}: trailing whitespace`);
    }
  }
}

for (const root of roots) await walk(root);
if (failures.length) {
  console.error(failures.join('\n'));
  process.exit(1);
}
console.log('Frontend formatting invariants passed.');
