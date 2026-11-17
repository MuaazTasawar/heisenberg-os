"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { keepPreviousData, useMutation, useQuery } from "@tanstack/react-query";
import { Banknote } from "lucide-react";
import DashboardShell from "../../components/DashboardShell";
import ErrorNotice from "../../components/ErrorNotice";
import GameStateGate from "../../components/GameStateGate";
import Pagination from "../../components/Pagination";
import Panel from "../../components/Panel";
import RiskBadge from "../../components/RiskBadge";
import StatTile from "../../components/StatTile";
import { api } from "../../lib/api";
import { formatMoney, formatNumber, formatRelative } from "../../lib/format";
import { KEYS, useRefreshGame } from "../../lib/queries";
import { riskFromScore } from "../../lib/risk";
import type { Empire, FrontBusiness, GameState, LaunderResult, Rules, Transaction } from "../../lib/types";
import { parseAmount, validateLaunder } from "../../lib/validation";

const LEDGER_PAGE_SIZE = 10;

export default function LaunderView() {
  return (
    <DashboardShell title="Launder money">
      <GameStateGate>{(data) => <LaunderControls data={data} />}</GameStateGate>
    </DashboardShell>
  );
}

function LaunderControls({ data }: { data: GameState }) {
  const { empire, rules } = data;
  const openFronts = (empire.front_businesses ?? []).filter((front) => front.is_active);
  return (
    <>
      <p className="mono muted">Active empire: {empire.name}</p>
      <div className="stat-grid">
        <StatTile label="Dirty money" value={formatMoney(empire.dirty_money)} tone="gold" hint="waiting to be laundered" />
        <StatTile label="Clean money" value={formatMoney(empire.clean_money)} tone="good" />
        <StatTile label="Open fronts" value={`${formatNumber(openFronts.length)} / ${formatNumber(rules.max_active_fronts)}`} />
      </div>
      <TransferPanel empire={empire} rules={rules} openFronts={openFronts} />
      <Ledger />
    </>
  );
}

/** The most the player can send, in whole cents (the server does not take fractions of a cent). */
function maxAmount(dirty: number): number {
  return Math.floor(dirty * 100) / 100;
}

function TransferPanel({ empire, rules, openFronts }: { empire: Empire; rules: Rules; openFronts: FrontBusiness[] }) {
  const refresh = useRefreshGame();
  const [amount, setAmount] = useState("");
  const [frontId, setFrontId] = useState("");
  const [submitted, setSubmitted] = useState(false);

  const send = useMutation({
    mutationFn: (input: { amount: number; business_id: number }) => api.launder.send(input),
    onSuccess: (result) => {
      setAmount("");
      setSubmitted(false);
      // A raided front is no longer one to choose.
      if (result.front_closed) setFrontId("");
      void refresh();
    },
  });

  const limits = { min: rules.launder_min, dirty: empire.dirty_money };
  const openIds = openFronts.map((front) => front.id);
  const errors = submitted ? validateLaunder({ amount, frontId }, limits, openIds) : {};
  const parsed = parseAmount(amount);
  const canMax = maxAmount(empire.dirty_money) >= rules.launder_min;

  const clean = parsed === null ? null : Math.round(parsed * rules.launder_rate * 100) / 100;
  const fee = parsed === null || clean === null ? null : Math.round((parsed - clean) * 100) / 100;

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    setSubmitted(true);
    if (Object.keys(validateLaunder({ amount, frontId }, limits, openIds)).length > 0 || parsed === null) return;
    send.mutate({ amount: parsed, business_id: Number(frontId) });
  }

  if (openFronts.length === 0 && !send.data) {
    return (
      <Panel title="Send money through a front" subtitle="Dirty money in, clean money out">
        <p>
          You need an open front business before you can launder money.{" "}
          <Link href="/dashboard/fronts">Open one</Link>.
        </p>
      </Panel>
    );
  }

  return (
    <Panel title="Send money through a front" subtitle={`You keep ${formatNumber(rules.launder_rate * 100, 0)}% of what you send`}>
      <form onSubmit={onSubmit} noValidate>
        <div className="field">
          <label htmlFor="launder-front">Front business</label>
          <select
            id="launder-front"
            value={frontId}
            onChange={(event) => setFrontId(event.target.value)}
            disabled={send.isPending}
            aria-invalid={errors.front ? true : undefined}
            aria-describedby={errors.front ? "launder-front-error" : undefined}
          >
            <option value="">Choose a front</option>
            {openFronts.map((front) => (
              <option key={front.id} value={front.id}>
                {front.name} (suspicion {formatNumber(front.suspicion, 0)})
              </option>
            ))}
          </select>
          {errors.front && (
            <span id="launder-front-error" className="field__error">
              {errors.front}
            </span>
          )}
        </div>

        <div className="field">
          <label htmlFor="launder-amount">Amount</label>
          <div className="amount-row">
            <input
              id="launder-amount"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
              inputMode="decimal"
              autoComplete="off"
              placeholder="5000"
              disabled={send.isPending}
              aria-invalid={errors.amount ? true : undefined}
              aria-describedby={errors.amount ? "launder-amount-error" : "launder-amount-hint"}
            />
            <button type="button" className="btn btn--ghost" onClick={() => setAmount(String(maxAmount(empire.dirty_money)))} disabled={send.isPending || !canMax}>
              Use all
            </button>
          </div>
          <span id="launder-amount-hint" className="field__hint">
            From {formatMoney(rules.launder_min)} up to {formatMoney(empire.dirty_money)}, your dirty money.
          </span>
          {errors.amount && (
            <span id="launder-amount-error" className="field__error">
              {errors.amount}
            </span>
          )}
        </div>

        {parsed !== null && clean !== null && fee !== null && parsed > 0 && (
          <p className="preview" aria-live="polite">
            Sending <strong>{formatMoney(parsed, { cents: true })}</strong> gives you <strong>{formatMoney(clean, { cents: true })}</strong> clean; the fee is{" "}
            {formatMoney(fee, { cents: true })}.
          </p>
        )}
        {parsed !== null && parsed > rules.large_transfer && (
          <p className="preview preview--warning" role="note">
            A single transfer above {formatMoney(rules.large_transfer)} is flagged as evidence. Splitting it into smaller transfers draws less attention.
          </p>
        )}

        <ErrorNotice error={send.error} title="The transfer failed" />
        <button type="submit" className="btn" disabled={send.isPending}>
          <Banknote aria-hidden="true" size={16} /> {send.isPending ? "Sending\u2026" : "Launder"}
        </button>
      </form>

      {send.data && <TransferResult result={send.data} />}
    </Panel>
  );
}

