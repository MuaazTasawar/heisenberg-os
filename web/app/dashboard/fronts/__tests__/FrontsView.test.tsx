import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../../lib/api";
import { resetAuthStore } from "../../../lib/store";
import type { FrontType, GameState } from "../../../lib/types";
import { aFront, aGameState, aRules, anEmpire, aUser, renderWithClient } from "../../../test-utils";
import FrontsView from "../FrontsView";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  me: vi.fn(),
  logout: vi.fn(),
  state: vi.fn(),
  types: vi.fn(),
  open: vi.fn(),
  close: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
  usePathname: () => "/dashboard/fronts",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    api: {
      auth: { me: mocks.me, logout: mocks.logout },
      game: { state: mocks.state },
      empire: { frontTypes: mocks.types, openFront: mocks.open, closeFront: mocks.close },
    },
  };
});

const TYPES: FrontType[] = [
  { type: "restaurant", label: "Restaurant", suspicion_factor: 1, description: "Cash-heavy and believable." },
  { type: "car_wash", label: "Car wash", suspicion_factor: 0.8, description: "Lots of small cash payments." },
];

beforeEach(() => {
  resetAuthStore();
  mocks.replace.mockReset();
  mocks.logout.mockReset();
  mocks.me.mockReset().mockResolvedValue(aUser());
  mocks.state.mockReset().mockResolvedValue(aGameState());
  mocks.types.mockReset().mockResolvedValue(TYPES);
  mocks.open.mockReset().mockResolvedValue(aFront({ id: 9, name: "Vamonos Pest", type: "car_wash" }));
  mocks.close.mockReset().mockResolvedValue(aFront({ is_active: false }));
});

async function showFronts(state: GameState = aGameState()) {
  mocks.state.mockResolvedValue(state);
  const rendered = renderWithClient(<FrontsView />);
  await screen.findByText(/Active empire:/);
  return rendered;
}

const nameField = () => screen.getByLabelText("Name") as HTMLInputElement;
const typeField = () => screen.getByLabelText("Type of business") as HTMLSelectElement;
const openButton = () => screen.getByRole("button", { name: /Open front|Opening/ });

async function fillForm(name: string, type: string) {
  await screen.findByRole("option", { name: "Car wash" });
  fireEvent.change(nameField(), { target: { value: name } });
  await userEvent.selectOptions(typeField(), type);
}

const twoFronts = () =>
  aGameState({
    empire: anEmpire({
      front_businesses: [
        aFront({ id: 1, name: "Los Pollos Hermanos", type: "restaurant", suspicion: 40, revenue: 12000 }),
        aFront({ id: 2, name: "Vamonos Pest", type: "car_wash", suspicion: 10 }),
        aFront({ id: 3, name: "Old Shop", is_active: false, suspicion: 100, shutdown_reason: "raided by the DEA: suspicion reached 100" }),
      ],
    }),
  });

