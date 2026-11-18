import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../../lib/api";
import { formatDateTime } from "../../../lib/format";
import { resetAuthStore } from "../../../lib/store";
import type { EvidenceFile } from "../../../lib/types";
import { aGameState, aThreat, aUser, renderWithClient } from "../../../test-utils";
import DeaView from "../DeaView";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  me: vi.fn(),
  logout: vi.fn(),
  state: vi.fn(),
  history: vi.fn(),
  evidence: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
  usePathname: () => "/dashboard/dea",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    api: {
      auth: { me: mocks.me, logout: mocks.logout },
      game: { state: mocks.state },
      dea: { history: mocks.history, evidence: mocks.evidence },
    },
  };
});

// The chart itself has its own test; here it only has to be given the right points.
vi.mock("../ThreatChart", async () => {
  const React = await import("react");
  return {
    default: ({ points }: { points: unknown[] }) => React.createElement("div", { "data-testid": "chart", "data-points": JSON.stringify(points) }),
  };
});

const NO_EVIDENCE: EvidenceFile = { source: "local", count: 0, total_weight: 0, top_sources: [] };
const day = (n: number) => `2026-11-${String(n).padStart(2, "0")}T10:00:00Z`;
const chartPoints = () => JSON.parse(screen.getByTestId("chart").getAttribute("data-points") ?? "[]") as { score: number }[];

beforeEach(() => {
  resetAuthStore();
  mocks.replace.mockReset();
  mocks.logout.mockReset();
  mocks.me.mockReset().mockResolvedValue(aUser());
  mocks.state.mockReset().mockResolvedValue(aGameState());
  mocks.history.mockReset().mockResolvedValue([]);
  mocks.evidence.mockReset().mockResolvedValue(NO_EVIDENCE);
});

async function showDea(threat = aThreat()) {
  mocks.state.mockResolvedValue(aGameState({ dea_threat: threat }));
  const rendered = renderWithClient(<DeaView />);
  await screen.findByText(/Lead agent:/);
  return rendered;
}

