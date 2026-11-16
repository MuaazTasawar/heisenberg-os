import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../../lib/api";
import { resetAuthStore } from "../../../lib/store";
import type { CookAdvice, CookResult, GameState } from "../../../lib/types";
import { aGameState, aRules, anEmpire, aUser, renderWithClient } from "../../../test-utils";
import EmpireView from "../EmpireView";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  me: vi.fn(),
  logout: vi.fn(),
  state: vi.fn(),
  advice: vi.fn(),
  cook: vi.fn(),
  claim: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
  usePathname: () => "/dashboard/empire",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    api: {
      auth: { me: mocks.me, logout: mocks.logout },
      game: { state: mocks.state },
      empire: { advice: mocks.advice, cook: mocks.cook, claimTerritory: mocks.claim },
    },
  };
});

const anAdvice = (overrides: Partial<CookAdvice> = {}): CookAdvice => ({
  source: "local", effort: 8.33, predicted_yield: 100, optimal_effort: 8.33, optimal_yield: 100, purity_projection: 90, efficiency_score: 100, ...overrides,
});

const aCookResult = (overrides: Partial<CookResult> = {}): CookResult => ({
  empire: anEmpire(), effort: 8.33, yield_gained: 100, dirty_gained: 150000, heat_added: 20.8, purity_delta: 1.5, ...overrides,
});

beforeEach(() => {
  resetAuthStore();
  mocks.replace.mockReset();
  mocks.logout.mockReset();
  mocks.me.mockReset().mockResolvedValue(aUser());
  mocks.state.mockReset().mockResolvedValue(aGameState());
  mocks.advice.mockReset().mockResolvedValue(anAdvice());
  mocks.cook.mockReset().mockResolvedValue(aCookResult());
  mocks.claim.mockReset().mockResolvedValue(anEmpire({ territory_count: 1 }));
});

async function showEmpire(state: GameState = aGameState()) {
  mocks.state.mockResolvedValue(state);
  const rendered = renderWithClient(<EmpireView />);
  await screen.findByText(/Active empire:/);
  return rendered;
}

const slider = () => screen.getByLabelText("Cooking effort") as HTMLInputElement;
const tile = (label: string) => screen.getByText(label).closest(".stat") as HTMLElement;
const adviceEfforts = () => mocks.advice.mock.calls.map((call) => call[0]);

