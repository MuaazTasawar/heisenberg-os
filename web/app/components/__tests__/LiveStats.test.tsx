import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import Home from "../../page";
import { renderWithClient } from "../../test-utils";
import { ApiError } from "../../lib/api";
import LiveStats from "../LiveStats";

const mocks = vi.hoisted(() => ({ stats: vi.fn() }));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, api: { stats: { get: mocks.stats } } };
});

beforeEach(() => {
  mocks.stats.mockReset();
});

describe("LiveStats", () => {
  it("shows placeholders, and no numbers, while loading", () => {
    mocks.stats.mockReturnValue(new Promise(() => {}));
    renderWithClient(<LiveStats />);
    expect(screen.getByLabelText("Live statistics")).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByText(/\d/)).not.toBeInTheDocument();
  });

  it("shows the real numbers, formatted", async () => {
    mocks.stats.mockResolvedValue({ empires: 1234, kilograms_cooked: 1234.5, dirty_laundered: 1_500_000, flagged_transfers: 7, raids: 3 });
    renderWithClient(<LiveStats />);

    expect(await screen.findByText("1,234")).toBeInTheDocument();
    expect(screen.getByText("1,234.5 kg")).toBeInTheDocument();
    expect(screen.getByText("$1.5M")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    for (const label of ["Empires", "Kilograms cooked", "Dirty money laundered", "Flagged transfers", "DEA raids"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("pairs every number with its label in the markup, label first", async () => {
    mocks.stats.mockResolvedValue({ empires: 2, kilograms_cooked: 0, dirty_laundered: 0, flagged_transfers: 0, raids: 0 });
    renderWithClient(<LiveStats />);
    const term = await screen.findByText("Empires");
    expect(term.tagName).toBe("DT");
    expect(term.nextElementSibling?.tagName).toBe("DD");
    expect(term.nextElementSibling).toHaveTextContent("2");
  });

  it("shows zero as zero instead of hiding it", async () => {
    mocks.stats.mockResolvedValue({ empires: 0, kilograms_cooked: 0, dirty_laundered: 0, flagged_transfers: 0, raids: 0 });
    renderWithClient(<LiveStats />);
    expect(await screen.findByText("0 kg")).toBeInTheDocument();
    expect(screen.getByText("$0")).toBeInTheDocument();
    expect(screen.getAllByText("0")).toHaveLength(3);
  });

  it("says the statistics are unavailable, and invents nothing, when they cannot be loaded", async () => {
    mocks.stats.mockRejectedValue(new ApiError("Cannot reach the server.", 0));
    renderWithClient(<LiveStats />);
    expect(await screen.findByText("Live statistics are unavailable right now.")).toBeInTheDocument();
    expect(screen.queryByText(/\d/)).not.toBeInTheDocument();
  });
});

describe("the landing page", () => {
  it("offers sign-in and registration, and shows no made-up figures", async () => {
    mocks.stats.mockResolvedValue({ empires: 5, kilograms_cooked: 100, dirty_laundered: 250_000, flagged_transfers: 2, raids: 1 });
    const { container } = renderWithClient(<Home />);

    expect(screen.getByRole("heading", { level: 1, name: "Heisenberg" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Enter" })).toHaveAttribute("href", "/login");
    expect(screen.getByRole("link", { name: "New identity" })).toHaveAttribute("href", "/register");
    expect(await screen.findByText("$250K")).toBeInTheDocument();
    for (const fake of ["99.1%", "$47M", "v2.0.1", "SECURE"]) expect(container.textContent).not.toContain(fake);
  });
});