function TransferResult({ result }: { result: LaunderResult }) {
  const { transaction, analysis, front } = result;
  return (
    <div className="transfer-result">
      {result.front_closed && (
        <ErrorNotice title={`${front.name} was raided`} message={front.shutdown_reason || "The DEA shut the front down."} />
      )}
      <p className="result" role="status">
        Sent {formatMoney(transaction.amount, { cents: true })} through {front.name}: <strong>{formatMoney(result.cleaned_amount, { cents: true })}</strong> clean, fee{" "}
        {formatMoney(result.fee, { cents: true })}. Suspicion +{formatNumber(result.suspicion_gain, 1)}
        {result.front_closed ? "." : ` (now ${formatNumber(front.suspicion, 0)} / 100).`}
      </p>
      {transaction.flagged && (
        <p className="preview preview--warning" role="note">
          <span className="badge" data-risk="critical">
            Flagged
          </span>{" "}
          This transfer became evidence{transaction.flag_reason ? `: ${transaction.flag_reason}` : "."}
        </p>
      )}
      <div className="analysis">
        <h3 className="analysis__title">Analysis</h3>
        <p>
          Risk {formatNumber(analysis.risk_score, 0)} / 100 <RiskBadge level={riskFromScore(analysis.risk_score)} />
          {analysis.anomaly && <span className="muted"> &middot; unusual pattern</span>}
        </p>
        {analysis.patterns.length > 0 && (
          <ul className="analysis__patterns">
            {analysis.patterns.map((pattern) => (
              <li key={pattern}>{pattern}</li>
            ))}
          </ul>
        )}
        <p className="muted">{analysis.recommendation}</p>
        <p className="muted projection__source">{analysis.source === "ml" ? "Machine-learning analysis" : "Built-in analysis"}</p>
      </div>
    </div>
  );
}

const TYPE_LABEL: Record<string, string> = { dirty: "Dirty", clean: "Clean", laundered: "Laundered" };

function Ledger() {
  const [offset, setOffset] = useState(0);
  const [flaggedOnly, setFlaggedOnly] = useState(false);
  const query = { limit: LEDGER_PAGE_SIZE, offset, ...(flaggedOnly ? { flagged: true } : {}) };

  const ledger = useQuery({
    queryKey: KEYS.ledger(query),
    queryFn: () => api.launder.transactions(query),
    placeholderData: keepPreviousData,
  });
  const page = ledger.data;

  return (
    <Panel
      title="Ledger"
      subtitle="Every transfer, newest first"
      actions={
        <label className="checkbox">
          <input
            type="checkbox"
            checked={flaggedOnly}
            onChange={(event) => {
              setFlaggedOnly(event.target.checked);
              setOffset(0);
            }}
          />
          Flagged only
        </label>
      }
    >
      <ErrorNotice error={ledger.error} title="Cannot load the ledger" onRetry={() => void ledger.refetch()} />
      {ledger.isPending && (
        <p className="muted" role="status">
          Loading the ledger&hellip;
        </p>
      )}
      {page && page.items.length === 0 && <p className="muted">{flaggedOnly ? "No flagged transfers. Keep it that way." : "No transactions yet."}</p>}
      {page && page.items.length > 0 && (
        <div className="table-wrap" aria-busy={ledger.isPlaceholderData}>
          <table className="table">
            <caption className="visually-hidden">Your transactions</caption>
            <thead>
              <tr>
                <th scope="col">When</th>
                <th scope="col">Type</th>
                <th scope="col">Description</th>
                <th scope="col" className="num">
                  Amount
                </th>
                <th scope="col" className="num">
                  Risk
                </th>
              </tr>
            </thead>
            <tbody>
              {page.items.map((transaction) => (
                <LedgerRow key={transaction.id} transaction={transaction} />
              ))}
            </tbody>
          </table>
        </div>
      )}
      {page && <Pagination total={page.total} limit={page.limit || LEDGER_PAGE_SIZE} offset={page.offset} onChange={setOffset} label="Transactions" />}
    </Panel>
  );
}

function LedgerRow({ transaction }: { transaction: Transaction }) {
  return (
    <tr>
      <td>{formatRelative(transaction.created_at)}</td>
      <td>{TYPE_LABEL[transaction.type] ?? transaction.type}</td>
      <td>
        {transaction.description}
        {transaction.flagged && (
          <>
            {" "}
            <span className="badge" data-risk="critical">
              Flagged
            </span>
            {transaction.flag_reason && <span className="muted ledger__reason"> {transaction.flag_reason}</span>}
          </>
        )}
      </td>
      <td className="num">{formatMoney(transaction.amount, { cents: true })}</td>
      <td className="num">{formatNumber(transaction.risk_score, 0)}</td>
    </tr>
  );
}
