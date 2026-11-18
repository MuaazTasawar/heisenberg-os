import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../../lib/api";
import { resetAuthStore } from "../../../lib/store";
import type { LegalCase, Page } from "../../../lib/types";
import { aCase, aUser, renderWithClient } from "../../../test-utils";
import LegalView from "../LegalView";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  me: vi.fn(),
  logout: vi.fn(),
  consult: vi.fn(),
  cases: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
  usePathname: () => "/dashboard/legal",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    api: {
      auth: { me: mocks.me, logout: mocks.logout },
      legal: { consult: mocks.consult, cases: mocks.cases },
    },
  };
});

const aPage = (items: LegalCase[], overrides: Partial<Page<LegalCase>> = {}): Page<LegalCase> => ({
  items,
  total: items.length,
  limit: 10,
  offset: 0,
  ...overrides,
});

beforeEach(() => {
  resetAuthStore();
  mocks.replace.mockReset();
  mocks.logout.mockReset();
  mocks.me.mockReset().mockResolvedValue(aUser());
  mocks.consult.mockReset().mockResolvedValue(aCase({ id: 50, charge: "Possession", advice: "Say nothing.\nAsk for me.", defense: "Chain of custody." }));
  mocks.cases.mockReset().mockResolvedValue(aPage([]));
});

async function showLegal() {
  const rendered = renderWithClient(<LegalView />);
  await screen.findByLabelText("The charge");
  return rendered;
}

const chargeField = () => screen.getByLabelText("The charge") as HTMLInputElement;
const evidenceField = () => screen.getByLabelText(/What they have on you/) as HTMLTextAreaElement;
const contextField = () => screen.getByLabelText(/Your situation/) as HTMLTextAreaElement;
const askButton = () => screen.getByRole("button", { name: /Ask Saul|Asking Saul/ });
const typeInto = (field: HTMLElement, value: string) => fireEvent.change(field, { target: { value } });

