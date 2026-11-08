// Number, money and date formatting. Everything returns an em dash for values that cannot be shown,
// so a missing number never renders as "NaN" or "undefined".

export const NOT_AVAILABLE = "\u2014";

const usd0 = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 });
const usd2 = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", minimumFractionDigits: 2, maximumFractionDigits: 2 });

function isNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

/** $1,234 (or $1,234.50 with cents). */
export function formatMoney(value: number | null | undefined, options: { cents?: boolean } = {}): string {
  if (!isNumber(value)) return NOT_AVAILABLE;
  if (options.cents) return usd2.format(Math.abs(value) < 0.005 ? 0 : value);
  return usd0.format(Math.abs(value) < 0.5 ? 0 : value);
}

/** $999, $1.2K, $15.6K, $1.5M, $2B: for tiles with little room. */
export function formatCompactMoney(value: number | null | undefined): string {
  if (!isNumber(value)) return NOT_AVAILABLE;
  const abs = Math.abs(value);
  if (abs < 1_000) return formatMoney(value);

  const units: [number, string][] = [
    [1_000, "K"],
    [1_000_000, "M"],
    [1_000_000_000, "B"],
  ];
  let index = units.reduce((found, [size], i) => (abs >= size ? i : found), 0);
  let scaled = Math.round((abs / units[index][0]) * 10) / 10;
  // 999.96K rounds to 1000K: show it as 1M instead
  if (scaled >= 1000 && index < units.length - 1) {
    index += 1;
    scaled = Math.round((abs / units[index][0]) * 10) / 10;
  }
  return `${value < 0 ? "-" : ""}$${scaled}${units[index][1]}`;
}

/** A plain number with thousands separators and at most `digits` decimals. */
export function formatNumber(value: number | null | undefined, digits = 0): string {
  if (!isNumber(value)) return NOT_AVAILABLE;
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: digits }).format(value);
}

/** 20.4 kg: up to two decimals, no trailing zeros. */
export function formatKg(value: number | null | undefined): string {
  if (!isNumber(value)) return NOT_AVAILABLE;
  return `${formatNumber(value, 2)} kg`;
}

/** Formats a value that is already a percentage (75 -> "75%"). */
export function formatPercent(value: number | null | undefined, digits = 0): string {
  if (!isNumber(value)) return NOT_AVAILABLE;
  return `${new Intl.NumberFormat("en-US", { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(value)}%`;
}

/** A 0-100 score with one decimal (61.5). */
export function formatScore(value: number | null | undefined): string {
  if (!isNumber(value)) return NOT_AVAILABLE;
  return new Intl.NumberFormat("en-US", { minimumFractionDigits: 1, maximumFractionDigits: 1 }).format(value);
}

function toDate(value: string | number | Date | null | undefined): Date | null {
  if (value === null || value === undefined || value === "") return null;
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

/** "Nov 4, 2026, 3:05 PM". */
export function formatDateTime(value: string | number | Date | null | undefined, options: { timeZone?: string } = {}): string {
  const date = toDate(value);
  if (!date) return NOT_AVAILABLE;
  return new Intl.DateTimeFormat("en-US", { dateStyle: "medium", timeStyle: "short", timeZone: options.timeZone }).format(date);
}

/** "Nov 4, 2026". */
export function formatDate(value: string | number | Date | null | undefined, options: { timeZone?: string } = {}): string {
  const date = toDate(value);
  if (!date) return NOT_AVAILABLE;
  return new Intl.DateTimeFormat("en-US", { dateStyle: "medium", timeZone: options.timeZone }).format(date);
}

/** "just now", "5 min ago", "3 h ago", "2 d ago"; older than a week falls back to the date. */
export function formatRelative(
  value: string | number | Date | null | undefined,
  now: number | Date = Date.now(),
  options: { timeZone?: string } = {},
): string {
  const date = toDate(value);
  const nowDate = toDate(now);
  if (!date || !nowDate) return NOT_AVAILABLE;

  const seconds = Math.round((nowDate.getTime() - date.getTime()) / 1000);
  if (seconds < -60) return formatDateTime(date, options); // clearly in the future
  if (seconds < 45) return "just now";
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} h ago`;
  const days = Math.round(hours / 24);
  if (days < 7) return `${days} d ago`;
  return formatDate(date, options);
}

export function pluralize(count: number, singular: string, plural = `${singular}s`): string {
  return `${formatNumber(count)} ${count === 1 ? singular : plural}`;
}
