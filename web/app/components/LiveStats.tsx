"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { formatCompactMoney, formatKg, formatNumber } from "../lib/format";

const PLACEHOLDERS = ["empires", "kilograms", "laundered", "flagged", "raids"];

/** Real numbers from /stats. When they cannot be loaded nothing is shown in their place: never made-up figures. */
export default function LiveStats() {
  const stats = useQuery({
    queryKey: ["stats"],
    queryFn: () => api.stats.get(),
    staleTime: 30_000,
    refetchInterval: 60_000,
    retry: 1,
  });

  if (stats.isPending) {
    return (
      <section className="live-stats" aria-label="Live statistics" aria-busy="true">
        {PLACEHOLDERS.map((name) => (
          <div key={name} className="live-stats__item">
            <span className="skeleton skeleton--value" aria-hidden="true" />
            <span className="skeleton skeleton--label" aria-hidden="true" />
          </div>
        ))}
      </section>
    );
  }

  if (!stats.data) {
    return (
      <section className="live-stats" aria-label="Live statistics">
        <p className="live-stats__unavailable" role="status">
          Live statistics are unavailable right now.
        </p>
      </section>
    );
  }

  const items: [string, string][] = [
    ["Empires", formatNumber(stats.data.empires)],
    ["Kilograms cooked", formatKg(stats.data.kilograms_cooked)],
    ["Dirty money laundered", formatCompactMoney(stats.data.dirty_laundered)],
    ["Flagged transfers", formatNumber(stats.data.flagged_transfers)],
    ["DEA raids", formatNumber(stats.data.raids)],
  ];

  return (
    <section className="live-stats" aria-label="Live statistics">
      <dl className="live-stats__list">
        {items.map(([label, value]) => (
          <div key={label} className="live-stats__item">
            <dt className="live-stats__label">{label}</dt>
            <dd className="live-stats__value">{value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
