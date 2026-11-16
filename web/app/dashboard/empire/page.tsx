import type { Metadata } from "next";
import EmpireView from "./EmpireView";

export const metadata: Metadata = { title: "Empire" };

export default function EmpirePage() {
  return <EmpireView />;
}
