"use client";
import { useState, useEffect } from "react";
import Link from "next/link";
import api from "../../lib/api";

const mono = "'Share Tech Mono', monospace";
const bebas = "'Bebas Neue', sans-serif";

export default function LaunderPage() {
  const [amount, setAmount] = useState("");
  const [businessId, setBusinessId] = useState("1");
  const [result, setResult] = useState<any>(null);
  const [txs, setTxs] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => { api.get("/transactions").then(r => setTxs(r.data.data || [])); }, []);

  const launder = async () => {
    setLoading(true);
    try {
      const res = await api.post("/launder", { amount: Number(amount), business_id: Number(businessId) });
      setResult(res.data.data);
      const txRes = await api.get("/transactions");
      setTxs(txRes.data.data || []);
    } catch { }
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
        <h1 style={{ fontFamily: bebas, fontSize: 24, letterSpacing: 6, color: "#27ae60" }}>LOS POLLOS — LAUNDERING</h1>
      </header>

      <div style={{ padding: "40px", maxWidth: 1100 }}>
        <div style={{ display: "grid", gridTemplateColumns: "360px 1fr", gap: 2 }}>

          <div style={{ background: "#0d0d0d", border: "1px solid #161616", padding: "32px" }}>
            <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 4, marginBottom: 24 }}>INITIATE TRANSFER</p>

            <div style={{ marginBottom: 16 }}>
              <label style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, display: "block", marginBottom: 8 }}>AMOUNT ($)</label>
              <input
                type="number" value={amount} onChange={e => setAmount(e.target.value)}
                style={inputStyle} placeholder="0.00"
                onFocus={e => e.target.style.borderColor = "#27ae60"}
                onBlur={e => e.target.style.borderColor = "#1e1e1e"}
              />
            </div>

            <div style={{ marginBottom: 24 }}>
              <label style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, display: "block", marginBottom: 8 }}>BUSINESS ID</label>
              <input
                type="number" value={businessId} onChange={e => setBusinessId(e.target.value)}
                style={inputStyle}
                onFocus={e => e.target.style.borderColor = "#27ae60"}
                onBlur={e => e.target.style.borderColor = "#1e1e1e"}
              />
            </div>

            <div style={{ background: "#060606", border: "1px solid #161616", padding: "12px 16px", marginBottom: 24 }}>
              <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 2, marginBottom: 4 }}>AFTER LAUNDERING (75%)</p>
              <p style={{ fontFamily: bebas, fontSize: 28, color: "#27ae60", letterSpacing: 2 }}>
                ${amount ? (Number(amount) * 0.75).toLocaleString() : "0"}
              </p>
            </div>

            <button
              onClick={launder} disabled={loading || !amount}
              style={{
                width: "100%", background: loading ? "#1a7a3a" : "#27ae60",
                color: "#060606", padding: "14px", border: "none",
                cursor: amount ? "pointer" : "not-allowed",
                fontFamily: mono, fontSize: 12, letterSpacing: 4, fontWeight: 700
              }}
            >
              {loading ? "PROCESSING..." : "LAUNDER MONEY"}
            </button>

            {result && (
              <div style={{ marginTop: 16, background: "rgba(39,174,96,0.08)", border: "1px solid rgba(39,174,96,0.2)", padding: "12px 16px" }}>
                <p style={{ fontFamily: mono, fontSize: 10, color: "#27ae60", letterSpacing: 2 }}>✓ ${result.amount?.toLocaleString()} cleaned</p>
              </div>
            )}
          </div>

          <div style={{ background: "#0d0d0d", border: "1px solid #161616", padding: "32px" }}>
            <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 4, marginBottom: 24 }}>TRANSACTION LEDGER</p>
            <div style={{ display: "flex", gap: 0, flexDirection: "column" }}>
              {txs.length === 0 ? (
                <p style={{ fontFamily: mono, fontSize: 10, color: "#1e1e1e", letterSpacing: 2 }}>NO TRANSACTIONS YET</p>
              ) : txs.map((tx: any, i: number) => (
                <div key={i} style={{
                  display: "flex", alignItems: "center", justifyContent: "space-between",
                  padding: "12px 16px", borderBottom: "1px solid #111",
                  background: tx.flagged ? "rgba(192,57,43,0.05)" : "transparent"
                }}>
                  <div style={{ flex: 1 }}>
                    <p style={{ fontFamily: mono, fontSize: 10, color: "#7a7570", letterSpacing: 1 }}>{tx.description}</p>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
                    {tx.flagged && (
                      <span style={{ fontFamily: mono, fontSize: 9, color: "#c0392b", border: "1px solid rgba(192,57,43,0.3)", padding: "2px 8px", letterSpacing: 2 }}>FLAGGED</span>
                    )}
                    <p style={{ fontFamily: bebas, fontSize: 18, color: tx.type === "laundered" ? "#27ae60" : "#c0392b", letterSpacing: 2 }}>
                      ${tx.amount?.toLocaleString()}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}