describe("LegalView", () => {
  it("asks for nothing before the player is known to be signed in", async () => {
    mocks.me.mockRejectedValue(new ApiError("authentication required", 401));
    renderWithClient(<LegalView />);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login?next=%2Fdashboard%2Flegal"));
    expect(mocks.cases).not.toHaveBeenCalled();
    expect(mocks.consult).not.toHaveBeenCalled();
  });

  describe("the form", () => {
    it("has labelled fields and counters that start at zero", async () => {
      await showLegal();
      expect(chargeField()).toHaveAccessibleDescription("0 / 200");
      expect(evidenceField()).toHaveAccessibleDescription("0 / 2000");
      expect(contextField()).toHaveAccessibleDescription("0 / 2000");
    });

    it("counts characters as people do, without the white space around the text", async () => {
      await showLegal();
      typeInto(chargeField(), "  \u{1F600}\u{1F600}abc  ");
      expect(chargeField()).toHaveAccessibleDescription("5 / 200");
      typeInto(evidenceField(), " hello ");
      expect(evidenceField()).toHaveAccessibleDescription("5 / 2000");
    });

    it("marks a counter that is over its limit", async () => {
      await showLegal();
      typeInto(chargeField(), "c".repeat(200));
      expect(screen.getByText("200 / 200")).not.toHaveAttribute("data-over");
      typeInto(chargeField(), "c".repeat(201));
      expect(screen.getByText("201 / 200")).toHaveAttribute("data-over", "true");
    });

    it("asks for a charge before calling the server", async () => {
      await showLegal();
      await userEvent.click(askButton());
      expect(await screen.findByText("Describe the charge.")).toBeInTheDocument();
      expect(chargeField()).toHaveAttribute("aria-invalid", "true");
      expect(mocks.consult).not.toHaveBeenCalled();
    });

    it("rejects text that is too long, naming the field", async () => {
      await showLegal();
      typeInto(chargeField(), "c".repeat(201));
      typeInto(evidenceField(), "e".repeat(2001));
      typeInto(contextField(), "x".repeat(2001));
      await userEvent.click(askButton());
      expect(await screen.findByText("The charge must be at most 200 characters.")).toBeInTheDocument();
      expect(screen.getByText("The evidence must be at most 2000 characters.")).toBeInTheDocument();
      expect(screen.getByText("Your situation must be at most 2000 characters.")).toBeInTheDocument();
      expect(mocks.consult).not.toHaveBeenCalled();
    });

    it("accepts text exactly at the limits", async () => {
      await showLegal();
      typeInto(chargeField(), "c".repeat(200));
      typeInto(evidenceField(), "e".repeat(2000));
      await userEvent.click(askButton());
      await waitFor(() => expect(mocks.consult).toHaveBeenCalledTimes(1));
    });
  });

  describe("asking Saul", () => {
    it("sends only the charge when nothing else was written", async () => {
      await showLegal();
      typeInto(chargeField(), "  Possession  ");
      await userEvent.click(askButton());
      await waitFor(() => expect(mocks.consult).toHaveBeenCalledTimes(1));
      expect(mocks.consult).toHaveBeenCalledWith({ charge: "Possession" });
    });

    it("sends the evidence and the situation, trimmed, when they were written", async () => {
      await showLegal();
      typeInto(chargeField(), "Possession");
      typeInto(evidenceField(), "  a bag of blue  ");
      typeInto(contextField(), "  I was at work  ");
      await userEvent.click(askButton());
      await waitFor(() => expect(mocks.consult).toHaveBeenCalledTimes(1));
      expect(mocks.consult).toHaveBeenCalledWith({ charge: "Possession", evidence: "a bag of blue", player_context: "I was at work" });
    });

    it("shows what Saul says, clears the form and reloads the case files", async () => {
      mocks.consult.mockResolvedValue(
        aCase({ id: 50, charge: "Possession", status: "open", severity: 3, risk: "high", evidence: "", advice: "Say nothing.\nAsk for me.", defense: "Chain of custody." }),
      );
      await showLegal();
      await screen.findByText(/No case files yet/);
      const loadsBefore = mocks.cases.mock.calls.length;
      typeInto(chargeField(), "Possession");
      typeInto(evidenceField(), "a bag");
      await userEvent.click(askButton());

      const result = await screen.findByRole("region", { name: "Saul's advice" });
      expect(within(result).getByText("Possession")).toBeInTheDocument();
      expect(within(result).getByText("High")).toBeInTheDocument();
      expect(within(result).getByText("Open")).toBeInTheDocument();
      expect(within(result).getByText("severity 3 of 4")).toBeInTheDocument();
      // The line break stays in the text (the page shows it with white-space: pre-wrap).
      expect(within(result).getByText(/Say nothing\./).textContent).toBe("Say nothing.\nAsk for me.");
      expect(within(result).getByText("Chain of custody.")).toBeInTheDocument();
      expect(within(result).queryByText("Evidence")).not.toBeInTheDocument();

      expect(chargeField().value).toBe("");
      expect(evidenceField().value).toBe("");
      await waitFor(() => expect(mocks.cases.mock.calls.length).toBeGreaterThan(loadsBefore));
    });

    it("shows the evidence Saul was given when the case has some", async () => {
      mocks.consult.mockResolvedValue(aCase({ id: 51, charge: "Possession", evidence: "A bag of blue crystals." }));
      await showLegal();
      typeInto(chargeField(), "Possession");
      await userEvent.click(askButton());
      const result = await screen.findByRole("region", { name: "Saul's advice" });
      expect(within(result).getByText("A bag of blue crystals.")).toBeInTheDocument();
    });

    it("shows that Saul is working and cannot be asked twice", async () => {
      mocks.consult.mockReturnValue(new Promise(() => {}));
      await showLegal();
      typeInto(chargeField(), "Possession");
      await userEvent.click(askButton());
      expect(await screen.findByText(/Saul is thinking/)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Asking Saul/ })).toBeDisabled();
      expect(chargeField()).toBeDisabled();
      expect(mocks.consult).toHaveBeenCalledTimes(1);
    });

    it.each([
      ["Saul is not available (no API key configured)", 503],
      ["Saul is not answering right now", 502],
      ["too many requests, slow down", 429],
    ])("shows the server's own words when it fails: %s", async (message, status) => {
      mocks.consult.mockRejectedValue(new ApiError(message, status));
      await showLegal();
      typeInto(chargeField(), "Possession");
      typeInto(evidenceField(), "a bag");
      await userEvent.click(askButton());

      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("Saul could not take your case");
      expect(alert).toHaveTextContent(message);
      expect(chargeField().value).toBe("Possession");
      expect(evidenceField().value).toBe("a bag");
      expect(askButton()).toBeEnabled();
      expect(screen.queryByRole("region", { name: "Saul's advice" })).not.toBeInTheDocument();
    });
  });

  describe("case files", () => {
    const files = () => within(screen.getByRole("list")).getAllByRole("listitem");

    it("asks for the first page", async () => {
      await showLegal();
      await waitFor(() => expect(mocks.cases).toHaveBeenCalledWith({ limit: 10, offset: 0 }));
    });

    it("has a message when there are none", async () => {
      await showLegal();
      expect(await screen.findByText(/No case files yet/)).toBeInTheDocument();
      expect(screen.queryByRole("list")).not.toBeInTheDocument();
    });

    it("lists each case with its risk, status and age", async () => {
      mocks.cases.mockResolvedValue(
        aPage([
          aCase({ id: 2, charge: "Possession with intent", risk: "high", status: "open" }),
          aCase({ id: 1, charge: "Fraud", risk: "low", status: "dismissed" }),
        ]),
      );
      await showLegal();
      await screen.findByText("Possession with intent");
      expect(files()).toHaveLength(2);
      expect(files()[0]).toHaveTextContent("Possession with intent");
      expect(files()[0]).toHaveTextContent("High");
      expect(files()[0]).toHaveTextContent("Open");
      expect(files()[0]).toHaveTextContent("1 h ago");
      expect(files()[1]).toHaveTextContent("Dismissed");
      expect(files()[1]).toHaveTextContent("Low");
    });

    it("keeps what Saul said inside the case, to open when wanted", async () => {
      mocks.cases.mockResolvedValue(aPage([aCase({ id: 2, charge: "Possession", evidence: "A bag.", advice: "Say nothing.", defense: "Chain of custody." })]));
      await showLegal();
      await screen.findByText("Possession");
      const details = files()[0].querySelector("details") as HTMLDetailsElement;
      expect(details).not.toBeNull();
      expect(details.open).toBe(false);
      expect(details.querySelector("summary")).toHaveTextContent("Possession");
      expect(within(details).getByText("A bag.")).toBeInTheDocument();
      expect(within(details).getByText("Say nothing.")).toBeInTheDocument();
      expect(within(details).getByText("Chain of custody.")).toBeInTheDocument();
    });

    it("leaves out the evidence heading when a case has none", async () => {
      mocks.cases.mockResolvedValue(aPage([aCase({ id: 2, evidence: "" })]));
      await showLegal();
      await screen.findByText("Possession with intent");
      expect(within(files()[0]).queryByText("Evidence")).not.toBeInTheDocument();
      expect(within(files()[0]).getByText("Advice")).toBeInTheDocument();
      expect(within(files()[0]).getByText("Defense")).toBeInTheDocument();
    });

    it("pages with the offsets the server expects", async () => {
      const first = Array.from({ length: 10 }, (_, index) => aCase({ id: 100 - index, charge: `Charge ${index}` }));
      mocks.cases.mockImplementation(async (query: { offset?: number }) =>
        query.offset ? aPage([aCase({ id: 5, charge: "Charge 10" })], { total: 12, offset: query.offset }) : aPage(first, { total: 12 }),
      );
      await showLegal();
      await screen.findByText("Charge 0");
      expect(screen.getByText(/Showing 1.10 of 12/)).toBeInTheDocument();

      await userEvent.click(screen.getByRole("button", { name: "Next" }));
      expect(await screen.findByText("Charge 10")).toBeInTheDocument();
      expect(mocks.cases).toHaveBeenLastCalledWith({ limit: 10, offset: 10 });
      expect(screen.queryByText("Charge 0")).not.toBeInTheDocument();

      await userEvent.click(screen.getByRole("button", { name: "Previous" }));
      await screen.findByText("Charge 0");
      expect(mocks.cases).toHaveBeenLastCalledWith({ limit: 10, offset: 0 });
    });

    it("explains when the files cannot be loaded, and recovers", async () => {
      mocks.cases.mockRejectedValueOnce(new ApiError("files are down", 500)).mockResolvedValue(aPage([aCase({ id: 2, charge: "Back again" })]));
      await showLegal();
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("Cannot load your case files");
      expect(alert).toHaveTextContent("files are down");
      await userEvent.click(within(alert).getByRole("button", { name: /Try again/ }));
      expect(await screen.findByText("Back again")).toBeInTheDocument();
    });

    it("shows that it is loading", async () => {
      mocks.cases.mockReturnValue(new Promise(() => {}));
      await showLegal();
      expect(await screen.findByText(/Pulling your files/)).toBeInTheDocument();
    });
  });
});
