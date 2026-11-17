import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../../lib/api";
import { resetAuthStore } from "../../../lib/store";
import type { GameState, LaunderResult, Page, Transaction } from "../../../lib/types";
import { aFront, aGameState, aRules, anEmpire, aUser, renderWithClient } from "../../../test-utils";
import LaunderView from "../LaunderView";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  me: vi.fn(),
  logout: vi.fn(),
  state: vi.fn(),
  send: vi.fn(),
  transactions: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
  usePathname: () => "/dashboard/launder",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    api: {
      auth: { me: mocks.me, logout: mocks.logout },
      game: { state: mocks.state },
      launder: { send: mocks.send, transactions: mocks.transactions },
    },
  };
});

const aTransaction = (overrides: Partial<Transaction> = {}): Transaction => ({
  id: 1,
  created_at: "2026-11-17T10:00:00Z",
  updated_at: "2026-11-17T10:00:00Z",
  empire_id: 1,
  amount: 5000,
  type: "laundered",
  description: "Laundered through Los Pollos Hermanos",
  business_id: 1,
  flagged: false,
  risk_score: 12,
  ...overrides,
});

const aPage = (items: Transaction[], overrides: Partial<Page<Transaction>> = {}): Page<Transaction> => ({
  items,
  total: items.length,
  limit: 10,
  offset: 0,
  ...overrides,
});

const aLaunderResult = (overrides: Partial<LaunderResult> = {}): LaunderResult => ({
  transaction: aTransaction(),
  cleaned_amount: 3750,
  fee: 1250,
  suspicion_gain: 2.5,
  front_closed: false,
  front: aFront({ id: 1, name: "Los Pollos Hermanos", suspicion: 12 }),
  empire: anEmpire(),
  analysis: { source: "local", risk_score: 20, anomaly: false, patterns: [], recommendation: "Looks routine." },
  ...overrides,
});

const richState = (): GameState =>
  aGameState({
    empire: anEmpire({
      dirty_money: 10000,
      front_businesses: [
        aFront({ id: 1, name: "Los Pollos Hermanos", suspicion: 12 }),
        aFront({ id: 2, name: "Vamonos Pest", suspicion: 30 }),
        aFront({ id: 3, name: "Old Shop", is_active: false }),
      ],
    }),
  });

beforeEach(() => {
  resetAuthStore();
  mocks.replace.mockReset();
  mocks.logout.mockReset();
  mocks.me.mockReset().mockResolvedValue(aUser());
  mocks.state.mockReset().mockResolvedValue(richState());
  mocks.send.mockReset().mockResolvedValue(aLaunderResult());
  mocks.transactions.mockReset().mockResolvedValue(aPage([]));
});

async function showLaunder(state: GameState = richState()) {
  mocks.state.mockResolvedValue(state);
  const rendered = renderWithClient(<LaunderView />);
  await screen.findByText(/Active empire:/);
  return rendered;
}

const amountField = () => screen.getByLabelText("Amount") as HTMLInputElement;
const frontField = () => screen.getByLabelText("Front business") as HTMLSelectElement;
const launderButton = () => screen.getByRole("button", { name: /Launder|Sending/ });
const type = (value: string) => fireEvent.change(amountField(), { target: { value } });

