"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation } from "@tanstack/react-query";
import AuthShell from "../components/AuthShell";
import ErrorNotice from "../components/ErrorNotice";
import TextField from "../components/TextField";
import { api, errorMessage } from "../lib/api";
import { useSession, useSignIn } from "../lib/session";
import type { RegisterInput } from "../lib/types";
import {
  firstInvalid,
  LIMITS,
  toRegisterInput,
  validateRegistration,
  type FieldErrors,
  type RegisterField,
  type RegisterForm as RegisterValues,
} from "../lib/validation";

const FIELD_ORDER = ["username", "email", "alias", "password", "confirm"] as const;
const EMPTY: RegisterValues = { username: "", email: "", alias: "", password: "", confirm: "" };

interface Problem {
  title: string;
  error?: unknown;
  message?: string;
}

export default function RegisterForm() {
  const router = useRouter();
  const session = useSession();
  // Wrapped, so TanStack Query's extra "context" argument never reaches the API function.
  const register = useMutation({ mutationFn: (input: RegisterInput) => api.auth.register(input) });
  const signIn = useSignIn();

  const [form, setForm] = useState<RegisterValues>(EMPTY);
  const [errors, setErrors] = useState<FieldErrors<RegisterField>>({});
  const [problem, setProblem] = useState<Problem | null>(null);

  // Signed in (a moment ago, or already before this visit): go to the dashboard.
  const signedIn = Boolean(session.data);
  useEffect(() => {
    if (signedIn) router.replace("/dashboard");
  }, [signedIn, router]);

  const set = (field: RegisterField) => (value: string) => setForm((current) => ({ ...current, [field]: value }));

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setProblem(null);

    const found = validateRegistration(form);
    setErrors(found);
    const first = firstInvalid(found, FIELD_ORDER);
    if (first) {
      document.getElementById(`register-${first}`)?.focus();
      return;
    }

    const input = toRegisterInput(form);
    try {
      await register.mutateAsync(input);
    } catch (error) {
      setProblem({ title: "Registration failed", error });
      return;
    }
    try {
      await signIn.mutateAsync({ email: input.email, password: input.password });
    } catch (error) {
      setProblem({
        title: "Account created",
        message: `Your account was created, but signing in failed: ${errorMessage(error, "please try again")} Use the sign-in page to continue.`,
      });
    }
  }

  const busy = register.isPending || signIn.isPending || signedIn;

  return (
    <AuthShell
      kicker="New identity"
      title={["Create", "your alias"]}
      footer={
        <>
          Already have one? <Link href="/login">Sign in</Link>
        </>
      }
    >
      <form onSubmit={submit} noValidate>
        {problem && <ErrorNotice error={problem.error} message={problem.message} title={problem.title} />}
        {signedIn && (
          <p className="auth__status" role="status">
            Welcome. Taking you to your empire&hellip;
          </p>
        )}
        <TextField
          id="register-username"
          label="Username"
          value={form.username}
          onChange={set("username")}
          error={errors.username}
          hint={`${LIMITS.usernameMin}-${LIMITS.usernameMax} characters: letters, numbers, dot, dash or underscore. Saved in lower case.`}
          autoComplete="username"
          required
          disabled={busy}
        />
        <TextField id="register-email" label="Email" type="email" value={form.email} onChange={set("email")} error={errors.email} autoComplete="email" required disabled={busy} />
        <TextField
          id="register-alias"
          label="Alias (optional)"
          value={form.alias}
          onChange={set("alias")}
          error={errors.alias}
          hint="The name other people see. Your username is used if you leave it empty."
          autoComplete="nickname"
          disabled={busy}
        />
        <TextField
          id="register-password"
          label="Password"
          type="password"
          value={form.password}
          onChange={set("password")}
          error={errors.password}
          hint={`${LIMITS.passwordMin} to ${LIMITS.passwordMaxBytes} characters.`}
          autoComplete="new-password"
          required
          disabled={busy}
        />
        <TextField
          id="register-confirm"
          label="Repeat the password"
          type="password"
          value={form.confirm}
          onChange={set("confirm")}
          error={errors.confirm}
          autoComplete="new-password"
          required
          disabled={busy}
        />
        <button type="submit" className="btn btn--block" disabled={busy}>
          {register.isPending || signIn.isPending ? "Creating\u2026" : "Say my name"}
        </button>
      </form>
    </AuthShell>
  );
}
