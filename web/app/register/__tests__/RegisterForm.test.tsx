import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../lib/api";
import { resetAuthStore, useAuthStore } from "../../lib/store";
import { aUser, renderWithClient } from "../../test-utils";
import RegisterForm from "../RegisterForm";

const mocks = vi.hoisted(() => ({ replace: vi.fn(), me: vi.fn(), login: vi.fn(), register: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
  usePathname: () => "/register",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, api: { auth: { me: mocks.me, login: mocks.login, register: mocks.register } } };
});

const user = aUser();
const SUBMIT = { name: "Say my name" };

beforeEach(() => {
  resetAuthStore();
  mocks.replace.mockReset();
  mocks.me.mockReset().mockRejectedValue(new ApiError("authentication required", 401));
  mocks.register.mockReset().mockResolvedValue(user);
  mocks.login.mockReset().mockResolvedValue({ token: "ignored", expires_at: "", user });
});

interface Values { username?: string; email?: string; alias?: string; password?: string; confirm?: string }

async function fillIn({ username = "walter", email = "walter@example.com", alias = "", password = "iamthedanger", confirm = password }: Values = {}) {
  await userEvent.type(screen.getByLabelText("Username"), username);
  await userEvent.type(screen.getByLabelText("Email"), email);
  if (alias) await userEvent.type(screen.getByLabelText("Alias (optional)"), alias);
  await userEvent.type(screen.getByLabelText("Password"), password);
  await userEvent.type(screen.getByLabelText("Repeat the password"), confirm);
}

describe("RegisterForm", () => {
  it("has labelled fields with hints and the right autofill hints", () => {
    renderWithClient(<RegisterForm />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Create your alias");
    expect(screen.getByLabelText("Username")).toHaveAttribute("autocomplete", "username");
    expect(screen.getByLabelText("Password")).toHaveAttribute("autocomplete", "new-password");
    expect(screen.getByLabelText("Repeat the password")).toHaveAttribute("autocomplete", "new-password");
    expect(screen.getByLabelText("Username")).toHaveAccessibleDescription(/3-32 characters/);
    expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("8 to 72 characters.");
    expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
  });

  it("shows every problem at once, focuses the first and does not call the server", async () => {
    renderWithClient(<RegisterForm />);
    await userEvent.click(screen.getByRole("button", SUBMIT));

    expect(screen.getByText("Choose a username.")).toBeInTheDocument();
    expect(screen.getByText("Enter your email address.")).toBeInTheDocument();
    expect(screen.getByText("Password must be at least 8 characters.")).toBeInTheDocument();
    expect(screen.getByLabelText("Username")).toHaveFocus();
    expect(mocks.register).not.toHaveBeenCalled();
  });

  it("checks the username, the password length in bytes and the repeated password", async () => {
    renderWithClient(<RegisterForm />);
    await fillIn({ username: "Walter White", password: "\u{1F575}".repeat(20), confirm: "different" });
    await userEvent.click(screen.getByRole("button", SUBMIT));

    expect(screen.getByLabelText("Username")).toHaveAccessibleDescription(/must be 3-32 characters/);
    expect(screen.getByText(/at most 72 bytes/)).toBeInTheDocument();
    expect(screen.getByText("The passwords do not match.")).toBeInTheDocument();
    expect(mocks.register).not.toHaveBeenCalled();
  });

  it("limits the alias", async () => {
    renderWithClient(<RegisterForm />);
    await fillIn({ alias: "a".repeat(65) });
    await userEvent.click(screen.getByRole("button", SUBMIT));
    expect(screen.getByText("Alias must be at most 64 characters.")).toBeInTheDocument();
    expect(mocks.register).not.toHaveBeenCalled();
  });

  it("registers, signs in with the same credentials and goes to the dashboard", async () => {
    renderWithClient(<RegisterForm />);
    await fillIn({ username: "WalterWhite", email: "Walter@Example.COM", alias: "Heisenberg", password: "iamthedanger" });
    await userEvent.click(screen.getByRole("button", SUBMIT));

    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/dashboard"));
    expect(mocks.register).toHaveBeenCalledWith({ username: "walterwhite", email: "walter@example.com", password: "iamthedanger", alias: "Heisenberg" });
    expect(mocks.login).toHaveBeenCalledWith({ email: "walter@example.com", password: "iamthedanger" });
    expect(useAuthStore.getState()).toMatchObject({ user, status: "authenticated" });
  });

  it("leaves out an empty alias so the server uses the username", async () => {
    renderWithClient(<RegisterForm />);
    await fillIn();
    await userEvent.click(screen.getByRole("button", SUBMIT));
    await waitFor(() => expect(mocks.register).toHaveBeenCalled());
    expect(mocks.register.mock.calls[0][0]).not.toHaveProperty("alias");
  });

  it("shows the server's reason, keeps what was typed and does not sign in", async () => {
    mocks.register.mockRejectedValue(new ApiError("username or email is already taken", 409));
    renderWithClient(<RegisterForm />);
    await fillIn({ password: "iamthedanger" });
    await userEvent.click(screen.getByRole("button", SUBMIT));

    expect(await screen.findByRole("alert")).toHaveTextContent("Registration failed");
    expect(screen.getByRole("alert")).toHaveTextContent("username or email is already taken");
    expect(screen.getByLabelText("Username")).toHaveValue("walter");
    expect(screen.getByLabelText("Password")).toHaveValue("iamthedanger");
    expect(screen.getByRole("button", SUBMIT)).toBeEnabled();
    expect(mocks.login).not.toHaveBeenCalled();
    expect(mocks.replace).not.toHaveBeenCalled();
  });

  it("passes on several server complaints exactly as the server wrote them", async () => {
    mocks.register.mockRejectedValue(new ApiError("password must be at least 8 characters; email must be a valid email address", 400));
    renderWithClient(<RegisterForm />);
    await fillIn();
    await userEvent.click(screen.getByRole("button", SUBMIT));
    expect(await screen.findByRole("alert")).toHaveTextContent("password must be at least 8 characters; email must be a valid email address");
  });

  it("explains what happened when the account exists but the automatic sign-in fails", async () => {
    mocks.login.mockRejectedValue(new ApiError("Too many requests. Try again in 20 seconds.", 429, 20));
    renderWithClient(<RegisterForm />);
    await fillIn();
    await userEvent.click(screen.getByRole("button", SUBMIT));

    const notice = await screen.findByRole("alert");
    expect(notice).toHaveTextContent("Account created");
    expect(notice).toHaveTextContent("Your account was created, but signing in failed");
    expect(notice).toHaveTextContent("Try again in 20 seconds");
    expect(notice).toHaveTextContent("sign-in page");
    expect(mocks.replace).not.toHaveBeenCalled();
  });

  it("cannot be submitted twice while it works", async () => {
    mocks.register.mockReturnValue(new Promise(() => {}));
    renderWithClient(<RegisterForm />);
    await fillIn();
    await userEvent.click(screen.getByRole("button", SUBMIT));

    const button = await screen.findByRole("button", { name: /creating/i });
    expect(button).toBeDisabled();
    expect(screen.getByLabelText("Username")).toBeDisabled();
    await userEvent.click(button);
    expect(mocks.register).toHaveBeenCalledTimes(1);
  });

  it("sends a visitor who is already signed in to the dashboard", async () => {
    mocks.me.mockResolvedValue(user);
    renderWithClient(<RegisterForm />);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/dashboard"));
    expect(screen.getByRole("status")).toHaveTextContent("Taking you to your empire");
  });
});
