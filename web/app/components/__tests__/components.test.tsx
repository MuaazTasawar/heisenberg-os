import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "../../lib/api";
import ErrorNotice from "../ErrorNotice";
import Panel from "../Panel";
import RiskBadge from "../RiskBadge";
import StatTile from "../StatTile";

describe("ErrorNotice", () => {
  it("renders nothing when there is no error", () => {
    const { container } = render(<ErrorNotice />);
    expect(container).toBeEmptyDOMElement();
    const empty = render(<ErrorNotice error={null} />);
    expect(empty.container).toBeEmptyDOMElement();
  });

  it("announces an ApiError with the server's message", () => {
    render(<ErrorNotice error={new ApiError("password must be at least 8 characters", 400)} title="Cannot register" />);
    expect(screen.getByRole("alert")).toHaveTextContent("Cannot register");
    expect(screen.getByRole("alert")).toHaveTextContent("password must be at least 8 characters");
  });

  it("never shows the text of an unexpected error", () => {
    render(<ErrorNotice error={new TypeError("Cannot read properties of undefined (reading 'x')")} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Something went wrong. Please try again.");
    expect(screen.getByRole("alert")).not.toHaveTextContent("undefined");
  });

  it("prefers an explicit message", () => {
    render(<ErrorNotice error={new ApiError("hidden", 500)} message="Pick a front business first." />);
    expect(screen.getByRole("alert")).toHaveTextContent("Pick a front business first.");
    expect(screen.getByRole("alert")).not.toHaveTextContent("hidden");
  });

  it("offers a retry only when it can retry", async () => {
    const { rerender } = render(<ErrorNotice message="Down." />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();

    const retry = vi.fn();
    rerender(<ErrorNotice message="Down." onRetry={retry} />);
    await userEvent.click(screen.getByRole("button", { name: /try again/i }));
    expect(retry).toHaveBeenCalledTimes(1);
  });
});

describe("Panel", () => {
  it("is a region named by its title", () => {
    render(
      <Panel title="Cooking" subtitle="Pick your effort" actions={<button type="button">Cook</button>}>
        <p>body</p>
      </Panel>,
    );
    const region = screen.getByRole("region", { name: "Cooking" });
    expect(region).toHaveTextContent("Pick your effort");
    expect(region).toHaveTextContent("body");
    expect(screen.getByRole("button", { name: "Cook" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Cooking" })).toBeInTheDocument();
  });

  it("gives two panels two different heading ids", () => {
    render(
      <>
        <Panel title="One" />
        <Panel title="Two" />
      </>,
    );
    expect(screen.getByRole("region", { name: "One" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Two" })).toBeInTheDocument();
  });

  it("works without a title", () => {
    const { container } = render(<Panel>just content</Panel>);
    expect(container).toHaveTextContent("just content");
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
  });
});

describe("StatTile", () => {
  it("shows the label, value and hint", () => {
    const { container } = render(<StatTile label="Clean money" value="$45,000" hint="after the 25% fee" tone="good" />);
    expect(container).toHaveTextContent("Clean money");
    expect(container).toHaveTextContent("$45,000");
    expect(container).toHaveTextContent("after the 25% fee");
    expect(container.firstChild).toHaveAttribute("data-tone", "good");
  });

  it("is neutral by default and the hint is optional", () => {
    const { container } = render(<StatTile label="Heat" value={12} />);
    expect(container.firstChild).toHaveAttribute("data-tone", "default");
    expect(container.querySelector(".stat__hint")).toBeNull();
  });
});

describe("RiskBadge", () => {
  it("says the level in words", () => {
    for (const [level, text] of [["low", "Low"], ["medium", "Medium"], ["high", "High"], ["critical", "Critical"]]) {
      const { unmount } = render(<RiskBadge level={level} />);
      expect(screen.getByText(text)).toHaveAttribute("data-risk", level);
      unmount();
    }
  });

  it("shows anything unknown as medium instead of an empty badge", () => {
    render(<RiskBadge level="elevated" />);
    expect(screen.getByText("Medium")).toHaveAttribute("data-risk", "medium");
  });
});
