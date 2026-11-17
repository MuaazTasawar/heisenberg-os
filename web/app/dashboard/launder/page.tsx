import type { Metadata } from "next";
import LaunderView from "./LaunderView";

export const metadata: Metadata = { title: "Launder money" };

export default function LaunderPage() {
  return <LaunderView />;
}
