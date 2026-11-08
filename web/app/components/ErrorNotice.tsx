import { RefreshCw, TriangleAlert } from "lucide-react";
import { errorMessage } from "../lib/api";

interface ErrorNoticeProps {
  /** Whatever was thrown; ApiError messages are shown, anything else gets a generic text. */
  error?: unknown;
  /** Shown instead of the text derived from `error`. */
  message?: string;
  title?: string;
  onRetry?: () => void;
}

/** A visible, announced error with an optional "Try again". Renders nothing when there is no error. */
export default function ErrorNotice({ error, message, title = "Something went wrong", onRetry }: ErrorNoticeProps) {
  const text = message ?? (error ? errorMessage(error) : "");
  if (!text) return null;

  return (
    <div className="notice" role="alert">
      <TriangleAlert aria-hidden="true" size={20} className="notice__icon" />
      <div className="notice__body">
        <strong className="notice__title">{title}</strong>
        <p>{text}</p>
      </div>
      {onRetry && (
        <button type="button" className="btn btn--ghost" onClick={onRetry}>
          <RefreshCw aria-hidden="true" size={16} /> Try again
        </button>
      )}
    </div>
  );
}
