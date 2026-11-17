"use client";

import { useState, type FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Store, Trash2 } from "lucide-react";
import DashboardShell from "../../components/DashboardShell";
import ErrorNotice from "../../components/ErrorNotice";
import GameStateGate from "../../components/GameStateGate";
import Panel from "../../components/Panel";
import StatTile from "../../components/StatTile";
import SuspicionBar from "../../components/SuspicionBar";
import { api } from "../../lib/api";
import { formatMoney, formatNumber } from "../../lib/format";
import { KEYS, useRefreshGame } from "../../lib/queries";
import { activeFronts } from "../../lib/rules";
import type { Empire, FrontBusiness, FrontType, GameState, Rules } from "../../lib/types";
import { FRONT_LIMITS, normalizeName, validateFront } from "../../lib/validation";

export default function FrontsView() {
  return (
    <DashboardShell title="Front businesses">
      <GameStateGate>{(data) => <FrontsControls data={data} />}</GameStateGate>
    </DashboardShell>
  );
}

function FrontsControls({ data }: { data: GameState }) {
  const { empire, rules } = data;
  const types = useQuery({
    queryKey: KEYS.frontTypes,
    queryFn: () => api.empire.frontTypes(),
    staleTime: 5 * 60_000,
  });
  const open = activeFronts(empire);
  const fronts = empire.front_businesses ?? [];
  const typeLabel = (type: string) => types.data?.find((item) => item.type === type)?.label ?? type;

  return (
    <>
      <p className="mono muted">Active empire: {empire.name}</p>
      <div className="stat-grid">
        <StatTile label="Open fronts" value={`${formatNumber(open)} / ${formatNumber(rules.max_active_fronts)}`} hint="the most you can run at once" />
        <StatTile label="Clean money" value={formatMoney(empire.clean_money)} tone="good" />
      </div>

      <OpenFrontPanel empire={empire} rules={rules} types={types.data} typesError={types.error} onRetryTypes={() => void types.refetch()} loadingTypes={types.isPending} />
      <FrontList fronts={fronts} typeLabel={typeLabel} />
    </>
  );
}

interface OpenFrontPanelProps {
  empire: Empire;
  rules: Rules;
  types: FrontType[] | undefined;
  typesError: unknown;
  loadingTypes: boolean;
  onRetryTypes: () => void;
}

function OpenFrontPanel({ empire, rules, types, typesError, loadingTypes, onRetryTypes }: OpenFrontPanelProps) {
  const refresh = useRefreshGame();
  const [name, setName] = useState("");
  const [type, setType] = useState("");
  const [submitted, setSubmitted] = useState(false);

  const open = useMutation({
    mutationFn: (input: { name: string; type: string }) => api.empire.openFront(input),
    onSuccess: () => {
      setName("");
      setType("");
      setSubmitted(false);
      void refresh();
    },
  });

  const atLimit = activeFronts(empire) >= rules.max_active_fronts;
  const list = types ?? [];
  const errors = submitted ? validateFront({ name, type }, list.map((item) => item.type)) : {};
  const chosen = list.find((item) => item.type === type);
  const opened = open.data;

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    setSubmitted(true);
    if (Object.keys(validateFront({ name, type }, list.map((item) => item.type))).length > 0) return;
    open.mutate({ name: normalizeName(name), type });
  }

  return (
    <Panel title="Open a front" subtitle="A legitimate business to run your dirty money through">
      {atLimit ? (
        <p>
          You already run {formatNumber(rules.max_active_fronts)} fronts, the most allowed. Close one to open another.
        </p>
      ) : (
        <form onSubmit={onSubmit} noValidate>
          <div className="field">
            <label htmlFor="front-name">Name</label>
            <input
              id="front-name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              maxLength={FRONT_LIMITS.nameMax * 4}
              autoComplete="off"
              aria-invalid={errors.name ? true : undefined}
              aria-describedby={errors.name ? "front-name-error" : undefined}
              disabled={open.isPending}
            />
            <span className="field__hint">
              {FRONT_LIMITS.nameMin}-{FRONT_LIMITS.nameMax} characters.
            </span>
            {errors.name && (
              <span id="front-name-error" className="field__error">
                {errors.name}
              </span>
            )}
          </div>

          <div className="field">
            <label htmlFor="front-type">Type of business</label>
            <select
              id="front-type"
              value={type}
              onChange={(event) => setType(event.target.value)}
              disabled={open.isPending || loadingTypes || list.length === 0}
              aria-invalid={errors.type ? true : undefined}
              aria-describedby={errors.type ? "front-type-error" : chosen ? "front-type-hint" : undefined}
            >
              <option value="">{loadingTypes ? "Loading types\u2026" : "Choose a type"}</option>
              {list.map((item) => (
                <option key={item.type} value={item.type}>
                  {item.label}
                </option>
              ))}
            </select>
            {chosen && !errors.type && (
              <span id="front-type-hint" className="field__hint">
                {chosen.description} Suspicion builds {formatNumber(chosen.suspicion_factor, 2)}&times; as fast as average.
              </span>
            )}
            {errors.type && (
              <span id="front-type-error" className="field__error">
                {errors.type}
              </span>
            )}
          </div>

          <ErrorNotice error={typesError} title="Cannot load the business types" onRetry={onRetryTypes} />
          <ErrorNotice error={open.error} title="Could not open the front" />
          {opened && (
            <p className="result" role="status">
              {opened.name} is open. Start sending it dirty money.
            </p>
          )}
          <button type="submit" className="btn" disabled={open.isPending || loadingTypes || list.length === 0}>
            <Store aria-hidden="true" size={16} /> {open.isPending ? "Opening\u2026" : "Open front"}
          </button>
        </form>
      )}
    </Panel>
  );
}

