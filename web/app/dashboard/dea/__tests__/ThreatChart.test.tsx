import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { formatDate, formatDateTime } from "../../../lib/format";
import type { ThreatPoint } from "../../../lib/dea";
import ThreatChart from "../ThreatChart";

// recharts measures the page to draw, which a test environment cannot do. These stand-ins record what
// the chart is given, so what is checked is our configuration: the data, the scale and the bands.
vi.mock("recharts", async () => {
  const React = await import("react");
  const box = (name: string, render: (props: Record<string, unknown>) => unknown) =>
    function Stub(props: Record<string, unknown>) {
      return React.createElement("div", { "data-testid": name, "data-content": String(render(props)) }, props.children as never);
    };
  return {
    ResponsiveContainer: box("container", (props) => `height ${String(props.height)}`),
    LineChart: box("chart", (props) => JSON.stringify(props.data)),
    CartesianGrid: () => null,
    ReferenceLine: box("band", (props) => String(props.y)),
    XAxis: box("x-axis", (props) => `${String(props.dataKey)} ${(props.tickFormatter as (value: number) => string)(Date.UTC(2026, 10, 17, 12))}`),
    YAxis: box("y-axis", (props) => JSON.stringify({ domain: props.domain, ticks: props.ticks })),
    Tooltip: box("tooltip", (props) =>
      JSON.stringify({
        label: (props.labelFormatter as (value: number) => string)(Date.UTC(2026, 10, 17, 12)),
        value: (props.formatter as (value: number) => unknown)(42),
      }),
    ),
    Line: box("line", (props) => `${String(props.dataKey)} ${String(props.stroke)}`),
  };
});

const POINTS: ThreatPoint[] = [
  { time: Date.UTC(2026, 10, 10), score: 12.5, evidence: 1, level: "low" },
  { time: Date.UTC(2026, 10, 12), score: 61, evidence: 4, level: "high" },
];

const text = (id: string) => screen.getByTestId(id).getAttribute("data-content") ?? "";

describe("ThreatChart", () => {
  it("draws the points it is given, in the order given", () => {
    render(<ThreatChart points={POINTS} />);
    expect(JSON.parse(text("chart"))).toEqual(POINTS);
    expect(text("container")).toContain("height 260");
  });

  it("plots the score against time", () => {
    render(<ThreatChart points={POINTS} />);
    expect(text("line")).toBe("score #c8a84b");
    expect(text("x-axis")).toContain("time");
    expect(text("x-axis")).toContain(formatDate(Date.UTC(2026, 10, 17, 12)));
  });

  it("always uses the 0-100 scale of the risk bands", () => {
    render(<ThreatChart points={POINTS} />);
    expect(JSON.parse(text("y-axis"))).toEqual({ domain: [0, 100], ticks: [0, 25, 50, 75, 100] });
  });

  it("marks the edges of the bands at 25, 50 and 75", () => {
    render(<ThreatChart points={POINTS} />);
    const bands = screen.getAllByTestId("band").map((band) => band.getAttribute("data-content"));
    expect(bands).toEqual(["25", "50", "75"]);
  });

  it("explains a point in words", () => {
    render(<ThreatChart points={POINTS} />);
    const tooltip = JSON.parse(text("tooltip"));
    expect(tooltip.label).toBe(formatDateTime(Date.UTC(2026, 10, 17, 12)));
    expect(tooltip.value).toEqual(["42 / 100", "Threat score"]);
  });
});
