import type { Metadata } from "next";
import FrontsView from "./FrontsView";

export const metadata: Metadata = { title: "Front businesses" };

export default function FrontsPage() {
  return <FrontsView />;
}