function FrontList({ fronts, typeLabel }: { fronts: FrontBusiness[]; typeLabel: (type: string) => string }) {
  const openFronts = fronts.filter((front) => front.is_active);
  const closed = fronts.filter((front) => !front.is_active);

  return (
    <>
      <Panel title="Open fronts" subtitle="The DEA raids a front when its suspicion reaches 100">
        {openFronts.length === 0 ? (
          <p className="muted">No open fronts. You cannot launder money without one.</p>
        ) : (
          <ul className="front-list">
            {openFronts.map((front) => (
              <OpenFrontItem key={front.id} front={front} label={typeLabel(front.type)} />
            ))}
          </ul>
        )}
      </Panel>

      {closed.length > 0 && (
        <Panel title="Closed fronts" subtitle="Shut down or raided">
          <ul className="front-list">
            {closed.map((front) => (
              <li key={front.id} className="front-list__item front-list__item--closed">
                <div>
                  <strong>{front.name}</strong>
                  <p className="muted">{typeLabel(front.type)}</p>
                </div>
                <p className="muted">{front.shutdown_reason || "Closed"}</p>
              </li>
            ))}
          </ul>
        </Panel>
      )}
    </>
  );
}

function OpenFrontItem({ front, label }: { front: FrontBusiness; label: string }) {
  const refresh = useRefreshGame();
  const [confirming, setConfirming] = useState(false);
  const close = useMutation({
    mutationFn: (id: number) => api.empire.closeFront(id),
    onSuccess: () => {
      setConfirming(false);
      void refresh();
    },
  });

  return (
    <li className="front-list__item">
      <div>
        <strong>{front.name}</strong>
        <p className="muted">
          {label} &middot; {formatMoney(front.revenue)} revenue
        </p>
      </div>
      <SuspicionBar value={front.suspicion} />
      <div className="front-list__actions">
        {confirming ? (
          <>
            <button type="button" className="btn btn--danger" onClick={() => close.mutate(front.id)} disabled={close.isPending} aria-label={`Confirm closing ${front.name}`}>
              {close.isPending ? "Closing\u2026" : "Yes, close it"}
            </button>
            <button type="button" className="btn btn--ghost" onClick={() => setConfirming(false)} disabled={close.isPending}>
              Keep it
            </button>
          </>
        ) : (
          <button type="button" className="btn btn--ghost" onClick={() => setConfirming(true)} aria-label={`Close ${front.name}`}>
            <Trash2 aria-hidden="true" size={16} /> Close
          </button>
        )}
      </div>
      <ErrorNotice error={close.error} title={`Could not close ${front.name}`} />
    </li>
  );
}
