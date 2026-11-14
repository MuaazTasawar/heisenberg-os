import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../lib/api";
import { SESSION_KEY } from "../../lib/session";
import { resetAuthStore, useAuthStore } from "../../lib/store";
import { aUser, renderWithClient } from "../../test-utils";
import LoginForm from "../LoginForm";

const mocks = vi.hoisted(() => ({ replace: vi.fn(), search: { value: "" }, me: vi.fn(), login: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
  usePathname: () => "/login",
  useSearchParams: () => new URLSearchParams(mocks.search.value),
}));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, api: { auth: { me: mocks.me, login: mocks.login } } };
});

const user = aUser();

beforeEach(() => {
  resetAuthStore();
  mocks.replace.mockReset();
  mocks.search.value = "";
  mocks.me.mockReset().mockRejectedValue(new ApiError("authentication required", 401));
  mocks.login.mockReset().mockResolvedValue({ token: "ignored", expires_at: "", user });
});

async function signIn(email = "walter@example.com", password = "iamthedanger") {
  await userEvent.type(screen.getByLabelText("Email"), email);
  await userEvent.type(screen.getByLabelText("Password"), password);
  await userEvent.click(screen.getByRole("button", { name: "I am the one who knocks" }));
}

describe("LoginForm", () => {
  it("has labelled fields and the right autofill hints", () => {
    renderWithClient(<LoginForm />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Identify yourself");
    expect(screen.getByLabelText("Email")).toHaveAttribute("type", "email");
    expect(screen.getByLabelText("Email")).toHaveAttribute("autocomplete", "email");
    expect(screen.getByLabelText("Password")).toHaveAttribute("type", "password");
    expect(screen.getByLabelText("Password")).toHaveAttribute("autocomplete", "current-password");
    expect(screen.getByRole("link", { name: "Create one" })).toHaveAttribute("href", "/register");
  });

  it("points at what is missing, focuses the first problem and does not call the server", async () => {
    renderWithClient(<LoginForm />);
    await userEvent.click(screen.getByRole("button", { name: "I am the one who knocks" }));

    expect(screen.getByText("Enter your email address.")).toBeInTheDocument();
    expect(screen.getByText("Enter your password.")).toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toHaveFocus();
    expect(screen.getByLabelText("Email")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Email")).toHaveAccessibleDescription("Enter your email address.");
    expect(mocks.login).not.toHaveBeenCalled();
  });

  it("rejects an email that is not an email", async () => {
    renderWithClient(<LoginForm />);
    await signIn("walter", "iamthedanger");
    expect(screen.getByText("Enter a valid email address.")).toBeInTheDocument();
    expect(mocks.login).not.toHaveBeenCalled();
  });

  it("signs in, remembers who signed in and goes to the dashboard", async () => {
    const { client } = renderWithClient(<LoginForm />);
    await signIn();

    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/dashboard"));
    expect(mocks.login).toHaveBeenCalledWith({ email: "walter@example.com", password: "iamthedanger" });
    expect(useAuthStore.getState()).toMatchObject({ user, status: "authenticated" });
    expect(client.getQueryData(SESSION_KEY)).toEqual(user);
  });

  it("sends the password exactly as typed", async () => {
    renderWithClient(<LoginForm />);
    await signIn("walter@example.com", "  Spaces And CAPS  ");
    await waitFor(() => expect(mocks.login).toHaveBeenCalled());
    expect(mocks.login).toHaveBeenCalledWith({ email: "walter@example.com", password: "  Spaces And CAPS  " });
  });

  it("goes back to where the player was heading", async () => {
    mocks.search.value = "next=%2Fdashboard%2Flegal%3Fcase%3D5";
    renderWithClient(<LoginForm />);
    await signIn();
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/dashboard/legal?case=5"));
  });

  it("never follows a next that leaves the site", async () => {
    for (const evil of ["https://evil.example/steal", "//evil.example", "/\\evil.example", "javascript:alert(1)", "/login"]) {
      mocks.replace.mockReset();
      mocks.search.value = `next=${encodeURIComponent(evil)}`;
      const { unmount } = renderWithClient(<LoginForm />);
      await signIn();
      await waitFor(() => expect(mocks.replace).toHaveBeenCalled());
      expect(mocks.replace, evil).toHaveBeenCalledWith("/dashboard");
      expect(mocks.replace, evil).not.toHaveBeenCalledWith(evil);
      unmount();
      resetAuthStore();
    }
  });

  it("shows the server's reason, keeps what was typed and allows another try", async () => {
    mocks.login.mockRejectedValue(new ApiError("invalid email or password", 401));
    renderWithClient(<LoginForm />);
    await signIn("walter@example.com", "wrong-password");

    expect(await screen.findByRole("alert")).toHaveTextContent("Sign-in failed");
    expect(screen.getByRole("alert")).toHaveTextContent("invalid email or password");
    expect(screen.getByLabelText("Email")).toHaveValue("walter@example.com");
    expect(screen.getByLabelText("Password")).toHaveValue("wrong-password");
    expect(screen.getByRole("button", { name: "I am the one who knocks" })).toBeEnabled();
    expect(mocks.replace).not.toHaveBeenCalled();
    expect(useAuthStore.getState().status).toBe("anonymous");
  });

  it("tells the player about a rate limit", async () => {
    mocks.login.mockRejectedValue(new ApiError("Too many requests. Try again in 30 seconds.", 429, 30));
    renderWithClient(<LoginForm />);
    await signIn();
    expect(await screen.findByRole("alert")).toHaveTextContent("Try again in 30 seconds");
  });

  it("shows that it is working and cannot be submitted twice", async () => {
    mocks.login.mockReturnValue(new Promise(() => {}));
    renderWithClient(<LoginForm />);
    await signIn();

    const button = await screen.findByRole("button", { name: /verifying/i });
    expect(button).toBeDisabled();
    expect(screen.getByLabelText("Email")).toBeDisabled();
    expect(screen.getByLabelText("Password")).toBeDisabled();
    await userEvent.click(button);
    expect(mocks.login).toHaveBeenCalledTimes(1);
  });

  it("sends a visitor who is already signed in straight on", async () => {
    mocks.me.mockResolvedValue(user);
    renderWithClient(<LoginForm />);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/dashboard"));
    expect(screen.getByRole("status")).toHaveTextContent("Signing you in");
    expect(mocks.login).not.toHaveBeenCalled();
  });

  it("is still usable when the session check itself cannot be made", async () => {
    mocks.me.mockRejectedValue(new ApiError("Cannot reach the server. Check your connection and try again.", 0));
    renderWithClient(<LoginForm />);
    await waitFor(() => expect(mocks.me).toHaveBeenCalled());
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "I am the one who knocks" })).toBeEnabled();
  });
});
