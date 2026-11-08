import { parseRisk, riskLabel } from "../lib/risk";

/** The risk level as a word (never colour alone). Unknown values are shown as medium. */
export default function RiskBadge({ level }: { level: string }) {
  const risk = parseRisk(level);
  return (
    <span className="badge" data-risk={risk}>
      {riskLabel(risk)}
    </span>
  );
}
