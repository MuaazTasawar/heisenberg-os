"use client";
import { useState } from "react";
import Link from "next/link";
import api from "../../lib/api";

const mono = "'Share Tech Mono', monospace";
const bebas = "'Bebas Neue', sans-serif";

export default function LegalPage() {
  const [charge, setCharge] = useState("");
  const [evidence, setEvidence] = useState("");
  const [context, setContext] = useState("");
  const [advice, setAdvice] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const consult = async () => {
    setLoading(true);
    const res = await api.post("/legal/consult", { charge, evidence, player_context: context });
    setAdvice(res.data.data);
    setLoading(false);
  };

  const inputStyle = {
    width: "100%", background: "#060606", border: "1px solid #1e1e1e",
    color: "#e8e4d8", padding: "12px 16px", fontSize: 14,
    fontFamily: "'Barlow', sans-serif", outline: "none",
  };

  return (
    <div style={{ minHeight: "100vh", background: "#060606", fontFamily: "'Barlow', sans-serif" }}>
      <header style={{ borderBottom: "1px solid #1a1a1a", padding: "0 40px", display: "flex", alignItems: "center", gap: 20, height: 64 }}>
        <Link href="/dashboard" style={{ fontFamily: mono, fontSize: 10, color: "#3a3530", letterSpacing: 3, textDecoration: "none" }}>← BACK</Link>
        <span style={{ width: 1, height: 20, background: "#1e1e1e" }} />
        <h1 style={{ fontFamily: bebas, fontSize: 24, letterSpacing: 6, color: "#c8a84b" }}>SAUL GOODMAN — LEGAL COUNSEL</h1>
      </header>

      <div style={{ padding: "40px", maxWidth: 900 }}>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>

          <div style={{ background: "#0d0d0d", border: "1px solid #161616", padding: "32px" }}>
            <p style={{ fontFamily: mono, fontSize: 9, color: "#8a6f2e", letterSpacing: 4, marginBottom: 24 }}>FILE YOUR CASE</p>

            <div style={{ marginBottom: 16 }}>
              <label style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, display: "block", marginBottom: 8 }}>CHARGE</label>
              <input
                value={charge} onChange={e => setCharge(e.target.value)} style={inputStyle}
                placeholder="e.g. Money laundering, possession..."
                onFocus={e => e.target.style.borderColor = "#c8a84b"}
                onBlur={e => e.target.style.borderColor = "#1e1e1e"}
              />
            </div>

            <div style={{ marginBottom: 16 }}>
              <label style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, display: "block", marginBottom: 8 }}>EVIDENCE AGAINST YOU</label>
              <input
                value={evidence} onChange={e => setEvidence(e.target.value)} style={inputStyle}
                placeholder="What do they have..."
                onFocus={e => e.target.style.borderColor = "#c8a84b"}
                onBlur={e => e.target.style.borderColor = "#1e1e1e"}
              />
            </div>

            <div style={{ marginBottom: 24 }}>
              <label style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, display: "block", marginBottom: 8 }}>YOUR SITUATION</label>
              <textarea
                value={context} onChange={e => setContext(e.target.value)}
                rows={4}
                style={{ ...inputStyle, resize: "vertical" as const }}
                placeholder="Describe what happened..."
                onFocus={e => e.target.style.borderColor = "#c8a84b"}
                onBlur={e => e.target.style.borderColor = "#1e1e1e"}
              />
            </div>

            <button
              onClick={consult} disabled={loading || !charge}
              style={{
                width: "100%", background: loading ? "#8a6f2e" : "#c8a84b",
                color: "#060606", padding: "14px", border: "none",
                cursor: charge ? "pointer" : "not-allowed",
                fontFamily: mono, fontSize: 12, letterSpacing: 4, fontWeight: 700
              }}
            >
              {loading ? "SAUL IS THINKING..." : "BETTER CALL SAUL"}
            </button>
          </div>

          <div style={{ background: "#0d0d0d", border: "1px solid #161616", padding: "32px", display: "flex", flexDirection: "column" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 24 }}>
              <div style={{ width: 44, height: 44, background: "#c8a84b", display: "flex", alignItems: "center", justifyContent: "center" }}>
                <span style={{ fontFamily: bebas, fontSize: 20, color: "#060606" }}>SG</span>
              </div>
              <div>
                <p style={{ fontFamily: bebas, fontSize: 18, color: "#c8a84b", letterSpacing: 3 }}>SAUL GOODMAN</p>
                <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 2 }}>505-503-4455 — I FIGHT FOR YOU</p>
              </div>
            </div>

            <div style={{ flex: 1, background: "#060606", border: "1px solid #161616", padding: "20px", minHeight: 200 }}>
              {advice ? (
                <>
                  <p style={{ fontFamily: mono, fontSize: 9, color: "#8a6f2e", letterSpacing: 3, marginBottom: 16 }}>LEGAL ADVICE</p>
                  <p style={{ fontFamily: "'Barlow', sans-serif", fontSize: 14, color: "#e8e4d8", lineHeight: 1.8, whiteSpace: "pre-wrap" }}>{advice.advice}</p>
                </>
              ) : (
                <div style={{ height: "100%", display: "flex", alignItems: "center", justifyContent: "center" }}>
                  <p style={{ fontFamily: mono, fontSize: 10, color: "#1e1e1e", letterSpacing: 3, textAlign: "center" }}>AWAITING CONSULTATION<br />FILE YOUR CASE →</p>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}