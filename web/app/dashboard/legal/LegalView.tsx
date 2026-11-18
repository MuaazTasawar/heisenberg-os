"use client";

import { useState, type FormEvent } from "react";
import { keepPreviousData, useMutation, useQuery } from "@tanstack/react-query";
import { Scale } from "lucide-react";
import DashboardShell from "../../components/DashboardShell";
import ErrorNotice from "../../components/ErrorNotice";
import Pagination from "../../components/Pagination";
import Panel from "../../components/Panel";
import RiskBadge from "../../components/RiskBadge";
import { api } from "../../lib/api";
import { formatRelative } from "../../lib/format";
import { KEYS, useRefreshGame } from "../../lib/queries";
import type { CaseStatus, ConsultInput, LegalCase } from "../../lib/types";
import { CONSULT_LIMITS, runeCount, validateConsult } from "../../lib/validation";

const CASES_PAGE_SIZE = 10;
const STATUS_LABEL: Record<CaseStatus, string> = { open: "Open", dismissed: "Dismissed", convicted: "Convicted" };

export default function LegalView() {
  return (
    <DashboardShell title="Legal">
      <ConsultPanel />
      <CaseFiles />
    </DashboardShell>
  );
}

function Counter({ id, length, max }: { id: string; length: number; max: number }) {
  return (
    <span id={id} className="field__hint counter" data-over={length > max ? "true" : undefined}>
      {length} / {max}
    </span>
  );
}

function ConsultPanel() {
  const refresh = useRefreshGame();
  const [charge, setCharge] = useState("");
  const [evidence, setEvidence] = useState("");
  const [context, setContext] = useState("");
  const [submitted, setSubmitted] = useState(false);

  const consult = useMutation({
    mutationFn: (input: ConsultInput) => api.legal.consult(input),
    onSuccess: () => {
      setCharge("");
      setEvidence("");
      setContext("");
      setSubmitted(false);
      void refresh();
    },
  });

  const errors = submitted ? validateConsult({ charge, evidence, context }) : {};

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    setSubmitted(true);
    if (Object.keys(validateConsult({ charge, evidence, context })).length > 0) return;
    const input: ConsultInput = { charge: charge.trim() };
    if (evidence.trim()) input.evidence = evidence.trim();
    if (context.trim()) input.player_context = context.trim();
    consult.mutate(input);
  }

  return (
    <Panel title="Better call Saul" subtitle="Tell him what you are charged with and he will tell you what to say (and what not to)">
      <form onSubmit={onSubmit} noValidate>
        <div className="field">
          <label htmlFor="consult-charge">The charge</label>
          <input
            id="consult-charge"
            value={charge}
            onChange={(event) => setCharge(event.target.value)}
            autoComplete="off"
            disabled={consult.isPending}
            aria-invalid={errors.charge ? true : undefined}
            aria-describedby={errors.charge ? "consult-charge-error consult-charge-count" : "consult-charge-count"}
          />
          <Counter id="consult-charge-count" length={runeCount(charge.trim())} max={CONSULT_LIMITS.chargeMax} />
          {errors.charge && (
            <span id="consult-charge-error" className="field__error">
              {errors.charge}
            </span>
          )}
        </div>

        <div className="field">
          <label htmlFor="consult-evidence">What they have on you (optional)</label>
          <textarea
            id="consult-evidence"
            rows={4}
            value={evidence}
            onChange={(event) => setEvidence(event.target.value)}
            disabled={consult.isPending}
            aria-invalid={errors.evidence ? true : undefined}
            aria-describedby={errors.evidence ? "consult-evidence-error consult-evidence-count" : "consult-evidence-count"}
          />
          <Counter id="consult-evidence-count" length={runeCount(evidence.trim())} max={CONSULT_LIMITS.textMax} />
          {errors.evidence && (
            <span id="consult-evidence-error" className="field__error">
              {errors.evidence}
            </span>
          )}
        </div>

        <div className="field">
          <label htmlFor="consult-context">Your situation (optional)</label>
          <textarea
            id="consult-context"
            rows={4}
            value={context}
            onChange={(event) => setContext(event.target.value)}
            disabled={consult.isPending}
            aria-invalid={errors.context ? true : undefined}
            aria-describedby={errors.context ? "consult-context-error consult-context-count" : "consult-context-count"}
          />
          <Counter id="consult-context-count" length={runeCount(context.trim())} max={CONSULT_LIMITS.textMax} />
          {errors.context && (
            <span id="consult-context-error" className="field__error">
              {errors.context}
            </span>
          )}
        </div>

        <ErrorNotice error={consult.error} title="Saul could not take your case" />
        {consult.isPending && (
          <p className="muted" role="status">
            Saul is thinking. This can take a few seconds&hellip;
          </p>
        )}
        <button type="submit" className="btn" disabled={consult.isPending}>
          <Scale aria-hidden="true" size={16} /> {consult.isPending ? "Asking Saul\u2026" : "Ask Saul"}
        </button>
      </form>

      {consult.data && (
        <section className="case-result" aria-label="Saul's advice">
          <CaseHeading legalCase={consult.data} />
          <CaseBody legalCase={consult.data} />
        </section>
      )}
    </Panel>
  );
}