describe("EmpireView", () => {
  it("asks for nothing before the player is known to be signed in", async () => {
    mocks.me.mockRejectedValue(new ApiError("authentication required", 401));
    renderWithClient(<EmpireView />);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login?next=%2Fdashboard%2Fempire"));
    expect(mocks.state).not.toHaveBeenCalled();
    expect(mocks.advice).not.toHaveBeenCalled();
  });

  it("shows the empire's figures", async () => {
    await showEmpire(
      aGameState({ empire: anEmpire({ name: "Casa Tranquila", dirty_money: 1500, clean_money: 2500, heat_level: 12.34, purity_rating: 96.5, batch_yield: 55, territory_count: 3 }) }),
    );
    expect(screen.getByText("Active empire: Casa Tranquila")).toBeInTheDocument();
    expect(tile("Dirty money")).toHaveTextContent("$1,500");
    expect(tile("Clean money")).toHaveTextContent("$2,500");
    expect(tile("Heat")).toHaveTextContent("12.3");
    expect(tile("Territories")).toHaveTextContent("3 / 10");
  });

  it("builds the slider from the server's rules and starts at the sweet spot", async () => {
    await showEmpire(aGameState({ rules: aRules({ min_effort: 2, max_effort: 9, optimal_effort: 7.5 }) }));
    expect(slider()).toHaveAttribute("min", "2");
    expect(slider()).toHaveAttribute("max", "9");
    expect(slider().value).toBe("7.5");
    expect(screen.getByRole("button", { name: /Use the sweet spot \(7.5\)/ })).toBeInTheDocument();
  });

  it("asks for advice once the slider has settled, not for every change", async () => {
    await showEmpire();
    await waitFor(() => expect(mocks.advice).toHaveBeenCalledWith(8.33));

    for (const value of ["3", "4", "5"]) fireEvent.change(slider(), { target: { value } });
    expect(slider().value).toBe("5");

    await waitFor(() => expect(adviceEfforts()).toEqual([8.33, 5]));
    await new Promise((resolve) => setTimeout(resolve, 450));
    expect(adviceEfforts()).toEqual([8.33, 5]);
  });

  it("shows the advice", async () => {
    mocks.advice.mockResolvedValue(anAdvice({ predicted_yield: 80, optimal_yield: 100, optimal_effort: 8.33, efficiency_score: 80, purity_projection: 91.2, source: "ml" }));
    await showEmpire();
    expect(await screen.findByText(/Expected yield/)).toHaveTextContent("80 kg");
    expect(screen.getByText(/Expected yield/)).toHaveTextContent("$120,000");
    expect(screen.getByText(/of the best batch/)).toHaveTextContent("80%");
    expect(screen.getByText(/of the best batch/)).toHaveTextContent("91.2%");
    expect(screen.getByText("Machine-learning advice")).toBeInTheDocument();
  });

  it("projects the heat, capped at 100", async () => {
    await showEmpire(aGameState({ empire: anEmpire({ heat_level: 20 }) }));
    // 20 + 8.33 * 2.5 = 40.825
    expect(screen.getByText(/Heat 20 /).querySelector("strong")).toHaveTextContent(/^40\.8$/);
    expect(screen.queryByText(/heat stops at 100/)).not.toBeInTheDocument();

    fireEvent.change(slider(), { target: { value: "10" } });
    expect(screen.getByText(/Heat 20 /).querySelector("strong")).toHaveTextContent(/^45$/);
  });

  it("stops the projected heat at 100", async () => {
    await showEmpire(aGameState({ empire: anEmpire({ heat_level: 90 }) }));
    expect(screen.getByText(/Heat 90 /).querySelector("strong")).toHaveTextContent(/^100$/);
    expect(screen.getByText(/heat stops at 100/)).toBeInTheDocument();
  });

  it("goes back to the sweet spot on request", async () => {
    await showEmpire();
    fireEvent.change(slider(), { target: { value: "2" } });
    await userEvent.click(screen.getByRole("button", { name: /Use the sweet spot/ }));
    expect(slider().value).toBe("8.33");
  });

  it("cooks with the chosen effort, shows the result and reloads the game", async () => {
    await showEmpire();
    fireEvent.change(slider(), { target: { value: "6.5" } });
    const loadsBefore = mocks.state.mock.calls.length;

    await userEvent.click(screen.getByRole("button", { name: /Cook batch/ }));

    expect(mocks.cook).toHaveBeenCalledTimes(1);
    expect(mocks.cook).toHaveBeenCalledWith(6.5);
    const result = await screen.findByText(/Batch cooked/);
    expect(result).toHaveTextContent("+100 kg");
    expect(result).toHaveTextContent("+$150,000");
    expect(result).toHaveTextContent("+20.8 heat");
    expect(result).toHaveTextContent("purity +1.5");
    await waitFor(() => expect(mocks.state.mock.calls.length).toBeGreaterThan(loadsBefore));
  });

  it("shows a loss of purity without a plus sign", async () => {
    mocks.cook.mockResolvedValue(aCookResult({ purity_delta: -0.75 }));
    await showEmpire();
    await userEvent.click(screen.getByRole("button", { name: /Cook batch/ }));
    expect(await screen.findByText(/Batch cooked/)).toHaveTextContent("purity -0.75");
  });

  it("disables the form while cooking", async () => {
    mocks.cook.mockReturnValue(new Promise(() => {}));
    await showEmpire();
    await userEvent.click(screen.getByRole("button", { name: /Cook batch/ }));
    expect(await screen.findByRole("button", { name: /Cooking/ })).toBeDisabled();
    expect(slider()).toBeDisabled();
  });

  it("shows why a batch failed and lets the player try again", async () => {
    mocks.cook.mockRejectedValueOnce(new ApiError("effort must be between 1 and 10", 400));
    await showEmpire();
    await userEvent.click(screen.getByRole("button", { name: /Cook batch/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent("effort must be between 1 and 10");
    expect(screen.getByRole("alert")).toHaveTextContent("The batch failed");
    expect(screen.getByRole("button", { name: /Cook batch/ })).toBeEnabled();
    expect(screen.queryByText(/Batch cooked/)).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Cook batch/ }));
    expect(await screen.findByText(/Batch cooked/)).toBeInTheDocument();
    expect(mocks.cook).toHaveBeenCalledTimes(2);
  });

  it("does not block cooking when the advice fails", async () => {
    mocks.advice.mockRejectedValue(new ApiError("advice is down", 500));
    await showEmpire();
    expect(await screen.findByText(/Advice is unavailable right now/)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Cook batch/ })).toBeEnabled();
  });

  describe("territory", () => {
    it("offers a territory the player can afford", async () => {
      await showEmpire(aGameState({ empire: anEmpire({ clean_money: 30000 }) }));
      expect(screen.getByText(/The next territory costs/)).toHaveTextContent("$25,000");
      expect(screen.queryByText(/more clean money/)).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Claim for $25,000" })).toBeEnabled();
    });

    it("says how much is missing when the player cannot afford it", async () => {
      await showEmpire(aGameState({ empire: anEmpire({ clean_money: 10000 }) }));
      expect(screen.getByText(/more clean money/)).toHaveTextContent("You need $15,000 more clean money.");
      expect(screen.getByRole("button", { name: "Claim for $25,000" })).toBeDisabled();
    });

    it("has nothing to claim at the maximum", async () => {
      await showEmpire(aGameState({ empire: anEmpire({ territory_count: 10 }), rules: aRules({ next_territory_cost: null }) }));
      expect(screen.getByText(/You hold the maximum of 10 territories/)).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /Claim for/ })).not.toBeInTheDocument();
    });

    it("claims it and reports what the server returned", async () => {
      mocks.claim.mockResolvedValue(anEmpire({ territory_count: 4 }));
      await showEmpire(aGameState({ empire: anEmpire({ clean_money: 30000, territory_count: 3 }) }));
      const loadsBefore = mocks.state.mock.calls.length;

      await userEvent.click(screen.getByRole("button", { name: "Claim for $25,000" }));

      expect(mocks.claim).toHaveBeenCalledTimes(1);
      expect(await screen.findByText(/Territory claimed/)).toHaveTextContent("You now hold 4; your yield bonus is +20%");
      await waitFor(() => expect(mocks.state.mock.calls.length).toBeGreaterThan(loadsBefore));
    });

    it("shows why a claim was refused", async () => {
      mocks.claim.mockRejectedValue(new ApiError("not enough clean money", 400));
      await showEmpire(aGameState({ empire: anEmpire({ clean_money: 30000 }) }));
      await userEvent.click(screen.getByRole("button", { name: "Claim for $25,000" }));
      expect(await screen.findByRole("alert")).toHaveTextContent("not enough clean money");
      expect(screen.getByRole("alert")).toHaveTextContent("Could not claim the territory");
      expect(screen.getByRole("button", { name: "Claim for $25,000" })).toBeEnabled();
    });
  });

  it("links to the fronts page", async () => {
    await showEmpire();
    expect(screen.getByRole("link", { name: /Manage front businesses/ })).toHaveAttribute("href", "/dashboard/fronts");
  });
});
