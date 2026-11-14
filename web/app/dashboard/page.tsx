import type { Metadata } from "next";
import DashboardView from "./DashboardView";

export const metadata: Metadata = { title: "Command center" };

export default function DashboardPage() {
  return <DashboardView />;
}