function CaseHeading({ legalCase }: { legalCase: LegalCase }) {
  return (
    <div className="case-heading">
      <strong className="case-heading__charge">{legalCase.charge}</strong>
      <RiskBadge level={legalCase.risk} />
      <span className="muted">{STATUS_LABEL[legalCase.status] ?? legalCase.status}</span>
      <span className="muted">severity {legalCase.severity} of 4</span>
    </div>
  );
}

function CaseBody({ legalCase }: { legalCase: LegalCase }) {
  return (
    <div className="case-body">
      {legalCase.evidence && (
        <>
          <h3 className="case-body__title">Evidence</h3>
          <p className="case-body__text">{legalCase.evidence}</p>
        </>
      )}
      <h3 className="case-body__title">Advice</h3>
      <p className="case-body__text">{legalCase.advice}</p>
      <h3 className="case-body__title">Defense</h3>
      <p className="case-body__text">{legalCase.defense}</p>
    </div>
  );
}

function CaseFiles() {
  const [offset, setOffset] = useState(0);
  const query = { limit: CASES_PAGE_SIZE, offset };
  const cases = useQuery({
    queryKey: KEYS.cases(query),
    queryFn: () => api.legal.cases(query),
    placeholderData: keepPreviousData,
  });
  const page = cases.data;

  return (
    <Panel title="Your case files" subtitle="Newest first. Open one to read what Saul said">
      <ErrorNotice error={cases.error} title="Cannot load your case files" onRetry={() => void cases.refetch()} />
      {cases.isPending && (
        <p className="muted" role="status">
          Pulling your files&hellip;
        </p>
      )}
      {page && page.items.length === 0 && <p className="muted">No case files yet. Keep it that way.</p>}
      {page && page.items.length > 0 && (
        <ul className="case-files" aria-busy={cases.isPlaceholderData}>
          {page.items.map((legalCase) => (
            <li key={legalCase.id}>
              <details className="case-file">
                <summary>
                  <span className="case-file__charge">{legalCase.charge}</span>
                  <RiskBadge level={legalCase.risk} />
                  <span className="muted">{STATUS_LABEL[legalCase.status] ?? legalCase.status}</span>
                  <span className="muted">{formatRelative(legalCase.created_at)}</span>
                </summary>
                <CaseBody legalCase={legalCase} />
              </details>
            </li>
          ))}
        </ul>
      )}
      {page && <Pagination total={page.total} limit={page.limit || CASES_PAGE_SIZE} offset={page.offset} onChange={setOffset} label="Cases" />}
    </Panel>
  );
}
