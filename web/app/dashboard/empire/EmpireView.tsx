"use client";

import { useState } from "react";
import Link from "next/link";
import { keepPreviousData, useMutation, useQuery } from "@tanstack/react-query";
import { FlaskConical, Store } from "lucide-react";
import DashboardShell from "../../components/DashboardShell";
import ErrorNotice from "../../components/ErrorNotice";
import GameStateGate from "../../components/GameStateGate";
import Panel from "../../components/Panel";
import RiskBadge from "../../components/RiskBadge";
import StatTile, { type StatTone } from "../../components/StatTile";
import { api } from "../../lib/api";
import { formatKg, formatMoney, formatNumber, formatPercent } from "../../lib/format";
import { useDebouncedValue } from "../../lib/hooks";
import { KEYS, useRefreshGame } from "../../lib/queries";
import { riskFromScore } from "../../lib/risk";
import { territoryStatus } from "../../lib/rules";
import type { Empire, GameState, Rules } from "../../lib/types";
import { clampEffort } from "../../lib/validation";

const HEAT_TONE: Record<string, StatTone> = { low: "good", medium: "gold", high: "danger", critical: "danger" };

export default function EmpireView() {
  return (
    <DashboardShell title="Empire">
      <GameStateGate>{(data) => <EmpireControls data={data} />}</GameStateGate>
    </DashboardShell>
  );
}

function EmpireControls({ data }: { data: GameState }) {
  const { empire, rules } = data;
  return (
    <>
      <p className="mono muted">Active empire: {empire.name}</p>
      <div className="stat-grid">
        <StatTile label="Dirty money" value={formatMoney(empire.dirty_money)} tone="gold" />
        <StatTile label="Clean money" value={formatMoney(empire.clean_money)} tone="good" />
        <StatTile label="Heat" value={formatNumber(empire.heat_level, 1)} tone={HEAT_TONE[riskFromScore(empire.heat_level)]} hint={`cools by ${formatNumber(rules.heat_decay_per_hour, 1)} per hour`} />
        <StatTile label="Purity" value={formatPercent(empire.purity_rating, 1)} />
        <StatTile label="Last batch" value={formatKg(empire.batch_yield)} />
        <StatTile label="Territories" value={`${formatNumber(empire.territory_count)} / ${formatNumber(rules.max_territories)}`} />
      </div>
      <CookPanel empire={empire} rules={rules} />
      <TerritoryPanel empire={empire} rules={rules} />
      <p>
        <Link href="/dashboard/fronts" className="btn btn--ghost">
          <Store aria-hidden="true" size={16} /> Manage front businesses
        </Link>
      </p>
    </>
  );
}

