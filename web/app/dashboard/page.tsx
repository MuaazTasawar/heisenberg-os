"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import api from "../lib/api";

const mono = "'Share Tech Mono', monospace";
const bebas = "'Bebas Neue', sans-serif";
const barlow = "'Barlow', sans-serif";

export default function Dashboard() {
  const [state, setState] = useState<any>(null);
  const router = useRouter();

  useEffect(() => {
    const token = localStorage.getItem("token");
    if (!token) { router.push("/login"); return; }
    api.get("/game/state").then(r => setState(r.data.data)).catch(() => router.push("/login"));
  }, []);

  if (!state) return (
    <div style={{ minHeight: "100vh", background: "#060606", display: "flex", alignItems: "center", justifyContent: "center" }}>
      <p style={{ fontFamily: mono, color: "#8a6f2e", letterSpacing: 4, fontSize: 12 }}>LOADING EMPIRE DATA...</p>
    </div>
  );

  const { empire, dea_threat } = state;
  const heat = dea_threat?.threat_score ?? 0;
  const heatColor = heat > 75 ? "#c0392b" : heat > 50 ? "#e67e22" : heat > 25 ? "#c8a84b" : "#27ae60";

  return (
    <div style={{ minHeight: "100vh", background: "#060606", fontFamily: barlow }}>

      <header style={{
        borderBottom: "1px solid #1a1a1a", padding: "0 40px",
        display: "flex", alignItems: "center", justifyContent: "space-between", height: 64
      }}>
        <div style={{ display: "flex", alignItems: "center", gap: 20 }}>
          <h1 style={{ fontFamily: bebas, fontSize: 28, letterSpacing: 6, color: "#c8a84b" }}>HEISENBERG OS</h1>
          <span style={{ width: 1, height: 24, background: "#1e1e1e" }} />
          <span style={{ fontFamily: mono, fontSize: 10, color: "#3a3530", letterSpacing: 3 }}>v2.0.1 — SECURE</span>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 24 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <div style={{ width: 6, height: 6, borderRadius: "50%", background: "#27ae60", boxShadow: "0 0 6px #27ae60" }} />
            <span style={{ fontFamily: mono, fontSize: 10, color: "#3a3530", letterSpacing: 2 }}>SYSTEMS ONLINE</span>
          </div>
          <button
            onClick={() => { localStorage.removeItem("token"); router.push("/login"); }}
            style={{ fontFamily: mono, fontSize: 10, color: "#3a3530", background: "none", border: "1px solid #1e1e1e", padding: "6px 16px", cursor: "pointer", letterSpacing: 2 }}
          >
            LOGOUT
          </button>
        </div>
      </header>

      <div style={{ padding: "40px" }}>

        <div style={{ marginBottom: 40 }}>
          <p style={{ fontFamily: mono, fontSize: 10, color: "#8a6f2e", letterSpacing: 4, marginBottom: 4 }}>ACTIVE EMPIRE</p>
          <h2 style={{ fontFamily: bebas, fontSize: 52, letterSpacing: 6, color: "#e8e4d8", lineHeight: 1 }}>{empire?.name || "EMPIRE"}</h2>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 2, marginBottom: 40 }}>
          {[
            { label: "PURITY RATING", value: `${(empire?.purity_rating ?? 0).toFixed(1)}%`, color: "#2980b9", sub: "Blue sky quality" },
            { label: "DIRTY MONEY", value: `$${((empire?.dirty_money ?? 0)).toLocaleString()}`, color: "#c0392b", sub: "Needs cleaning" },
            { label: "CLEAN MONEY", value: `$${((empire?.clean_money ?? 0)).toLocaleString()}`, color: "#27ae60", sub: "Laundered" },
            { label: "DEA HEAT", value: `${heat.toFixed(0)}%`, color: heatColor, sub: dea_threat?.lead_agent ?? "No agent assigned" },
          ].map(stat => (
            <div key={stat.label} style={{ background: "#0d0d0d", border: "1px solid #161616", padding: "24px 28px" }}>
              <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, marginBottom: 16 }}>{stat.label}</p>
              <p style={{ fontFamily: bebas, fontSize: 42, color: stat.color, letterSpacing: 2, lineHeight: 1, marginBottom: 8 }}>{stat.value}</p>
              <p style={{ fontFamily: mono, fontSize: 10, color: "#3a3530", letterSpacing: 1 }}>{stat.sub}</p>
            </div>
          ))}
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 2, marginBottom: 40 }}>
          <div style={{ background: "#0d0d0d", border: "1px solid #161616", padding: "20px 28px" }}>
            <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, marginBottom: 8 }}>BATCH YIELD</p>
            <p style={{ fontFamily: bebas, fontSize: 28, color: "#e8e4d8", letterSpacing: 2 }}>{(empire?.batch_yield ?? 0).toFixed(2)} kg</p>
          </div>
          <div style={{ background: "#0d0d0d", border: "1px solid #161616", padding: "20px 28px" }}>
            <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 3, marginBottom: 8 }}>TERRITORIES</p>
            <p style={{ fontFamily: bebas, fontSize: 28, color: "#e8e4d8", letterSpacing: 2 }}>{empire?.territory_count ?? 0} active</p>
          </div>
        </div>

        <p style={{ fontFamily: mono, fontSize: 9, color: "#3a3530", letterSpacing: 4, marginBottom: 16 }}>OPERATIONS</p>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 2 }}>
          {[
            { href: "/dashboard/empire", label: "COOK LAB", sub: "Manage synthesis", icon: "⚗", accent: "#2980b9" },
            { href: "/dashboard/launder", label: "LAUNDER", sub: "Clean the money", icon: "💵", accent: "#27ae60" },
            { href: "/dashboard/dea", label: "DEA STATUS", sub: "Monitor threat level", icon: "🚔", accent: "#c0392b" },
            { href: "/dashboard/legal", label: "CALL SAUL", sub: "Legal consultation", icon: "⚖", accent: "#c8a84b" },
          ].map(nav => (
            <Link key={nav.href} href={nav.href} style={{
              background: "#0d0d0d",
              borderTop: `2px solid ${nav.accent}`,
              borderLeft: "1px solid #161616",
              borderRight: "1px solid #161616",
              borderBottom: "1px solid #161616",
              padding: "28px", textDecoration: "none", display: "block",
              transition: "background 0.2s"
            }}
              onMouseEnter={e => (e.currentTarget.style.background = "#131313")}
              onMouseLeave={e => (e.currentTarget.style.background = "#0d0d0d")}
            >
              <p style={{ fontSize: 28, marginBottom: 16 }}>{nav.icon}</p>
              <p style={{ fontFamily: bebas, fontSize: 22, color: "#e8e4d8", letterSpacing: 3, marginBottom: 4 }}>{nav.label}</p>
              <p style={{ fontFamily: mono, fontSize: 10, color: "#3a3530", letterSpacing: 1 }}>{nav.sub}</p>
              <p style={{ fontFamily: mono, fontSize: 10, color: nav.accent, marginTop: 20, letterSpacing: 2 }}>ENTER →</p>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}