import type { Metadata } from "next";
import DeaView from "./DeaView";

export const metadata: Metadata = { title: "DEA file" };

export default function DeaPage() {
  return <DeaView />;
}