function CookPanel({ empire, rules }: { empire: Empire; rules: Rules }) {
  const [effort, setEffort] = useState(() => clampEffort(rules.optimal_effort, rules));
  // The advice follows the slider once it has stopped moving, not on every pixel.
  const settled = useDebouncedValue(effort, 300);
  const refresh = useRefreshGame();

  const advice = useQuery({
    queryKey: KEYS.advice(settled),
    queryFn: () => api.empire.advice(settled),
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  });
  const cook = useMutation({
    mutationFn: (amount: number) => api.empire.cook(amount),
    onSuccess: () => {
      void refresh();
    },
  });

  const heatAfter = Math.min(100, empire.heat_level + effort * rules.heat_per_effort);
  const result = cook.data;

  return (
    <Panel title="Cook a batch" subtitle="More effort means more product, but more heat">
      <div className="field">
        <label htmlFor="cook-effort">Cooking effort</label>
        <div className="slider">
          <input
            id="cook-effort"
            type="range"
            min={rules.min_effort}
            max={rules.max_effort}
            step={0.1}
            value={effort}
            onChange={(event) => setEffort(clampEffort(Number(event.target.value), rules))}
            disabled={cook.isPending}
          />
          <output htmlFor="cook-effort" className="mono slider__value">
            {formatNumber(effort, 1)}
          </output>
        </div>
        <span className="field__hint">
          From {formatNumber(rules.min_effort, 1)} to {formatNumber(rules.max_effort, 1)}.{" "}
          <button type="button" className="link-button" onClick={() => setEffort(clampEffort(rules.optimal_effort, rules))} disabled={cook.isPending}>
            Use the sweet spot ({formatNumber(rules.optimal_effort, 2)})
          </button>
        </span>
      </div>

      <p className="projection__heat">
        Heat {formatNumber(empire.heat_level, 1)} &rarr; <strong>{formatNumber(heatAfter, 1)}</strong> <RiskBadge level={riskFromScore(heatAfter)} />
        {empire.heat_level + effort * rules.heat_per_effort > 100 && <span className="muted"> (heat stops at 100)</span>}
      </p>

      <div className="projection" aria-busy={advice.isFetching}>
        {advice.data ? (
          <>
            <p>
              Expected yield <strong>{formatKg(advice.data.predicted_yield)}</strong>, worth about{" "}
              <strong>{formatMoney(advice.data.predicted_yield * rules.dirty_per_kg)}</strong> in dirty money.
            </p>
            <p className="muted">
              That is {formatPercent(advice.data.efficiency_score, 0)} of the best batch ({formatKg(advice.data.optimal_yield)} at effort{" "}
              {formatNumber(advice.data.optimal_effort, 2)}). A best-effort batch would bring purity to {formatPercent(advice.data.purity_projection, 1)}.
            </p>
            <p className="muted projection__source">{advice.data.source === "ml" ? "Machine-learning advice" : "Built-in estimate"}</p>
          </>
        ) : advice.isError ? (
          <p className="muted">Advice is unavailable right now, but you can still cook.</p>
        ) : (
          <p className="muted">Working out what this batch would bring&hellip;</p>
        )}
      </div>

      <ErrorNotice error={cook.error} title="The batch failed" />
      {result && (
        <p className="result" role="status">
          Batch cooked: <strong>+{formatKg(result.yield_gained)}</strong>, <strong>+{formatMoney(result.dirty_gained)}</strong> dirty money,{" "}
          +{formatNumber(result.heat_added, 1)} heat, purity {result.purity_delta >= 0 ? "+" : ""}
          {formatNumber(result.purity_delta, 2)}.
        </p>
      )}
      <button type="button" className="btn" onClick={() => cook.mutate(effort)} disabled={cook.isPending}>
        <FlaskConical aria-hidden="true" size={16} /> {cook.isPending ? "Cooking\u2026" : "Cook batch"}
      </button>
    </Panel>
  );
}

function TerritoryPanel({ empire, rules }: { empire: Empire; rules: Rules }) {
  const refresh = useRefreshGame();
  const status = territoryStatus(rules, empire);
  const claim = useMutation({
    mutationFn: () => api.empire.claimTerritory(),
    onSuccess: () => {
      void refresh();
    },
  });
  const claimed = claim.data;

  return (
    <Panel title="Territory" subtitle={`Each territory adds ${formatPercent(rules.territory_bonus * 100)} to your yield`}>
      {status.atMax ? (
        <p>You hold the maximum of {formatNumber(rules.max_territories)} territories.</p>
      ) : (
        <>
          <p>
            The next territory costs <strong>{formatMoney(status.cost)}</strong> in clean money. You hold {formatNumber(empire.territory_count)} of{" "}
            {formatNumber(rules.max_territories)}.
          </p>
          {!status.affordable && <p className="muted">You need {formatMoney(status.shortfall)} more clean money.</p>}
        </>
      )}
      <ErrorNotice error={claim.error} title="Could not claim the territory" />
      {claimed && (
        <p className="result" role="status">
          Territory claimed. You now hold {formatNumber(claimed.territory_count)}; your yield bonus is +
          {formatPercent(claimed.territory_count * rules.territory_bonus * 100)}.
        </p>
      )}
      {!status.atMax && (
        <button type="button" className="btn" onClick={() => claim.mutate()} disabled={!status.affordable || claim.isPending}>
          {claim.isPending ? "Claiming\u2026" : `Claim for ${formatMoney(status.cost)}`}
        </button>
      )}
    </Panel>
  );
}
