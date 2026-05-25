"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import api from "../lib/api";

const inputStyle = {
  width: "100%", background: "#111", border: "1px solid #1e1e1e",
  color: "#e8e4d8", padding: "14px 16px", fontSize: 14,
  fontFamily: "'Barlow', sans-serif", outline: "none",
};
const labelStyle = {
  fontFamily: "'Share Tech Mono', monospace", fontSize: 10,
  color: "#8a6f2e", letterSpacing: 3, display: "block" as const, marginBottom: 8
};

export default function Register() {
  const [form, setForm] = useState({ username: "", email: "", password: "", alias: "" });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  const handle = (k: string, v: string) => setForm(f => ({ ...f, [k]: v }));

  const handleRegister = async () => {
    setLoading(true);
    try {
      await api.post("/auth/register", form);
      router.push("/login");
    } catch {
      setError("Registration failed. Choose a different alias.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{
      minHeight: "100vh", background: "#060606", display: "flex",
      alignItems: "center", justifyContent: "center", fontFamily: "'Barlow', sans-serif",
      backgroundImage: `repeating-linear-gradient(0deg, transparent, transparent 60px, rgba(200,168,75,0.02) 60px, rgba(200,168,75,0.02) 61px),
        repeating-linear-gradient(90deg, transparent, transparent 60px, rgba(200,168,75,0.02) 60px, rgba(200,168,75,0.02) 61px)`
    }}>
      <div style={{ width: "100%", maxWidth: 440, padding: "0 24px" }}>
        <div style={{ marginBottom: 40, textAlign: "center" }}>
          <p style={{ fontFamily: "'Share Tech Mono', monospace", color: "#8a6f2e", fontSize: 10, letterSpacing: 6, marginBottom: 12 }}>NEW IDENTITY</p>
          <h1 style={{ fontFamily: "'Bebas Neue', sans-serif", fontSize: 56, letterSpacing: 8, color: "#c8a84b" }}>BUILD YOUR</h1>
          <h1 style={{ fontFamily: "'Bebas Neue', sans-serif", fontSize: 56, letterSpacing: 8, color: "#c8a84b", marginTop: -16 }}>EMPIRE</h1>
        </div>

        {error && (
          <div style={{ background: "rgba(192,57,43,0.1)", border: "1px solid rgba(192,57,43,0.3)", padding: "12px 16px", marginBottom: 24, fontFamily: "'Share Tech Mono', monospace", fontSize: 12, color: "#c0392b", letterSpacing: 1 }}>
            ⚠ {error}
          </div>
        )}

        {[
          { key: "username", label: "USERNAME", type: "text" },
          { key: "alias", label: "ALIAS (your street name)", type: "text" },
          { key: "email", label: "EMAIL", type: "email" },
          { key: "password", label: "PASSWORD", type: "password" },
        ].map(field => (
          <div key={field.key} style={{ marginBottom: 16 }}>
            <label style={labelStyle}>{field.label}</label>
            <input
              type={field.type}
              value={(form as any)[field.key]}
              onChange={e => handle(field.key, e.target.value)}
              style={inputStyle}
              onFocus={e => e.target.style.borderColor = "#c8a84b"}
              onBlur={e => e.target.style.borderColor = "#1e1e1e"}
            />
          </div>
        ))}

        <button
          onClick={handleRegister}
          disabled={loading}
          style={{
            width: "100%", marginTop: 16, background: loading ? "#8a6f2e" : "#c8a84b",
            color: "#060606", padding: "16px", border: "none", cursor: "pointer",
            fontFamily: "'Share Tech Mono', monospace", fontSize: 13, letterSpacing: 4, fontWeight: 700
          }}
        >
          {loading ? "CREATING..." : "SAY MY NAME"}
        </button>

        <p style={{ textAlign: "center", marginTop: 32, fontFamily: "'Share Tech Mono', monospace", fontSize: 11, color: "#3a3530" }}>
          Already in the game?{" "}
          <Link href="/login" style={{ color: "#8a6f2e", textDecoration: "none" }}>Sign in</Link>
        </p>
      </div>
    </div>
  );
}