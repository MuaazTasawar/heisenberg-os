"use client";

import { CartesianGrid, Line, LineChart, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { formatDate, formatDateTime } from "../../lib/format";
import type { ThreatPoint } from "../../lib/dea";

const BAND_EDGES = [25, 50, 75];

/** The threat score over time, on the same 0-100 scale and bands as everywhere else. */
export default function ThreatChart({ points }: { points: ThreatPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height={260}>
      <LineChart data={points} margin={{ top: 8, right: 16, bottom: 4, left: 0 }}>
        <CartesianGrid stroke="#1e1e1e" vertical={false} />
        {BAND_EDGES.map((edge) => (
          <ReferenceLine key={edge} y={edge} stroke="#2a2a2a" strokeDasharray="4 4" />
        ))}
        <XAxis dataKey="time" type="number" domain={["dataMin", "dataMax"]} tickFormatter={(value: number) => formatDate(value)} stroke="#7a7570" fontSize={12} />
        <YAxis domain={[0, 100]} ticks={[0, 25, 50, 75, 100]} stroke="#7a7570" fontSize={12} width={36} />
        <Tooltip
          labelFormatter={(value) => formatDateTime(Number(value))}
          formatter={(value) => [`${value} / 100`, "Threat score"]}
          contentStyle={{ background: "#111111", border: "1px solid #2a2a2a", borderRadius: 4 }}
        />
        <Line type="monotone" dataKey="score" stroke="#c8a84b" strokeWidth={2} dot={{ r: 3 }} isAnimationActive={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}
