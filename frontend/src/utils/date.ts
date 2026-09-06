// Cross-browser timestamp parser (Safari, Chrome, Firefox, WebKit)
export function parseTimestamp(ts: string | number | undefined): Date | null {
  if (!ts) return null;
  if (typeof ts === 'number') {
    if (ts > 1e11) return new Date(ts);
    return new Date(ts * 1000);
  }
  const str = String(ts).trim();
  if (!str) return null;

  // Numeric string timestamp
  if (/^\d+$/.test(str)) {
    const num = Number(str);
    if (num > 1e11) return new Date(num);
    return new Date(num * 1000);
  }

  // Normalize "2026-08-24 21:37:48 UTC" -> "2026-08-24T21:37:48Z"
  const normalized = str.replace(/\s+UTC$/i, 'Z').replace(' ', 'T');
  const d = new Date(normalized);
  if (!isNaN(d.getTime())) return d;

  // If format is "YYYY-MM-DD HH:mm:ss" without explicit timezone, Sirius default is UTC
  if (/^\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}/.test(str) && !str.endsWith('Z')) {
    const asUtc = new Date(str.replace(' ', 'T') + 'Z');
    if (!isNaN(asUtc.getTime())) return asUtc;
  }

  const fallback = new Date(str);
  if (!isNaN(fallback.getTime())) return fallback;
  return null;
}

// Formats timestamp in user's local browser timezone (YYYY-MM-DD HH:mm:ss)
export function formatToLocalTime(ts: string | number | undefined, includeDate = true): string {
  const d = parseTimestamp(ts);
  if (!d) return typeof ts === 'string' ? ts : '—';

  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  const hours = String(d.getHours()).padStart(2, '0');
  const minutes = String(d.getMinutes()).padStart(2, '0');
  const seconds = String(d.getSeconds()).padStart(2, '0');

  if (!includeDate) {
    return `${hours}:${minutes}:${seconds}`;
  }

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
}

// Returns a human-friendly relative duration (e.g. "4m ago", "2h ago")
export function formatRelativeTime(ts: string | number | undefined): string {
  const d = parseTimestamp(ts);
  if (!d) return '';
  const now = Date.now();
  const diffSec = Math.floor((now - d.getTime()) / 1000);
  if (diffSec < 0 || diffSec < 5) return 'just now';
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays}d ago`;
}
