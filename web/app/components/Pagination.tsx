import { formatNumber } from "../lib/format";

interface PaginationProps {
  total: number;
  limit: number;
  offset: number;
  onChange: (offset: number) => void;
  /** What is being paged, for the screen reader ("Transactions"). */
  label: string;
}

/** Previous / next with the range that is showing. */
export default function Pagination({ total, limit, offset, onChange, label }: PaginationProps) {
  if (total <= 0) return null;
  const first = offset + 1;
  const last = Math.min(offset + limit, total);
  const hasPrevious = offset > 0;
  const hasNext = offset + limit < total;

  return (
    <nav className="pagination" aria-label={`${label} pages`}>
      <button type="button" className="btn btn--ghost" onClick={() => onChange(Math.max(0, offset - limit))} disabled={!hasPrevious}>
        Previous
      </button>
      <span className="muted" aria-live="polite">
        Showing {formatNumber(first)}&ndash;{formatNumber(last)} of {formatNumber(total)}
      </span>
      <button type="button" className="btn btn--ghost" onClick={() => onChange(offset + limit)} disabled={!hasNext}>
        Next
      </button>
    </nav>
  );
}
