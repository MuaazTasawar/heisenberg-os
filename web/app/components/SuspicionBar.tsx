import { clamp, formatNumber } from "../lib/format";
import { riskColor, riskFromScore, riskLabel } from "../lib/risk";

/** How close a front business is to a DEA raid (100 means raided), as a bar and in words. */
export default function SuspicionBar({ value }: { value: number }) {
  const shown = clamp(Number.isFinite(value) ? value : 0, 0, 100);
  const level = riskFromScore(shown);
  return (
    <div className="suspicion">
      <div className="meter" role="meter" aria-label="Suspicion" aria-valuemin={0} aria-valuemax={100} aria-valuenow={shown}>
        <div className="meter__fill" style={{ width: `${shown}%`, background: riskColor(level) }} />
      </div>
      <span className="suspicion__label">
        {formatNumber(shown, 0)} / 100 &middot; {riskLabel(level)}
      </span>
    </div>
  );
}
