import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../lib/api";
import { resetAuthStore } from "../../lib/store";
import { aCase, aFront, aGameState, aRules, aThreat, anEmpire, aUser, renderWithClient } from "../../test-utils";
import type { GameState } from "../../lib/types";
import DashboardView from "../DashboardView";

const mocks = vi.hoisted(() => ({ replace: vi.fn(), me: vi.fn(), logout: vi.fn(), state: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
  usePathname: () => "/dashboard",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, api: { auth: { me: mocks.me, logout: mocks.logout }, game: { state: mocks.state } } };
});

beforeEach(() => {
  resetAuthStore();
  mocks.replace.mockReset();
  mocks.logout.mockReset();
  mocks.me.mockReset().mockResolvedValue(aUser());
  mocks.state.mockReset().mockResolvedValue(aGameState());
});

/** The tile with this label. */
const tile = (label: string) => screen.getByText(label).closest(".stat") as HTMLElement;

async function showDashboard(state: GameState = aGameState()) {
  mocks.state.mockResolvedValue(state);
  const rendered = renderWithClient(<DashboardView />);
  await screen.findByText(/Active empire:/);
  return rendered;
}

describe("DashboardView", () => {
  it("asks for nothing before the player is known to be signed in", async () => {
    mocks.me.mockRejectedValue(new ApiError("authentication required", 401));
    renderWithClient(<DashboardView />);

    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login?next=%2Fdashboard"));
    expect(mocks.state).not.toHaveBeenCalled();
    expect(screen.queryByText("Command center")).not.toBeInTheDocument();
  });

  it("shows placeholders while the empire loads", async () => {
    mocks.state.mockReturnValue(new Promise(() => {}));
    renderWithClient(<DashboardView />);
    expect(await screen.findByRole("status", { name: "Loading your empire" })).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("heading", { level: 1, name: "Command center" })).toBeInTheDocument();
  });

  it("shows the empire's figures, formatted", async () => {
    await showDashboard(
      aGameState({
        empire: anEmpire({
          name: "Heisenberg's Empire", dirty_money: 93750.4, clean_money: 45000, heat_level: 62.5, purity_rating: 61.34,
          batch_yield: 62.5, territory_count: 2, front_businesses: [aFront({ id: 1 }), aFront({ id: 2, is_active: false })],
        }),
      }),
    );

    expect(screen.getByText("Active empire: Heisenberg's Empire")).toBeInTheDocument();
    expect(tile("Dirty money")).toHaveTextContent("$93,750");
    expect(tile("Clean money")).toHaveTextContent("$45,000");
    expect(tile("Heat")).toHaveTextContent("62.5");
    expect(tile("Heat")).toHaveTextContent("cools by 6 per hour");
    expect(tile("Purity")).toHaveTextContent("61.3%");
    expect(tile("Last batch")).toHaveTextContent("62.5 kg");
    expect(tile("Territories")).toHaveTextContent("2 / 10");
    expect(tile("Front businesses")).toHaveTextContent("1 / 5");
  });

  it("colours the heat tile by how dangerous the heat is", async () => {
    for (const [heat, tone] of [[10, "good"], [30, "gold"], [60, "danger"], [90, "danger"]] as const) {
      const { unmount } = await showDashboard(aGameState({ empire: anEmpire({ heat_level: heat }) }));
      expect(tile("Heat"), `heat ${heat}`).toHaveAttribute("data-tone", tone);
      unmount();
      resetAuthStore();
    }
  });

  it("shows the DEA's assessment", async () => {
    await showDashboard(
      aGameState({
        dea_threat: aThreat({
          threat_score: 61.51, risk_level: "high", evidence_count: 1, source: "heuristic",
          prediction: "A case is being built. Lawyer up.", recommendation: "Shut down one territory. Consult Saul immediately.",
        }),
      }),
    );

    const panel = screen.getByRole("region", { name: "DEA threat" });
    expect(panel).toHaveTextContent("Lead agent: Hank Schrader");
    expect(within(panel).getByText("61.5")).toBeInTheDocument();
    expect(within(panel).getByText("High")).toHaveAttribute("data-risk", "high");
    expect(within(panel).getByRole("meter", { name: "DEA threat score" })).toHaveAttribute("aria-valuenow", "61.51");
    expect(panel).toHaveTextContent("A case is being built. Lawyer up.");
    expect(panel).toHaveTextContent("Consult Saul immediately.");
    expect(panel).toHaveTextContent("1 piece of evidence");
    expect(panel).toHaveTextContent("built-in estimate");
  });

  it("says where the assessment came from and pluralises the evidence", async () => {
    await showDashboard(aGameState({ dea_threat: aThreat({ source: "ml", evidence_count: 3 }) }));
    const panel = screen.getByRole("region", { name: "DEA threat" });
    expect(panel).toHaveTextContent("3 pieces of evidence");
    expect(panel).toHaveTextContent("machine-learning assessment");
  });

  it("keeps the threat meter in range even if the score is not", async () => {
    await showDashboard(aGameState({ dea_threat: aThreat({ threat_score: 140, risk_level: "critical" }) }));
    expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "100");
  });

  it("shows an unknown risk level as medium instead of crashing", async () => {
    await showDashboard(aGameState({ dea_threat: aThreat({ risk_level: "elevated" as never }) }));
    expect(within(screen.getByRole("region", { name: "DEA threat" })).getByText("Medium")).toBeInTheDocument();
  });

  describe("expansion", () => {
    it("tells how much more is needed, using the server's price", async () => {
      await showDashboard(aGameState({ empire: anEmpire({ clean_money: 45000 }), rules: aRules({ next_territory_cost: 100000 }) }));
      const panel = screen.getByRole("region", { name: "Expansion" });
      expect(panel).toHaveTextContent("The next territory costs $100,000");
      expect(panel).toHaveTextContent("You need $55,000 more clean money.");
    });

    it("says when the player can afford it", async () => {
      await showDashboard(aGameState({ empire: anEmpire({ clean_money: 30000 }), rules: aRules({ next_territory_cost: 25000 }) }));
      expect(screen.getByRole("region", { name: "Expansion" })).toHaveTextContent("You can afford it.");
    });

    it("says when there is nothing left to buy", async () => {
      await showDashboard(aGameState({ empire: anEmpire({ territory_count: 10 }), rules: aRules({ next_territory_cost: null }) }));
      const panel = screen.getByRole("region", { name: "Expansion" });
      expect(panel).toHaveTextContent("You hold the maximum of 10 territories.");
      expect(panel).not.toHaveTextContent("costs");
    });

    it("links to the pages where the player acts", async () => {
      await showDashboard();
      expect(screen.getByRole("link", { name: /cook a batch/i })).toHaveAttribute("href", "/dashboard/empire");
      expect(screen.getByRole("link", { name: /launder money/i })).toHaveAttribute("href", "/dashboard/launder");
      expect(screen.getByRole("link", { name: /open the dea file/i })).toHaveAttribute("href", "/dashboard/dea");
      expect(screen.getByRole("link", { name: /better call saul/i })).toHaveAttribute("href", "/dashboard/legal");
    });
  });

  describe("legal trouble", () => {
    it("is calm when there are no cases", async () => {
      await showDashboard();
      expect(screen.getByRole("region", { name: "Legal trouble" })).toHaveTextContent("No cases. Keep it that way.");
    });

    it("lists the latest cases with their risk and status", async () => {
      await showDashboard(
        aGameState({
          legal_cases: [
            aCase({ id: 2, charge: "Money laundering", risk: "critical", status: "open" }),
            aCase({ id: 1, charge: "Possession with intent", risk: "low", status: "dismissed", created_at: new Date(Date.now() - 2 * 86_400_000).toISOString() }),
          ],
        }),
      );
      const items = within(screen.getByRole("region", { name: "Legal trouble" })).getAllByRole("listitem");
      expect(items).toHaveLength(2);
      expect(items[0]).toHaveTextContent("Money laundering");
      expect(within(items[0]).getByText("Critical")).toHaveAttribute("data-risk", "critical");
      expect(items[0]).toHaveTextContent("open");
      expect(items[0]).toHaveTextContent("1 h ago");
      expect(items[1]).toHaveTextContent("Possession with intent");
      expect(items[1]).toHaveTextContent("dismissed");
      expect(items[1]).toHaveTextContent("2 d ago");
    });
  });

  describe("how it works", () => {
    it("explains the rules with the numbers the server sent, not fixed ones", async () => {
      await showDashboard(aGameState({ rules: aRules({ launder_rate: 0.6, large_transfer: 20000, max_active_fronts: 3, next_territory_cost: null }) }));
      const panel = screen.getByRole("region", { name: "How it works" });
      expect(panel).toHaveTextContent("60% of what you send into clean money");
      expect(panel).toHaveTextContent("above $20,000 is flagged");
      expect(panel).toHaveTextContent("up to 3 front businesses");
      expect(panel).not.toHaveTextContent("75%");
      expect(panel).not.toHaveTextContent("$50,000");
    });
  });

  describe("when things go wrong", () => {
    it("explains it and offers a retry, which works", async () => {
      mocks.state.mockRejectedValueOnce(new ApiError("The server had a problem. Please try again.", 500));
      renderWithClient(<DashboardView />);

      const notice = await screen.findByRole("alert");
      expect(notice).toHaveTextContent("Cannot load your empire");
      expect(notice).toHaveTextContent("The server had a problem.");

      mocks.state.mockResolvedValue(aGameState());
      await userEvent.click(screen.getByRole("button", { name: /try again/i }));
      expect(await screen.findByText(/Active empire:/)).toBeInTheDocument();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("keeps showing the last numbers when a refresh fails", async () => {
      const { client } = await showDashboard(aGameState({ empire: anEmpire({ dirty_money: 12345 }) }));
      mocks.state.mockRejectedValue(new ApiError("Cannot reach the server. Check your connection and try again.", 0));

      await act(async () => {
        await client.invalidateQueries({ queryKey: ["game", "state"] });
      });

      const notice = await screen.findByRole("alert");
      expect(notice).toHaveTextContent("Could not refresh - showing the last known numbers");
      expect(tile("Dirty money")).toHaveTextContent("$12,345");
    });
  });
});
