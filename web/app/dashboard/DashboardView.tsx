"use client";

import Link from "next/link";
import { Banknote, FlaskConical, Scale, ShieldAlert } from "lucide-react";
import DashboardShell from "../components/DashboardShell";
import GameStateGate from "../components/GameStateGate";
import Panel from "../components/Panel";
import RiskBadge from "../components/RiskBadge";
import StatTile, { type StatTone } from "../components/StatTile";
import { clamp, formatKg, formatMoney, formatNumber, formatPercent, formatRelative, formatScore } from "../lib/format";
import { activeFronts, describeRules, territoryStatus } from "../lib/rules";
import { parseRisk, riskColor, riskFromScore } from "../lib/risk";
import type { GameState } from "../lib/types";

const HEAT_TONE: Record<string, StatTone> = { low: "good", medium: "gold", high: "danger", critical: "danger" };

export default function DashboardView() {
  return (
    <DashboardShell title="Command center">
      <GameStateGate>{(data) => <Overview data={data} />}</GameStateGate>
    </DashboardShell>
  );
}

function Overview({ data }: { data: GameState }) {
  const { empire, dea_threat: threat, legal_cases: cases, rules } = data;
  const heatLevel = riskFromScore(empire.heat_level);
  const expansion = territoryStatus(rules, empire);
  const threatLevel = parseRisk(threat.risk_level);
  const score = clamp(threat.threat_score, 0, 100);

  return (
    <>
      <p className="mono muted">Active empire: {empire.name}</p>

      <div className="stat-grid">
        <StatTile label="Dirty money" value={formatMoney(empire.dirty_money)} tone="gold" hint="waiting to be laundered" />
        <StatTile label="Clean money" value={formatMoney(empire.clean_money)} tone="good" hint="safe to spend" />
        <StatTile label="Heat" value={formatNumber(empire.heat_level, 1)} tone={HEAT_TONE[heatLevel]} hint={`cools by ${formatNumber(rules.heat_decay_per_hour, 1)} per hour`} />
        <StatTile label="Purity" value={formatPercent(empire.purity_rating, 1)} />
        <StatTile label="Last batch" value={formatKg(empire.batch_yield)} />
        <StatTile label="Territories" value={`${formatNumber(empire.territory_count)} / ${formatNumber(rules.max_territories)}`} />
        <StatTile label="Front businesses" value={`${formatNumber(activeFronts(empire))} / ${formatNumber(rules.max_active_fronts)}`} hint="open of the most you can run" />
      </div>

      <div className="grid-2">
        <Panel title="DEA threat" subtitle={`Lead agent: ${threat.lead_agent}`} actions={<RiskBadge level={threatLevel} />}>
          <p className="threat__score">
            <span className="mono">{formatScore(threat.threat_score)}</span>
            <span className="muted"> / 100</span>
          </p>
          <div className="meter" role="meter" aria-label="DEA threat score" aria-valuemin={0} aria-valuemax={100} aria-valuenow={score}>
            <div className="meter__fill" style={{ width: `${score}%`, background: riskColor(threatLevel) }} />
          </div>
          <p className="threat__text">{threat.prediction}</p>
          <p className="muted">{threat.recommendation}</p>
          <p className="muted threat__meta">
            {formatNumber(threat.evidence_count)} piece{threat.evidence_count === 1 ? "" : "s"} of evidence &middot;{" "}
            {threat.source === "ml" ? "machine-learning assessment" : "built-in estimate"}
          </p>
        </Panel>

        <Panel title="Expansion" subtitle="More territory means a bigger yield">
          {expansion.atMax ? (
            <p>You hold the maximum of {formatNumber(rules.max_territories)} territories.</p>
          ) : expansion.affordable ? (
            <p>
              The next territory costs <strong>{formatMoney(expansion.cost)}</strong>. You can afford it.
            </p>
          ) : (
            <p>
              The next territory costs <strong>{formatMoney(expansion.cost)}</strong>. You need {formatMoney(expansion.shortfall)} more clean money.
            </p>
          )}
          <p className="quick-links">
            <Link href="/dashboard/empire" className="btn btn--ghost">
              <FlaskConical aria-hidden="true" size={16} /> Cook a batch
            </Link>
            <Link href="/dashboard/launder" className="btn btn--ghost">
              <Banknote aria-hidden="true" size={16} /> Launder money
            </Link>
          </p>
        </Panel>
      </div>

      <Panel
        title="Legal trouble"
        subtitle="Your latest cases"
        actions={
          <Link href="/dashboard/legal" className="btn btn--ghost">
            <Scale aria-hidden="true" size={16} /> Better call Saul
          </Link>
        }
      >
        {cases.length === 0 ? (
          <p className="muted">No cases. Keep it that way.</p>
        ) : (
          <ul className="case-list">
            {cases.map((legalCase) => (
              <li key={legalCase.id} className="case-list__item">
                <span className="case-list__charge">{legalCase.charge}</span>
                <RiskBadge level={legalCase.risk} />
                <span className="muted">{legalCase.status}</span>
                <span className="muted">{formatRelative(legalCase.created_at)}</span>
              </li>
            ))}
          </ul>
        )}
      </Panel>

      <Panel title="How it works" subtitle="Straight from the server, so these numbers are always current">
        <ul className="rules-list">
          {describeRules(rules).map((line) => (
            <li key={line.id}>{line.text}</li>
          ))}
        </ul>
        <p>
          <Link href="/dashboard/dea" className="btn btn--ghost">
            <ShieldAlert aria-hidden="true" size={16} /> Open the DEA file
          </Link>
        </p>
      </Panel>
    </>
  );
}