describe("FrontsView", () => {
  it("asks for nothing before the player is known to be signed in", async () => {
    mocks.me.mockRejectedValue(new ApiError("authentication required", 401));
    renderWithClient(<FrontsView />);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login?next=%2Fdashboard%2Ffronts"));
    expect(mocks.state).not.toHaveBeenCalled();
    expect(mocks.types).not.toHaveBeenCalled();
  });

  it("counts the open fronts against the limit from the rules", async () => {
    await showFronts({ ...twoFronts(), rules: aRules({ max_active_fronts: 4 }) });
    expect(screen.getByText("Open fronts", { selector: ".stat__label" }).closest(".stat")).toHaveTextContent("2 / 4");
  });

  it("lists open fronts with their type, revenue and suspicion", async () => {
    await showFronts(twoFronts());
    const item = screen.getByText("Los Pollos Hermanos").closest("li") as HTMLElement;
    await waitFor(() => expect(within(item).getByText(/Restaurant/)).toBeInTheDocument());
    expect(item).toHaveTextContent("$12,000 revenue");
    expect(within(item).getByRole("meter", { name: "Suspicion" })).toHaveAttribute("aria-valuenow", "40");
    const openRegion = screen.getByRole("region", { name: "Open fronts" });
    expect(within(openRegion).getByText("Vamonos Pest")).toBeInTheDocument();
    expect(within(openRegion).queryByText("Old Shop")).not.toBeInTheDocument();
  });

  it("lists closed fronts with the reason", async () => {
    await showFronts(twoFronts());
    const item = screen.getByText("Old Shop").closest("li") as HTMLElement;
    expect(item).toHaveClass("front-list__item--closed");
    expect(item).toHaveTextContent("raided by the DEA: suspicion reached 100");
    expect(within(item).queryByRole("button")).not.toBeInTheDocument();
  });

  it("says so when there are no open fronts", async () => {
    await showFronts(aGameState());
    expect(screen.getByText(/No open fronts/)).toBeInTheDocument();
  });

  it("offers the business types the server returns and describes the chosen one", async () => {
    await showFronts();
    expect(await screen.findByRole("option", { name: "Restaurant" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Car wash" })).toBeInTheDocument();
    await userEvent.selectOptions(typeField(), "car_wash");
    expect(typeField()).toHaveAccessibleDescription(/Lots of small cash payments\. Suspicion builds 0\.8\u00d7 as fast as average\./);
  });

  it("checks the form before asking the server", async () => {
    await showFronts();
    await screen.findByRole("option", { name: "Car wash" });
    await userEvent.click(openButton());
    expect(await screen.findByText("Give the front a name.")).toBeInTheDocument();
    expect(screen.getByText("Pick a type of business.")).toBeInTheDocument();
    expect(nameField()).toHaveAttribute("aria-invalid", "true");
    expect(mocks.open).not.toHaveBeenCalled();
  });

  it("rejects a name with angle brackets", async () => {
    await showFronts();
    await fillForm("<b>Pest</b>", "car_wash");
    await userEvent.click(openButton());
    expect(await screen.findByText(/cannot contain < or >/)).toBeInTheDocument();
    expect(mocks.open).not.toHaveBeenCalled();
  });

  it("opens a front with the name tidied, then clears the form and reloads the game", async () => {
    await showFronts();
    await fillForm("  Vamonos    Pest ", "car_wash");
    const loadsBefore = mocks.state.mock.calls.length;

    await userEvent.click(openButton());

    expect(mocks.open).toHaveBeenCalledTimes(1);
    expect(mocks.open).toHaveBeenCalledWith({ name: "Vamonos Pest", type: "car_wash" });
    expect(await screen.findByText(/Vamonos Pest is open/)).toBeInTheDocument();
    expect(nameField().value).toBe("");
    expect(typeField().value).toBe("");
    await waitFor(() => expect(mocks.state.mock.calls.length).toBeGreaterThan(loadsBefore));
  });

  it("shows the server's reason and keeps what was typed", async () => {
    mocks.open.mockRejectedValue(new ApiError("you already have a front with that name", 409));
    await showFronts();
    await fillForm("Vamonos Pest", "car_wash");
    await userEvent.click(openButton());

    expect(await screen.findByRole("alert")).toHaveTextContent("you already have a front with that name");
    expect(screen.getByRole("alert")).toHaveTextContent("Could not open the front");
    expect(nameField().value).toBe("Vamonos Pest");
    expect(typeField().value).toBe("car_wash");
    expect(openButton()).toBeEnabled();
  });

  it("cannot be submitted twice while it works", async () => {
    mocks.open.mockReturnValue(new Promise(() => {}));
    await showFronts();
    await fillForm("Vamonos Pest", "car_wash");
    await userEvent.click(openButton());
    expect(await screen.findByRole("button", { name: /Opening/ })).toBeDisabled();
    expect(mocks.open).toHaveBeenCalledTimes(1);
  });

  it("replaces the form with an explanation at the limit", async () => {
    const full = aGameState({
      empire: anEmpire({ front_businesses: [aFront({ id: 1 }), aFront({ id: 2, name: "B" }), aFront({ id: 3, name: "C" })] }),
      rules: aRules({ max_active_fronts: 3 }),
    });
    await showFronts(full);
    expect(screen.getByText(/already run 3 fronts/)).toBeInTheDocument();
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();
  });

  it("does not count closed fronts towards the limit", async () => {
    const state = aGameState({
      empire: anEmpire({ front_businesses: [aFront({ id: 1 }), aFront({ id: 2, name: "B", is_active: false })] }),
      rules: aRules({ max_active_fronts: 2 }),
    });
    await showFronts(state);
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
  });

  it("explains when the types cannot be loaded, and recovers", async () => {
    mocks.types.mockRejectedValueOnce(new ApiError("types are down", 500)).mockResolvedValue(TYPES);
    await showFronts();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Cannot load the business types");
    expect(openButton()).toBeDisabled();

    await userEvent.click(within(alert).getByRole("button", { name: /Try again/ }));
    expect(await screen.findByRole("option", { name: "Car wash" })).toBeInTheDocument();
    expect(openButton()).toBeEnabled();
  });

  describe("closing a front", () => {
    it("asks first and does nothing until it is confirmed", async () => {
      await showFronts(twoFronts());
      await userEvent.click(screen.getByRole("button", { name: "Close Los Pollos Hermanos" }));

      expect(mocks.close).not.toHaveBeenCalled();
      expect(screen.getByRole("button", { name: "Confirm closing Los Pollos Hermanos" })).toBeInTheDocument();

      await userEvent.click(screen.getByRole("button", { name: "Keep it" }));
      expect(mocks.close).not.toHaveBeenCalled();
      expect(screen.getByRole("button", { name: "Close Los Pollos Hermanos" })).toBeInTheDocument();
    });

    it("only asks about the front that was chosen", async () => {
      await showFronts(twoFronts());
      await userEvent.click(screen.getByRole("button", { name: "Close Los Pollos Hermanos" }));
      expect(screen.getByRole("button", { name: "Close Vamonos Pest" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Confirm closing Vamonos Pest" })).not.toBeInTheDocument();
    });

    it("closes the confirmed front and reloads the game", async () => {
      await showFronts(twoFronts());
      const loadsBefore = mocks.state.mock.calls.length;
      await userEvent.click(screen.getByRole("button", { name: "Close Vamonos Pest" }));
      await userEvent.click(screen.getByRole("button", { name: "Confirm closing Vamonos Pest" }));

      expect(mocks.close).toHaveBeenCalledTimes(1);
      expect(mocks.close).toHaveBeenCalledWith(2);
      await waitFor(() => expect(mocks.state.mock.calls.length).toBeGreaterThan(loadsBefore));
    });

    it("shows why a front could not be closed", async () => {
      mocks.close.mockRejectedValue(new ApiError("front not found", 404));
      await showFronts(twoFronts());
      await userEvent.click(screen.getByRole("button", { name: "Close Vamonos Pest" }));
      await userEvent.click(screen.getByRole("button", { name: "Confirm closing Vamonos Pest" }));

      expect(await screen.findByRole("alert")).toHaveTextContent("front not found");
      expect(screen.getByRole("alert")).toHaveTextContent("Could not close Vamonos Pest");
    });
  });
});
