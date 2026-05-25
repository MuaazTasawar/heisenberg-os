"use client";
import { useState, useEffect } from "react";
import Link from "next/link";
import api from "../../lib/api";

const mono = "'Share Tech Mono', monospace";
const bebas = "'Bebas Neue', sans-serif";

export default function EmpirePage() {
  const [empire, setEmpire] = useState<any>(null);
  const [effort, setEffort] = useState(5);
  const [msg, setMsg] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => { api.get("/empire").then(r => setEmpire(r.data.data)); }, []);

  const cook = async () => {
    setLoading(true);
    const res = await api.post("/empire/cook", { effort });
    setEmpire(res.data.data);
    setMsg(`Batch complete — yield +${(effort * (res.data.data.purity_rating / 100) * 5).toFixed(2)}kg`);
    setLoading(false);
  };

  return (
    <div style={{ minHeight: "100vh", background: "#060606", fontFamily: "'Barlow', sans-serif" }}>
      <header style={{ borderBottom: "1px solid #1a1a1a", padding: "0 40px", display: "flex", alignItems: "center", gap: 20, height: 64 }}>
        <Link href="/dashboard" style={{ fontFamily: mono, fontSize: 10, color: "#3a3530", letterSpacing: 3, textDecoration: "none" }}>← BACK</Link>
        <span style={{ width: 1, height: 20, background: "#1e1e1e" }} />
        <h1 style={{ fontFamily: bebas, fontSize: 24, letterSpacing: 6, color: "#c8a84b" }}>COOK LAB</h1>
      </header>

      <div style={{ padding: "40px", maxWidth: 900 }}>
        {empire && (
          <>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 2, marginBottom: 40 }}>
              {[
                { label: "PURITY", value: `${empire.purity_rating?.toFixed(1)}%`, color: "#2980b9" },
                { label: "TOTAL YIELD", value: `${empire.batch_yield?.toFixed(2)} kg`, color: "#c8a84b" },
                { label: "HEAT LEVEL", value: `${empire.heat_level?.toFixed(1)}%`, color: empire.heat_level > 50 ? "#c0392b" : "#27ae60" },
              ].map(s => (
                <div key={s.label} style={{ background: "#0d0d0d", border: "1px solid #161616", padding: "24px" }}>
                  <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, marginBottom: 12 }}>{s.label}</p>
                  <p style={{ fontFamily: bebas, fontSize: 36, color: s.color, letterSpacing: 2 }}>{s.value}</p>
                </div>
              ))}
            </div>

            <div style={{ background: "#0d0d0d", border: "1px solid #161616", padding: "32px", marginBottom: 24 }}>
              <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 4, marginBottom: 24 }}>COOK INTENSITY</p>

              <div style={{ display: "flex", alignItems: "center", gap: 20, marginBottom: 32 }}>
                <input
                  type="range" min={1} max={10} value={effort}
                  onChange={e => setEffort(Number(e.target.value))}
                  style={{ flex: 1, accentColor: "#c8a84b", height: 4 }}
                />
                <div style={{ background: "#060606", border: "1px solid #c8a84b", padding: "8px 20px", minWidth: 60, textAlign: "center" }}>
                  <p style={{ fontFamily: bebas, fontSize: 28, color: "#c8a84b", lineHeight: 1 }}>{effort}</p>
                  <p style={{ fontFamily: mono, fontSize: 8, color: "#8a6f2e", letterSpacing: 2 }}>/ 10</p>
                </div>
              </div>

              <div style={{ display: "flex", gap: 16, marginBottom: 24, flexWrap: "wrap" }}>
                {[
                  { label: "YIELD GAIN", value: `+${(effort * (empire.purity_rating / 100) * 5).toFixed(2)} kg` },
                  { label: "HEAT INCREASE", value: `+${(effort * 2.5).toFixed(1)}%` },
                  { label: "EARNINGS", value: `+$${((effort * (empire.purity_rating / 100) * 5) * 1500).toLocaleString()}` },
                ].map(p => (
                  <div key={p.label} style={{ flex: 1, background: "#060606", border: "1px solid #161616", padding: "12px 16px", minWidth: 120 }}>
                    <p style={{ fontFamily: mono, fontSize: 8, color: "#3a3530", letterSpacing: 2, marginBottom: 4 }}>{p.label}</p>
                    <p style={{ fontFamily: bebas, fontSize: 20, color: "#c8a84b", letterSpacing: 2 }}>{p.value}</p>
                  </div>
                ))}
              </div>

              <button
                onClick={cook}
                disabled={loading}
                style={{
                  background: loading ? "#8a6f2e" : "#c8a84b",
                  color: "#060606", padding: "14px 40px", border: "none",
                  cursor: "pointer", fontFamily: mono, fontSize: 12,
                  letterSpacing: 4, fontWeight: 700
                }}
              >
                {loading ? "COOKING..." : "INITIATE COOK"}
              </button>
            </div>

            {msg && (
              <div style={{ background: "rgba(39,174,96,0.08)", border: "1px solid rgba(39,174,96,0.2)", padding: "12px 20px" }}>
                <p style={{ fontFamily: mono, fontSize: 11, color: "#27ae60", letterSpacing: 2 }}>✓ {msg}</p>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}