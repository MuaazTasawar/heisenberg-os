"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import api from "../../lib/api";

const mono = "'Share Tech Mono', monospace";
const bebas = "'Bebas Neue', sans-serif";

export default function DEAPage() {
  const [threat, setThreat] = useState<any>(null);

  useEffect(() => { api.get("/dea/threat").then(r => setThreat(r.data.data)); }, []);

  const score = threat?.threat_score ?? 0;
  const risk = score > 75 ? "CRITICAL" : score > 50 ? "HIGH" : score > 25 ? "ELEVATED" : "LOW";
  const riskColor = score > 75 ? "#c0392b" : score > 50 ? "#e67e22" : score > 25 ? "#c8a84b" : "#27ae60";

  return (
    <div style={{ minHeight: "100vh", background: "#060606", fontFamily: "'Barlow', sans-serif" }}>
      <header style={{ borderBottom: "1px solid #1a1a1a", padding: "0 40px", display: "flex", alignItems: "center", gap: 20, height: 64 }}>
        <Link href="/dashboard" style={{ fontFamily: mono, fontSize: 10, color: "#3a3530", letterSpacing: 3, textDecoration: "none" }}>← BACK</Link>
        <span style={{ width: 1, height: 20, background: "#1e1e1e" }} />
        <h1 style={{ fontFamily: bebas, fontSize: 24, letterSpacing: 6, color: "#c0392b" }}>DEA — OPERATION HEISENBERG</h1>
      </header>

      <div style={{ padding: "40px", maxWidth: 900 }}>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2, marginBottom: 2 }}>
          <div style={{ background: "#0d0d0d", borderTop: `2px solid ${riskColor}`, borderLeft: "1px solid #161616", borderRight: "1px solid #161616", borderBottom: "1px solid #161616", padding: "40px" }}>
            <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 4, marginBottom: 16 }}>THREAT SCORE</p>
            <p style={{ fontFamily: bebas, fontSize: 96, color: riskColor, lineHeight: 0.9, letterSpacing: 2, marginBottom: 16 }}>{score.toFixed(0)}%</p>
            <div style={{ display: "inline-block", background: `${riskColor}18`, border: `1px solid ${riskColor}40`, padding: "4px 14px" }}>
              <p style={{ fontFamily: mono, fontSize: 10, color: riskColor, letterSpacing: 4 }}>{risk}</p>
            </div>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {[
              { label: "LEAD AGENT", value: threat?.lead_agent ?? "Unknown" },
              { label: "EVIDENCE COUNT", value: `${threat?.evidence_count ?? 0} items` },
              { label: "CASE STATUS", value: threat?.is_active ? "ACTIVE" : "CLOSED" },
            ].map(item => (
              <div key={item.label} style={{ flex: 1, background: "#0d0d0d", border: "1px solid #161616", padding: "20px 24px", display: "flex", flexDirection: "column", justifyContent: "center" }}>
                <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, marginBottom: 8 }}>{item.label}</p>
                <p style={{ fontFamily: bebas, fontSize: 24, color: "#e8e4d8", letterSpacing: 2 }}>{item.value}</p>
              </div>
            ))}
          </div>
        </div>

        <div style={{ background: "#0d0d0d", border: "1px solid #161616", borderTop: "none", padding: "24px" }}>
          <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 4, marginBottom: 16 }}>THREAT GAUGE</p>
          <div style={{ background: "#060606", height: 8, width: "100%", position: "relative" }}>
            <div style={{ background: riskColor, height: "100%", width: `${score}%`, transition: "width 1s ease" }} />
          </div>
          <div style={{ display: "flex", justifyContent: "space-between", marginTop: 8 }}>
            {["0", "25", "50", "75", "100"].map(v => (
              <p key={v} style={{ fontFamily: mono, fontSize: 9, color: "#3a3530" }}>{v}</p>
            ))}
          </div>
        </div>

        <div style={{ marginTop: 2, background: "rgba(192,57,43,0.05)", border: "1px solid rgba(192,57,43,0.15)", padding: "16px 24px" }}>
          <p style={{ fontFamily: mono, fontSize: 11, color: "#c0392b", letterSpacing: 2 }}>
            {score > 75
              ? "⚠ CRITICAL — They know who you are. Contact Saul immediately."
              : score > 50
              ? "⚠ HIGH — Case is being built. Reduce operations."
              : score > 25
              ? "⚠ ELEVATED — Hank is sniffing around. Stay clean."
              : "✓ LOW — No active investigation. Continue operations."}
          </p>
        </div>
      </div>
    </div>
  );
}