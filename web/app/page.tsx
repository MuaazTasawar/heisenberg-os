import Link from "next/link";

export default function Home() {
  return (
    <main style={{
      minHeight: "100vh",
      background: "#060606",
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      justifyContent: "center",
      position: "relative",
      overflow: "hidden",
      fontFamily: "'Barlow', sans-serif"
    }}>
      <div style={{
        position: "absolute", inset: 0,
        backgroundImage: `repeating-linear-gradient(0deg, transparent, transparent 60px, rgba(200,168,75,0.03) 60px, rgba(200,168,75,0.03) 61px),
          repeating-linear-gradient(90deg, transparent, transparent 60px, rgba(200,168,75,0.03) 60px, rgba(200,168,75,0.03) 61px)`,
        pointerEvents: "none"
      }} />

      <div style={{ position: "relative", textAlign: "center", maxWidth: 700, padding: "0 2rem" }}>
        <p style={{ fontFamily: "'Share Tech Mono', monospace", color: "#8a6f2e", fontSize: 11, letterSpacing: 8, marginBottom: 32, textTransform: "uppercase" }}>
          classified — operation heisenberg
        </p>

        <h1 style={{
          fontFamily: "'Bebas Neue', sans-serif",
          fontSize: "clamp(72px, 12vw, 140px)",
          letterSpacing: 12,
          color: "#c8a84b",
          lineHeight: 0.9,
          marginBottom: 8,
        }}>
          Heisenberg
        </h1>
        <h2 style={{
          fontFamily: "'Bebas Neue', sans-serif",
          fontSize: "clamp(28px, 5vw, 52px)",
          letterSpacing: 20,
          color: "#3a3530",
          marginBottom: 48,
        }}>
          Operating System
        </h2>

        <div style={{ width: 60, height: 1, background: "#c8a84b", margin: "0 auto 48px" }} />

        <p style={{ color: "#7a7570", fontSize: 14, letterSpacing: 2, marginBottom: 64, lineHeight: 1.8 }}>
          Build your empire. Launder your money.<br />
          Stay ahead of the DEA. Better call Saul.
        </p>

        <div style={{ display: "flex", gap: 16, justifyContent: "center", flexWrap: "wrap" }}>
          <Link href="/login" style={{
            fontFamily: "'Share Tech Mono', monospace",
            background: "#c8a84b",
            color: "#060606",
            padding: "14px 40px",
            fontSize: 13,
            letterSpacing: 4,
            textDecoration: "none",
            fontWeight: 600,
            transition: "all 0.2s",
          }}>
            ENTER
          </Link>
          <Link href="/register" style={{
            fontFamily: "'Share Tech Mono', monospace",
            border: "1px solid #c8a84b",
            color: "#c8a84b",
            padding: "14px 40px",
            fontSize: 13,
            letterSpacing: 4,
            textDecoration: "none",
            transition: "all 0.2s",
          }}>
            NEW IDENTITY
          </Link>
        </div>

        <div style={{ marginTop: 80, display: "flex", gap: 48, justifyContent: "center", flexWrap: "wrap" }}>
          {[["99.1%", "PURITY"], ["$47M", "LAUNDERED"], ["0", "CONVICTIONS"]].map(([val, label]) => (
            <div key={label} style={{ textAlign: "center" }}>
              <p style={{ fontFamily: "'Bebas Neue'", fontSize: 36, color: "#c8a84b", letterSpacing: 4 }}>{val}</p>
              <p style={{ fontFamily: "'Share Tech Mono', monospace", fontSize: 10, color: "#3a3530", letterSpacing: 3 }}>{label}</p>
            </div>
          ))}
        </div>
      </div>
    </main>
  );
}