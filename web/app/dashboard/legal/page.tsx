import type { Metadata } from "next";
import LegalView from "./LegalView";

export const metadata: Metadata = { title: "Legal" };

export default function LegalPage() {
  return <LegalView />;
}
