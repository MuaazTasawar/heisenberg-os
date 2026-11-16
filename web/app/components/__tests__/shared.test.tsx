import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../lib/api";
import { aGameState, anEmpire, renderWithClient } from "../../test-utils";
import GameStateGate from "../GameStateGate";
import Pagination from "../Pagination";
import SuspicionBar from "../SuspicionBar";
import { render } from "@testing-library/react";

const mocks = vi.hoisted(() => ({ state: vi.fn() }));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, api: { game: { state: mocks.state } } };
});

beforeEach(() => {
  mocks.state.mockReset();
});

describe("SuspicionBar", () => {
  const meter = () => screen.getByRole("meter", { name: "Suspicion" });

  it("shows the value as a bar and in words", () => {
    render(<SuspicionBar value={42.4} />);
    expect(meter()).toHaveAttribute("aria-valuenow", "42.4");
    expect(screen.getByText(/42 \/ 100/)).toHaveTextContent("Medium");
    expect(meter().firstElementChild).toHaveStyle({ width: "42.4%" });
  });

  it.each([
    [0, "Low"],
    [24.9, "Low"],
    [25, "Medium"],
    [50, "High"],
    [75, "Critical"],
    [100, "Critical"],
  ])("labels %s as %s", (value, label) => {
    render(<SuspicionBar value={value} />);
    expect(screen.getByText(/\/ 100/)).toHaveTextContent(label);
  });

  it("stays inside 0-100 and survives bad numbers", () => {
    const { rerender } = render(<SuspicionBar value={250} />);
    expect(meter()).toHaveAttribute("aria-valuenow", "100");
    rerender(<SuspicionBar value={-5} />);
    expect(meter()).toHaveAttribute("aria-valuenow", "0");
    rerender(<SuspicionBar value={Number.NaN} />);
    expect(meter()).toHaveAttribute("aria-valuenow", "0");
  });

  it("uses a different colour for each band", () => {
    const { rerender } = render(<SuspicionBar value={10} />);
    const low = (meter().firstElementChild as HTMLElement).style.background;
    rerender(<SuspicionBar value={90} />);
    const critical = (meter().firstElementChild as HTMLElement).style.background;
    expect(low).not.toBe("");
    expect(low).not.toBe(critical);
  });
});

describe("Pagination", () => {
  const previous = () => screen.getByRole("button", { name: "Previous" });
  const next = () => screen.getByRole("button", { name: "Next" });

  it("renders nothing when there is nothing to page", () => {
    const { container } = render(<Pagination total={0} limit={10} offset={0} onChange={() => {}} label="Transactions" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the range on the first page and cannot go back", () => {
    render(<Pagination total={25} limit={10} offset={0} onChange={() => {}} label="Transactions" />);
    expect(screen.getByRole("navigation", { name: "Transactions pages" })).toBeInTheDocument();
    expect(screen.getByText(/Showing 1.10 of 25/)).toBeInTheDocument();
    expect(previous()).toBeDisabled();
    expect(next()).toBeEnabled();
  });

  it("shows a short last page and cannot go forward", () => {
    render(<Pagination total={25} limit={10} offset={20} onChange={() => {}} label="Transactions" />);
    expect(screen.getByText(/Showing 21.25 of 25/)).toBeInTheDocument();
    expect(next()).toBeDisabled();
    expect(previous()).toBeEnabled();
  });

  it("cannot go forward when the last page is exactly full", () => {
    render(<Pagination total={20} limit={10} offset={10} onChange={() => {}} label="Transactions" />);
    expect(screen.getByText(/Showing 11.20 of 20/)).toBeInTheDocument();
    expect(next()).toBeDisabled();
  });

  it("asks for the neighbouring offsets", async () => {
    const onChange = vi.fn();
    render(<Pagination total={35} limit={10} offset={10} onChange={onChange} label="Cases" />);
    await userEvent.click(next());
    expect(onChange).toHaveBeenLastCalledWith(20);
    await userEvent.click(previous());
    expect(onChange).toHaveBeenLastCalledWith(0);
  });

  it("never asks for a negative offset", async () => {
    const onChange = vi.fn();
    render(<Pagination total={35} limit={10} offset={5} onChange={onChange} label="Cases" />);
    await userEvent.click(previous());
    expect(onChange).toHaveBeenCalledWith(0);
  });
});

describe("GameStateGate", () => {
  const show = () => renderWithClient(<GameStateGate>{(data) => <p>Empire: {data.empire.name}</p>}</GameStateGate>);

  it("shows placeholders while loading", () => {
    mocks.state.mockReturnValue(new Promise(() => {}));
    show();
    expect(screen.getByRole("status", { name: "Loading your empire" })).toHaveAttribute("aria-busy", "true");
  });

  it("shows the page once the state arrives", async () => {
    mocks.state.mockResolvedValue(aGameState({ empire: anEmpire({ name: "Casa Tranquila" }) }));
    show();
    expect(await screen.findByText("Empire: Casa Tranquila")).toBeInTheDocument();
    expect(screen.queryByRole("status", { name: "Loading your empire" })).not.toBeInTheDocument();
  });

  it("shows the error with a retry, and recovers", async () => {
    mocks.state.mockRejectedValueOnce(new ApiError("database is down", 500)).mockResolvedValue(aGameState());
    show();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Cannot load your empire");
    expect(alert).toHaveTextContent("database is down");

    await userEvent.click(screen.getByRole("button", { name: /Try again/ }));
    expect(await screen.findByText(/Empire:/)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("keeps the last numbers on screen when a refresh fails", async () => {
    mocks.state.mockResolvedValueOnce(aGameState({ empire: anEmpire({ name: "Casa Tranquila" }) }));
    const { client } = show();
    await screen.findByText("Empire: Casa Tranquila");

    mocks.state.mockRejectedValue(new ApiError("bad gateway", 502));
    await client.refetchQueries({ queryKey: ["game", "state"] });

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Could not refresh"));
    expect(screen.getByText("Empire: Casa Tranquila")).toBeInTheDocument();
  });
});
