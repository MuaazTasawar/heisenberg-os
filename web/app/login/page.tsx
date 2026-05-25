"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import api from "../lib/api";

export default function Login() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  const handleLogin = async () => {
    setLoading(true);
    setError("");
    try {
      const res = await api.post("/auth/login", { email, password });
      localStorage.setItem("token", res.data.data.token);
      router.push("/dashboard");
    } catch {
      setError("Identity not recognized. Try again.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{
      minHeight: "100vh", background: "#060606", display: "flex",
      alignItems: "center", justifyContent: "center",
      fontFamily: "'Barlow', sans-serif",
      backgroundImage: `repeating-linear-gradient(0deg, transparent, transparent 60px, rgba(200,168,75,0.02) 60px, rgba(200,168,75,0.02) 61px),
        repeating-linear-gradient(90deg, transparent, transparent 60px, rgba(200,168,75,0.02) 60px, rgba(200,168,75,0.02) 61px)`
    }}>
      <div style={{ width: "100%", maxWidth: 420, padding: "0 24px" }}>

        <div style={{ marginBottom: 48, textAlign: "center" }}>
          <p style={{ fontFamily: "'Share Tech Mono', monospace", color: "#8a6f2e", fontSize: 10, letterSpacing: 6, marginBottom: 12 }}>SECURE ACCESS</p>
          <h1 style={{ fontFamily: "'Bebas Neue', sans-serif", fontSize: 56, letterSpacing: 8, color: "#c8a84b" }}>IDENTIFY</h1>
          <h1 style={{ fontFamily: "'Bebas Neue', sans-serif", fontSize: 56, letterSpacing: 8, color: "#c8a84b", marginTop: -16 }}>YOURSELF</h1>
        </div>

        {error && (
          <div style={{
            background: "rgba(192,57,43,0.1)", border: "1px solid rgba(192,57,43,0.3)",
            padding: "12px 16px", marginBottom: 24,
            fontFamily: "'Share Tech Mono', monospace", fontSize: 12, color: "#c0392b", letterSpacing: 1
          }}>
            ⚠ {error}
          </div>
        )}

        <div style={{ marginBottom: 16 }}>
          <label style={{ fontFamily: "'Share Tech Mono', monospace", fontSize: 10, color: "#8a6f2e", letterSpacing: 3, display: "block", marginBottom: 8 }}>EMAIL</label>
          <input
            value={email}
            onChange={e => setEmail(e.target.value)}
            onKeyDown={e => e.key === "Enter" && handleLogin()}
            style={{
              width: "100%", background: "#111", border: "1px solid #1e1e1e",
              color: "#e8e4d8", padding: "14px 16px", fontSize: 14,
              fontFamily: "'Barlow', sans-serif", outline: "none",
              transition: "border-color 0.2s"
            }}
            onFocus={e => e.target.style.borderColor = "#c8a84b"}
            onBlur={e => e.target.style.borderColor = "#1e1e1e"}
          />
        </div>

        <div style={{ marginBottom: 32 }}>
          <label style={{ fontFamily: "'Share Tech Mono', monospace", fontSize: 10, color: "#8a6f2e", letterSpacing: 3, display: "block", marginBottom: 8 }}>PASSWORD</label>
          <input
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            onKeyDown={e => e.key === "Enter" && handleLogin()}
            style={{
              width: "100%", background: "#111", border: "1px solid #1e1e1e",
              color: "#e8e4d8", padding: "14px 16px", fontSize: 14,
              fontFamily: "'Barlow', sans-serif", outline: "none",
              transition: "border-color 0.2s"
            }}
            onFocus={e => e.target.style.borderColor = "#c8a84b"}
            onBlur={e => e.target.style.borderColor = "#1e1e1e"}
          />
        </div>

        <button
          onClick={handleLogin}
          disabled={loading}
          style={{
            width: "100%", background: loading ? "#8a6f2e" : "#c8a84b",
            color: "#060606", padding: "16px", border: "none", cursor: "pointer",
            fontFamily: "'Share Tech Mono', monospace", fontSize: 13,
            letterSpacing: 4, fontWeight: 700, transition: "all 0.2s"
          }}
        >
          {loading ? "VERIFYING..." : "I AM THE ONE WHO KNOCKS"}
        </button>

        <p style={{ textAlign: "center", marginTop: 32, fontFamily: "'Share Tech Mono', monospace", fontSize: 11, color: "#3a3530" }}>
          No identity?{" "}
          <Link href="/register" style={{ color: "#8a6f2e", textDecoration: "none" }}>
            Create one
          </Link>
        </p>
      </div>
    </div>
  );
}