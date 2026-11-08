import type { ReactNode } from "react";

export type StatTone = "default" | "gold" | "good" | "danger";

interface StatTileProps {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  tone?: StatTone;
  icon?: ReactNode;
}

/** One number with a label, for the rows of figures on the dashboard. */
export default function StatTile({ label, value, hint, tone = "default", icon }: StatTileProps) {
  return (
    <div className="stat" data-tone={tone}>
      <span className="stat__label">
        {icon}
        {label}
      </span>
      <strong className="stat__value">{value}</strong>
      {hint && <span className="stat__hint">{hint}</span>}
    </div>
  );
}
