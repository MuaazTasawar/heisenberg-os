import Link from "next/link";
import LiveStats from "./components/LiveStats";

export default function Home() {
  return (
    <main className="landing">
      <div className="landing__inner">
        <p className="landing__kicker">Classified &mdash; operation Heisenberg</p>
        <h1 className="landing__title">Heisenberg</h1>
        <p className="landing__subtitle">Operating System</p>
        <div className="landing__rule" />
        <p className="landing__blurb">
          Build your empire. Launder your money.
          <br />
          Stay ahead of the DEA. Better call Saul.
        </p>
        <div className="landing__actions">
          <Link href="/login" className="btn">
            Enter
          </Link>
          <Link href="/register" className="btn btn--ghost">
            New identity
          </Link>
        </div>
        <LiveStats />
      </div>
    </main>
  );
}