describe("LaunderView", () => {
  it("asks for nothing before the player is known to be signed in", async () => {
    mocks.me.mockRejectedValue(new ApiError("authentication required", 401));
    renderWithClient(<LaunderView />);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login?next=%2Fdashboard%2Flaunder"));
    expect(mocks.state).not.toHaveBeenCalled();
    expect(mocks.transactions).not.toHaveBeenCalled();
  });

  it("sends the player to open a front when there is none", async () => {
    await showLaunder(aGameState({ empire: anEmpire({ front_businesses: [aFront({ is_active: false })] }) }));
    expect(screen.getByText(/You need an open front business/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open one" })).toHaveAttribute("href", "/dashboard/fronts");
    expect(screen.queryByLabelText("Amount")).not.toBeInTheDocument();
  });

  it("offers only the open fronts, with their suspicion", async () => {
    await showLaunder();
    expect(within(frontField()).getByRole("option", { name: "Los Pollos Hermanos (suspicion 12)" })).toBeInTheDocument();
    expect(within(frontField()).getByRole("option", { name: "Vamonos Pest (suspicion 30)" })).toBeInTheDocument();
    expect(within(frontField()).queryByRole("option", { name: /Old Shop/ })).not.toBeInTheDocument();
  });

  describe("preview", () => {
    it("shows what the player keeps, from the rate in the rules", async () => {
      await showLaunder();
      type("5000");
      expect(screen.getByText(/Sending/)).toHaveTextContent("Sending $5,000.00 gives you $3,750.00 clean; the fee is $1,250.00.");
    });

    it("follows a different rate", async () => {
      await showLaunder({ ...richState(), rules: aRules({ launder_rate: 0.5 }) });
      type("1000");
      expect(screen.getByText(/Sending/)).toHaveTextContent("gives you $500.00 clean; the fee is $500.00.");
      expect(screen.getByText(/You keep 50%/)).toBeInTheDocument();
    });

    it("understands dollars and commas, and says nothing for text that is not an amount", async () => {
      await showLaunder();
      type("$1,234.50");
      expect(screen.getByText(/Sending/)).toHaveTextContent("Sending $1,234.50");
      type("lots");
      expect(screen.queryByText(/Sending/)).not.toBeInTheDocument();
    });

    it("warns above the large-transfer limit and not at it", async () => {
      await showLaunder(aGameState({ empire: richState().empire, rules: aRules({ large_transfer: 50000 }) }));
      type("50000");
      expect(screen.queryByText(/flagged as evidence/)).not.toBeInTheDocument();
      type("50000.01");
      expect(screen.getByText(/flagged as evidence/)).toHaveTextContent("above $50,000");
    });
  });

  describe("use all", () => {
    it("fills in all the dirty money", async () => {
      await showLaunder();
      await userEvent.click(screen.getByRole("button", { name: "Use all" }));
      expect(amountField().value).toBe("10000");
    });

    it("rounds down to whole cents so the server accepts it", async () => {
      const state = richState();
      state.empire.dirty_money = 1234.5678;
      await showLaunder(state);
      await userEvent.click(screen.getByRole("button", { name: "Use all" }));
      expect(amountField().value).toBe("1234.56");
    });

    it("is disabled when there is less than the minimum transfer", async () => {
      const state = richState();
      state.empire.dirty_money = 50;
      await showLaunder(state);
      expect(screen.getByRole("button", { name: "Use all" })).toBeDisabled();
    });
  });

  describe("validation", () => {
    it("asks for a front and an amount before calling the server", async () => {
      await showLaunder();
      await userEvent.click(launderButton());
      expect(await screen.findByText("Choose which front business to use.")).toBeInTheDocument();
      expect(screen.getByText("Enter an amount in dollars, for example 5000.")).toBeInTheDocument();
      expect(mocks.send).not.toHaveBeenCalled();
    });

    it("rejects an amount below the minimum or above the dirty money", async () => {
      await showLaunder();
      await userEvent.selectOptions(frontField(), "1");
      type("99");
      await userEvent.click(launderButton());
      expect(await screen.findByText("The minimum transfer is $100.")).toBeInTheDocument();

      type("10000.01");
      expect(screen.getByText("You only have $10,000 in dirty money.")).toBeInTheDocument();
      expect(amountField()).toHaveAttribute("aria-invalid", "true");
      expect(mocks.send).not.toHaveBeenCalled();
    });
  });

  describe("sending", () => {
    it("sends the amount to the chosen front and shows what came of it", async () => {
      mocks.send.mockResolvedValue(
        aLaunderResult({ analysis: { source: "ml", risk_score: 62, anomaly: true, patterns: ["round numbers", "rapid repeats"], recommendation: "Space your transfers out." } }),
      );
      await showLaunder();
      await userEvent.selectOptions(frontField(), "2");
      type("5,000");
      const loadsBefore = mocks.state.mock.calls.length;

      await userEvent.click(launderButton());

      expect(mocks.send).toHaveBeenCalledTimes(1);
      expect(mocks.send).toHaveBeenCalledWith({ amount: 5000, business_id: 2 });
      const summary = await screen.findByText(/Sent \$5,000\.00 through/);
      expect(summary).toHaveTextContent("$3,750.00 clean, fee $1,250.00");
      expect(summary).toHaveTextContent("Suspicion +2.5 (now 12 / 100)");
      expect(screen.getByText(/Risk 62 \/ 100/)).toHaveTextContent("High");
      expect(screen.getByText(/unusual pattern/)).toBeInTheDocument();
      expect(screen.getByText("round numbers")).toBeInTheDocument();
      expect(screen.getByText("rapid repeats")).toBeInTheDocument();
      expect(screen.getByText("Space your transfers out.")).toBeInTheDocument();
      expect(screen.getByText("Machine-learning analysis")).toBeInTheDocument();
      expect(amountField().value).toBe("");
      await waitFor(() => expect(mocks.state.mock.calls.length).toBeGreaterThan(loadsBefore));
    });

    it("says when the transfer became evidence", async () => {
      mocks.send.mockResolvedValue(aLaunderResult({ transaction: aTransaction({ flagged: true, flag_reason: "single transfer over $50,000" }) }));
      await showLaunder();
      await userEvent.selectOptions(frontField(), "1");
      type("5000");
      await userEvent.click(launderButton());
      expect(await screen.findByText(/This transfer became evidence/)).toHaveTextContent("single transfer over $50,000");
    });

    it("announces a raid and no longer offers the closed front", async () => {
      mocks.send.mockResolvedValue(
        aLaunderResult({ front_closed: true, front: aFront({ id: 1, name: "Los Pollos Hermanos", is_active: false, suspicion: 100, shutdown_reason: "raided by the DEA: suspicion reached 100" }) }),
      );
      await showLaunder();
      await userEvent.selectOptions(frontField(), "1");
      type("5000");
      await userEvent.click(launderButton());

      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("Los Pollos Hermanos was raided");
      expect(alert).toHaveTextContent("raided by the DEA: suspicion reached 100");
      expect(screen.getByText(/Sent \$5,000\.00 through/)).not.toHaveTextContent("now");
      expect(frontField().value).toBe("");
    });

    it("shows why a transfer failed and keeps what was typed", async () => {
      mocks.send.mockRejectedValue(new ApiError("not enough dirty money", 400));
      await showLaunder();
      await userEvent.selectOptions(frontField(), "1");
      type("5000");
      await userEvent.click(launderButton());

      expect(await screen.findByRole("alert")).toHaveTextContent("not enough dirty money");
      expect(screen.getByRole("alert")).toHaveTextContent("The transfer failed");
      expect(amountField().value).toBe("5000");
      expect(frontField().value).toBe("1");
      expect(launderButton()).toBeEnabled();
    });

    it("cannot be submitted twice while it works", async () => {
      mocks.send.mockReturnValue(new Promise(() => {}));
      await showLaunder();
      await userEvent.selectOptions(frontField(), "1");
      type("5000");
      await userEvent.click(launderButton());
      expect(await screen.findByRole("button", { name: /Sending/ })).toBeDisabled();
      expect(mocks.send).toHaveBeenCalledTimes(1);
    });
  });

  describe("ledger", () => {
    const rows = () => within(screen.getByRole("table")).getAllByRole("row").slice(1);

    it("asks for the first page and lists the transactions", async () => {
      mocks.transactions.mockResolvedValue(
        aPage([
          aTransaction({ id: 2, amount: 70000, flagged: true, flag_reason: "single transfer over $50,000", risk_score: 80 }),
          aTransaction({ id: 1, amount: 1234.5, type: "dirty", description: "Batch sold", risk_score: 0 }),
        ]),
      );
      await showLaunder();
      await screen.findByRole("table");
      expect(mocks.transactions).toHaveBeenCalledWith({ limit: 10, offset: 0 });

      expect(rows()).toHaveLength(2);
      expect(rows()[0]).toHaveTextContent("$70,000.00");
      expect(rows()[0]).toHaveTextContent("Flagged");
      expect(rows()[0]).toHaveTextContent("single transfer over $50,000");
      expect(rows()[0]).toHaveTextContent("80");
      expect(rows()[1]).toHaveTextContent("Dirty");
      expect(rows()[1]).toHaveTextContent("$1,234.50");
      expect(rows()[1]).not.toHaveTextContent("Flagged");
    });

    it("has a message for an empty ledger", async () => {
      await showLaunder();
      expect(await screen.findByText("No transactions yet.")).toBeInTheDocument();
      expect(screen.queryByRole("table")).not.toBeInTheDocument();
    });

    it("pages with the offsets the server expects", async () => {
      const first = Array.from({ length: 10 }, (_, index) => aTransaction({ id: 100 - index, description: `Row ${index}` }));
      mocks.transactions.mockImplementation(async (query: { offset?: number }) =>
        query.offset ? aPage([aTransaction({ id: 5, description: "Row 10" })], { total: 25, offset: query.offset }) : aPage(first, { total: 25 }),
      );
      await showLaunder();
      await screen.findByText("Row 0");
      expect(screen.getByText(/Showing 1.10 of 25/)).toBeInTheDocument();

      await userEvent.click(screen.getByRole("button", { name: "Next" }));
      expect(await screen.findByText("Row 10")).toBeInTheDocument();
      expect(mocks.transactions).toHaveBeenLastCalledWith({ limit: 10, offset: 10 });
      expect(screen.queryByText("Row 0")).not.toBeInTheDocument();

      await userEvent.click(screen.getByRole("button", { name: "Previous" }));
      await screen.findByText("Row 0");
      expect(mocks.transactions).toHaveBeenLastCalledWith({ limit: 10, offset: 0 });
    });

    it("filters to flagged transfers and starts again from the first page", async () => {
      mocks.transactions.mockImplementation(async (query: { offset?: number; flagged?: boolean }) =>
        query.flagged
          ? aPage([aTransaction({ id: 7, description: "Flagged one", flagged: true })], { total: 1 })
          : aPage([aTransaction({ id: 9, description: "Row A" })], { total: 25, offset: query.offset ?? 0 }),
      );
      await showLaunder();
      await screen.findByText("Row A");
      await userEvent.click(screen.getByRole("button", { name: "Next" }));
      await waitFor(() => expect(mocks.transactions).toHaveBeenLastCalledWith({ limit: 10, offset: 10 }));

      await userEvent.click(screen.getByRole("checkbox", { name: "Flagged only" }));
      expect(await screen.findByText("Flagged one")).toBeInTheDocument();
      expect(mocks.transactions).toHaveBeenLastCalledWith({ limit: 10, offset: 0, flagged: true });
    });

    it("has its own message when nothing is flagged", async () => {
      await showLaunder();
      await screen.findByText("No transactions yet.");
      await userEvent.click(screen.getByRole("checkbox", { name: "Flagged only" }));
      expect(await screen.findByText(/No flagged transfers/)).toBeInTheDocument();
    });

    it("explains when the ledger cannot be loaded, and recovers", async () => {
      mocks.transactions.mockRejectedValueOnce(new ApiError("ledger is down", 500)).mockResolvedValue(aPage([aTransaction({ description: "Back again" })]));
      await showLaunder();
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("Cannot load the ledger");
      expect(alert).toHaveTextContent("ledger is down");
      await userEvent.click(within(alert).getByRole("button", { name: /Try again/ }));
      expect(await screen.findByText("Back again")).toBeInTheDocument();
    });

    it("reloads after a transfer", async () => {
      await showLaunder();
      await screen.findByText("No transactions yet.");
      const callsBefore = mocks.transactions.mock.calls.length;
      await userEvent.selectOptions(frontField(), "1");
      type("5000");
      await userEvent.click(launderButton());
      await screen.findByText(/Sent \$5,000\.00 through/);
      await waitFor(() => expect(mocks.transactions.mock.calls.length).toBeGreaterThan(callsBefore));
    });
  });
});
