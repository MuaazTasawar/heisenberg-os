"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import DashboardShell from "../../components/DashboardShell";
import ErrorNotice from "../../components/ErrorNotice";
import GameStateGate from "../../components/GameStateGate";
import Panel from "../../components/Panel";
import RiskBadge from "../../components/RiskBadge";
import StatTile, { type StatTone } from "../../components/StatTile";
import { api } from "../../lib/api";
import { buildThreatSeries, threatTrend } from "../../lib/dea";
import { clamp, formatDateTime, formatNumber, formatScore, pluralize } from "../../lib/format";
import { KEYS, useRefreshGame } from "../../lib/queries";
import { parseRisk, riskColor } from "../../lib/risk";
import type { DeaThreat, GameState } from "../../lib/types";
import ThreatChart from "./ThreatChart";

const HISTORY_LIMIT = 60;
const LEVEL_TONE: Record<string, StatTone> = { low: "good", medium: "gold", high: "danger", critical: "danger" };

export default function DeaView() {
  return (
    <DashboardShell title="DEA file">
      <GameStateGate>{(data) => <DeaControls data={data} />}</GameStateGate>
    </DashboardShell>
  );
}

function DeaControls({ data }: { data: GameState }) {
  return (
    <>
      <ThreatPanel threat={data.dea_threat} />
      <HistoryPanel />
      <EvidencePanel />
    </>
  );
}

function ThreatPanel({ threat }: { threat: DeaThreat }) {
  const refresh = useRefreshGame();
  const [checking, setChecking] = useState(false);
  const level = parseRisk(threat.risk_level);
  const score = clamp(threat.threat_score, 0, 100);

  async function checkAgain() {
    setChecking(true);
    try {
      await refresh();
    } finally {
      setChecking(false);
    }
  }

  return (
    <Panel
      title="How close is Hank?"
      subtitle={`Lead agent: ${threat.lead_agent}`}
      actions={
        <button type="button" className="btn btn--ghost" onClick={() => void checkAgain()} disabled={checking}>
          <RefreshCw aria-hidden="true" size={16} /> {checking ? "Checking\u2026" : "Check again"}
        </button>
      }
    >
      <div className="stat-grid">
        <StatTile label="Threat score" value={`${formatScore(threat.threat_score)} / 100`} tone={LEVEL_TONE[level]} hint={<RiskBadge level={level} />} />
        <StatTile label="Evidence" value={formatNumber(threat.evidence_count)} hint={pluralize(threat.evidence_count, "piece") + " on file"} />
        <StatTile label="Assessed" value={formatDateTime(threat.updated_at)} hint={threat.source === "ml" ? "machine-learning assessment" : "built-in estimate"} />
      </div>
      <div className="meter" role="meter" aria-label="DEA threat score" aria-valuemin={0} aria-valuemax={100} aria-valuenow={score}>
        <div className="meter__fill" style={{ width: `${score}%`, background: riskColor(level) }} />
      </div>
      <p className="threat__text">{threat.prediction}</p>
      <p className="muted">{threat.recommendation}</p>
    </Panel>
  );
}

function HistoryPanel() {
  const history = useQuery({
    queryKey: KEYS.history,
    queryFn: () => api.dea.history(HISTORY_LIMIT),
    staleTime: 15_000,
  });
  const series = useMemo(() => buildThreatSeries(history.data ?? []), [history.data]);
  const trend = threatTrend(series);
  const latest = series[series.length - 1];

  return (
    <Panel title="The paper trail" subtitle="The DEA writes a new entry whenever its assessment changes">
      <ErrorNotice error={history.error} title="Cannot load the DEA's history" onRetry={() => void history.refetch()} />
      {history.isPending && (
        <p className="muted" role="status">
          Loading the paper trail&hellip;
        </p>
      )}
      {history.data && series.length === 0 && <p className="muted">No history yet. The DEA has not built a file on you.</p>}
      {series.length > 0 && (
        <>
          {trend.direction !== "none" && (
            <p className="trend" data-direction={trend.direction}>
              {trend.direction === "up" && `Up ${formatNumber(trend.change, 1)} points since the previous assessment.`}
              {trend.direction === "down" && `Down ${formatNumber(Math.abs(trend.change), 1)} points since the previous assessment.`}
              {trend.direction === "flat" && "Unchanged since the previous assessment."}
            </p>
          )}
          <div role="img" aria-label={`Chart of the DEA threat score over time. ${series.length} ${series.length === 1 ? "entry" : "entries"}, latest ${formatScore(latest.score)} out of 100.`}>
            <ThreatChart points={series} />
          </div>
          <details className="history-table">
            <summary>Show the same data as a table</summary>
            <div className="table-wrap">
              <table className="table">
                <caption className="visually-hidden">DEA threat history, newest first</caption>
                <thead>
                  <tr>
                    <th scope="col">When</th>
                    <th scope="col" className="num">
                      Score
                    </th>
                    <th scope="col">Risk</th>
                    <th scope="col" className="num">
                      Evidence
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {[...series].reverse().map((point) => (
                    <tr key={point.time}>
                      <td>{formatDateTime(point.time)}</td>
                      <td className="num">{formatScore(point.score)}</td>
                      <td>
                        <RiskBadge level={point.level} />
                      </td>
                      <td className="num">{formatNumber(point.evidence)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </details>
        </>
      )}
    </Panel>
  );
}

function EvidencePanel() {
  const evidence = useQuery({
    queryKey: KEYS.evidence,
    queryFn: () => api.dea.evidence(),
    staleTime: 15_000,
  });
  const file = evidence.data;

  return (
    <Panel title="What the DEA has on you" subtitle="Every flagged transfer and raided front adds to the file">
      <ErrorNotice error={evidence.error} title="Cannot load the evidence file" onRetry={() => void evidence.refetch()} />
      {evidence.isPending && (
        <p className="muted" role="status">
          Opening the file&hellip;
        </p>
      )}
      {file && file.count === 0 && <p className="muted">The DEA has nothing on you. Yet.</p>}
      {file && file.count > 0 && (
        <>
          <p>
            <strong>{pluralize(file.count, "piece")}</strong> of evidence, total weight <strong>{formatNumber(file.total_weight, 1)}</strong>.{" "}
            <span className="muted">{file.source === "ml" ? "Machine-learning assessment." : "Built-in estimate."}</span>
          </p>
          {file.top_sources.length > 0 && (
            <div className="table-wrap">
              <table className="table">
                <caption className="visually-hidden">The heaviest pieces of evidence</caption>
                <thead>
                  <tr>
                    <th scope="col">Source</th>
                    <th scope="col">Kind</th>
                    <th scope="col" className="num">
                      Weight
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {file.top_sources.map((item, index) => (
                    <tr key={`${item.source}-${item.kind}-${index}`}>
                      <td>{item.source}</td>
                      <td>{item.kind.replace(/_/g, " ")}</td>
                      <td className="num">{formatNumber(item.weight, 1)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </Panel>
  );
}
