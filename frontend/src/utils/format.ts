// Format arbitrary number with locale commas and optional decimal precision
export function formatNumber(amount: number | string | undefined, decimals?: number): string {
  if (amount === undefined || amount === null || amount === '') return '0';

  let num: number;
  if (typeof amount === 'string') {
    const cleaned = amount.replace(/,/g, '').trim();
    num = parseFloat(cleaned);
  } else {
    num = amount;
  }

  if (isNaN(num)) return '0';

  if (decimals !== undefined) {
    return num.toLocaleString(undefined, {
      minimumFractionDigits: decimals,
      maximumFractionDigits: decimals,
    });
  }

  return num.toLocaleString();
}

// Format duration/uptime in readable string (e.g. 2d 14h 32m)
export function formatUptime(seconds?: number): string {
  if (!seconds || seconds <= 0) return 'Just started';
  const d = Math.floor(seconds / (3600 * 24));
  const h = Math.floor((seconds % (3600 * 24)) / 3600);
  const m = Math.floor((seconds % 3600) / 60);

  const parts = [];
  if (d > 0) parts.push(`${d}d`);
  if (h > 0) parts.push(`${h}h`);
  if (m > 0) parts.push(`${m}m`);
  return parts.length > 0 ? parts.join(' ') : '< 1m';
}

// Format XPX amount into clean, readable compact notation (e.g. 6.3M XPX, 100k XPX)
export function formatXPX(amount: number | string | undefined, unit = 'XPX'): string {
  if (amount === undefined || amount === null || amount === '') return `0 ${unit}`.trim();
  
  let num: number;
  if (typeof amount === 'string') {
    const cleaned = amount.replace(/xpx/gi, '').replace(/,/g, '').trim();
    num = parseFloat(cleaned);
  } else {
    num = amount;
  }

  if (isNaN(num)) return `0 ${unit}`.trim();

  let formattedNum: string;

  if (Math.abs(num) >= 1_000_000_000) {
    const val = (num / 1_000_000_000).toFixed(2).replace(/\.?0+$/, '');
    formattedNum = `${val}B`;
  } else if (Math.abs(num) >= 1_000_000) {
    const val = (num / 1_000_000).toFixed(2).replace(/\.?0+$/, '');
    formattedNum = `${val}M`;
  } else if (Math.abs(num) >= 100_000) {
    const val = (num / 1_000).toFixed(1).replace(/\.?0+$/, '');
    formattedNum = `${val}k`;
  } else if (Math.abs(num) >= 1_000) {
    formattedNum = num.toLocaleString(undefined, { maximumFractionDigits: 2 });
  } else if (num === 0) {
    formattedNum = '0';
  } else if (Math.abs(num) < 0.001) {
    formattedNum = num.toFixed(4);
  } else {
    formattedNum = num.toLocaleString(undefined, { maximumFractionDigits: 3 });
  }

  return unit ? `${formattedNum} ${unit}` : formattedNum;
}

// Format XPX amount strictly in Millions (e.g. 6.72 M XPX)
export function formatXPXInMillions(amount: number | string | undefined, unit = 'XPX'): string {
  if (amount === undefined || amount === null || amount === '') return `0.00 M ${unit}`.trim();
  
  let num: number;
  if (typeof amount === 'string') {
    const cleaned = amount.replace(/xpx/gi, '').replace(/,/g, '').trim();
    num = parseFloat(cleaned);
  } else {
    num = amount;
  }

  if (isNaN(num)) return `0.00 M ${unit}`.trim();

  const millions = num / 1_000_000;
  const formatted = millions.toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });

  return unit ? `${formatted} M ${unit}` : `${formatted} M`;
}

// Format 50-character hex address or unformatted Base32 address into Sirius formatted Base32 (e.g. XA4KTR-5L2KXO-...)
export function formatSiriusAddress(addr?: string): string {
  if (!addr) return '—';
  const clean = addr.replace(/-/g, '').trim();
  
  // If 50-char hex
  if (clean.length === 50 && /^[0-9a-fA-F]+$/.test(clean)) {
    const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
    let bits = '';
    for (let i = 0; i < clean.length; i += 2) {
      const byte = parseInt(clean.substring(i, i + 2), 16);
      bits += byte.toString(2).padStart(8, '0');
    }
    let base32 = '';
    for (let i = 0; i < bits.length; i += 5) {
      const chunk = bits.substring(i, i + 5);
      base32 += alphabet[parseInt(chunk, 2)];
    }
    const parts = [];
    for (let i = 0; i < base32.length; i += 6) {
      parts.push(base32.substring(i, i + 6));
    }
    return parts.join('-');
  }

  // If already 40-char Base32 without hyphens
  if (clean.length === 40 && /^[A-Z2-7]+$/i.test(clean)) {
    const upper = clean.toUpperCase();
    const parts = [];
    for (let i = 0; i < upper.length; i += 6) {
      parts.push(upper.substring(i, i + 6));
    }
    return parts.join('-');
  }

  return addr;
}

// Format raw byte count into human-readable notation (e.g. 14.2 MB, 1.5 GB)
export function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const clampedIdx = Math.min(i, sizes.length - 1);
  return `${parseFloat((bytes / Math.pow(k, clampedIdx)).toFixed(2))} ${sizes[clampedIdx]}`;
}