describe("DeaView", () => {
  it("asks for nothing before the player is known to be signed in", async () => {
    mocks.me.mockRejectedValue(new ApiError("authentication required", 401));
    renderWithClient(<DeaView />);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login?next=%2Fdashboard%2Fdea"));
    expect(mocks.state).not.toHaveBeenCalled();
    expect(mocks.history).not.toHaveBeenCalled();
    expect(mocks.evidence).not.toHaveBeenCalled();
  });

  describe("the threat", () => {
    it("shows the score, the risk, the agent and what the DEA is thinking", async () => {
      await showDea(
        aThreat({ threat_score: 62.4, risk_level: "high", evidence_count: 3, lead_agent: "Hank Schrader", prediction: "Hank is closing in.", recommendation: "Lay low for a while." }),
      );
      const tile = screen.getByText("Threat score").closest(".stat") as HTMLElement;
      expect(tile).toHaveTextContent("62.4 / 100");
      expect(tile).toHaveTextContent("High");
      expect(screen.getByText("Lead agent: Hank Schrader")).toBeInTheDocument();
      expect(screen.getByText("Hank is closing in.")).toBeInTheDocument();
      expect(screen.getByText("Lay low for a while.")).toBeInTheDocument();
      expect(screen.getByText("Evidence", { selector: ".stat__label" }).closest(".stat")).toHaveTextContent("3 pieces on file");
    });

    it("draws the gauge, and keeps it inside 0-100", async () => {
      await showDea(aThreat({ threat_score: 62.4, risk_level: "high" }));
      expect(screen.getByRole("meter", { name: "DEA threat score" })).toHaveAttribute("aria-valuenow", "62.4");
    });

    it("does not let an impossible score overflow the gauge", async () => {
      await showDea(aThreat({ threat_score: 140, risk_level: "critical" }));
      expect(screen.getByRole("meter", { name: "DEA threat score" })).toHaveAttribute("aria-valuenow", "100");
    });

    it("says where the assessment came from", async () => {
      await showDea(aThreat({ source: "ml" }));
      expect(screen.getByText("machine-learning assessment")).toBeInTheDocument();
    });

    it("says so when it is the built-in estimate", async () => {
      await showDea(aThreat({ source: "heuristic" }));
      expect(screen.getByText("built-in estimate")).toBeInTheDocument();
    });

    it("checks again on request, reloading the threat, the history and the evidence", async () => {
      await showDea();
      await waitFor(() => expect(mocks.history).toHaveBeenCalled());
      await waitFor(() => expect(mocks.evidence).toHaveBeenCalled());
      const before = { state: mocks.state.mock.calls.length, history: mocks.history.mock.calls.length, evidence: mocks.evidence.mock.calls.length };

      await userEvent.click(screen.getByRole("button", { name: /Check again/ }));

      await waitFor(() => expect(mocks.state.mock.calls.length).toBeGreaterThan(before.state));
      await waitFor(() => expect(mocks.history.mock.calls.length).toBeGreaterThan(before.history));
      await waitFor(() => expect(mocks.evidence.mock.calls.length).toBeGreaterThan(before.evidence));
      expect(await screen.findByRole("button", { name: /Check again/ })).toBeEnabled();
    });
  });

  describe("the history", () => {
    it("asks for the latest 60 entries", async () => {
      await showDea();
      await waitFor(() => expect(mocks.history).toHaveBeenCalledWith(60));
    });

    it("charts the entries oldest first, although the server sends them newest first", async () => {
      mocks.history.mockResolvedValue([
        aThreat({ id: 3, created_at: day(12), threat_score: 60 }),
        aThreat({ id: 2, created_at: day(8), threat_score: 30 }),
        aThreat({ id: 1, created_at: day(2), threat_score: 5 }),
      ]);
      await showDea();
      await screen.findByTestId("chart");
      expect(chartPoints().map((item) => item.score)).toEqual([5, 30, 60]);
    });

    it("describes the chart for screen readers", async () => {
      mocks.history.mockResolvedValue([aThreat({ created_at: day(8), threat_score: 30 }), aThreat({ id: 2, created_at: day(2), threat_score: 5 })]);
      await showDea();
      const chart = await screen.findByRole("img");
      expect(chart).toHaveAccessibleName("Chart of the DEA threat score over time. 2 entries, latest 30.0 out of 100.");
      expect(within(chart).getByTestId("chart")).toBeInTheDocument();
    });

    it("offers the same data as a table, newest first", async () => {
      mocks.history.mockResolvedValue([
        aThreat({ id: 2, created_at: day(12), threat_score: 60, evidence_count: 4 }),
        aThreat({ id: 1, created_at: day(2), threat_score: 5, evidence_count: 0 }),
      ]);
      await showDea();
      await screen.findByTestId("chart");
      expect(screen.getByText("Show the same data as a table")).toBeInTheDocument();

      const rows = within(screen.getByRole("table", { name: "DEA threat history, newest first", hidden: true })).getAllByRole("row", { hidden: true }).slice(1);
      expect(rows).toHaveLength(2);
      expect(rows[0]).toHaveTextContent(formatDateTime(Date.parse(day(12))));
      expect(rows[0]).toHaveTextContent("60.0");
      expect(rows[0]).toHaveTextContent("High");
      expect(rows[0]).toHaveTextContent("4");
      expect(rows[1]).toHaveTextContent("5.0");
      expect(rows[1]).toHaveTextContent("Low");
    });

    it.each([
      [[10, 25], "Up 15 points since the previous assessment."],
      [[40, 12.5], "Down 27.5 points since the previous assessment."],
      [[40, 40], "Unchanged since the previous assessment."],
    ])("describes how the score moved: %j", async (scores, sentence) => {
      mocks.history.mockResolvedValue([
        aThreat({ id: 2, created_at: day(9), threat_score: scores[1] }),
        aThreat({ id: 1, created_at: day(3), threat_score: scores[0] }),
      ]);
      await showDea();
      expect(await screen.findByText(sentence)).toBeInTheDocument();
    });

    it("has no trend to describe with a single entry", async () => {
      mocks.history.mockResolvedValue([aThreat({ created_at: day(3), threat_score: 20 })]);
      await showDea();
      await screen.findByTestId("chart");
      expect(screen.queryByText(/since the previous assessment/)).not.toBeInTheDocument();
    });

    it("has a message instead of a chart when there is no history", async () => {
      await showDea();
      expect(await screen.findByText(/No history yet/)).toBeInTheDocument();
      expect(screen.queryByTestId("chart")).not.toBeInTheDocument();
    });

    it("shows that it is loading", async () => {
      mocks.history.mockReturnValue(new Promise(() => {}));
      await showDea();
      expect(await screen.findByText(/Loading the paper trail/)).toBeInTheDocument();
    });

    it("explains when the history cannot be loaded, and recovers", async () => {
      mocks.history.mockRejectedValueOnce(new ApiError("history is down", 500)).mockResolvedValue([aThreat({ created_at: day(3), threat_score: 20 })]);
      await showDea();
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("Cannot load the DEA's history");
      expect(alert).toHaveTextContent("history is down");
      await userEvent.click(within(alert).getByRole("button", { name: /Try again/ }));
      expect(await screen.findByTestId("chart")).toBeInTheDocument();
    });
  });

  describe("the evidence file", () => {
    it("says so when there is nothing", async () => {
      await showDea();
      expect(await screen.findByText("The DEA has nothing on you. Yet.")).toBeInTheDocument();
      expect(screen.queryByRole("table", { name: "The heaviest pieces of evidence" })).not.toBeInTheDocument();
    });

    it("sums up the file and lists the heaviest sources", async () => {
      mocks.evidence.mockResolvedValue({
        source: "ml",
        count: 2,
        total_weight: 7.5,
        top_sources: [
          { source: "Transfer #12", kind: "large_transfer", weight: 5 },
          { source: "Front: Vamonos Pest", kind: "front_raid", weight: 2.5 },
        ],
      });
      await showDea();
      const summary = await screen.findByText(/of evidence, total weight/);
      expect(summary).toHaveTextContent("2 pieces of evidence, total weight 7.5.");
      expect(summary).toHaveTextContent("Machine-learning assessment.");

      const rows = within(screen.getByRole("table", { name: "The heaviest pieces of evidence" })).getAllByRole("row").slice(1);
      expect(rows).toHaveLength(2);
      expect(rows[0]).toHaveTextContent("Transfer #12");
      expect(rows[0]).toHaveTextContent("large transfer");
      expect(within(rows[0]).getAllByRole("cell")[2]).toHaveTextContent(/^5$/);
      expect(rows[1]).toHaveTextContent("front raid");
      expect(within(rows[1]).getAllByRole("cell")[2]).toHaveTextContent(/^2\.5$/);
    });

    it("uses the singular for one piece and says when it is an estimate", async () => {
      mocks.evidence.mockResolvedValue({ source: "local", count: 1, total_weight: 1, top_sources: [] });
      await showDea();
      const summary = await screen.findByText(/of evidence, total weight/);
      expect(summary).toHaveTextContent("1 piece of evidence");
      expect(summary).toHaveTextContent("Built-in estimate.");
      expect(screen.queryByRole("table", { name: "The heaviest pieces of evidence" })).not.toBeInTheDocument();
    });

    it("explains when the file cannot be opened, and recovers", async () => {
      mocks.evidence.mockRejectedValueOnce(new ApiError("evidence is down", 500)).mockResolvedValue(NO_EVIDENCE);
      await showDea();
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("Cannot load the evidence file");
      await userEvent.click(within(alert).getByRole("button", { name: /Try again/ }));
      expect(await screen.findByText("The DEA has nothing on you. Yet.")).toBeInTheDocument();
    });
  });
});
