"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import AuthShell from "../components/AuthShell";
import ErrorNotice from "../components/ErrorNotice";
import TextField from "../components/TextField";
import { safeNextPath } from "../lib/redirect";
import { useSession, useSignIn } from "../lib/session";
import { firstInvalid, validateLogin, type FieldErrors, type LoginField } from "../lib/validation";

export default function LoginForm() {
  const router = useRouter();
  const next = safeNextPath(useSearchParams().get("next"));
  const session = useSession();
  const signIn = useSignIn();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errors, setErrors] = useState<FieldErrors<LoginField>>({});

  // Signed in (just now, or already before this visit): carry on to where the player was going.
  const signedIn = Boolean(session.data);
  useEffect(() => {
    if (signedIn) router.replace(next);
  }, [signedIn, next, router]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const found = validateLogin({ email, password });
    setErrors(found);
    const first = firstInvalid(found, ["email", "password"] as const);
    if (first) {
      document.getElementById(`login-${first}`)?.focus();
      return;
    }
    signIn.mutate({ email: email.trim(), password });
  }

  const busy = signIn.isPending || signedIn;

  return (
    <AuthShell
      kicker="Secure access"
      title={["Identify", "yourself"]}
      footer={
        <>
          No identity? <Link href="/register">Create one</Link>
        </>
      }
    >
      <form onSubmit={submit} noValidate>
        <ErrorNotice error={signIn.error} title="Sign-in failed" />
        {signedIn && (
          <p className="auth__status" role="status">
            Signing you in&hellip;
          </p>
        )}
        <TextField id="login-email" label="Email" type="email" value={email} onChange={setEmail} error={errors.email} autoComplete="email" required disabled={busy} />
        <TextField id="login-password" label="Password" type="password" value={password} onChange={setPassword} error={errors.password} autoComplete="current-password" required disabled={busy} />
        <button type="submit" className="btn btn--block" disabled={busy}>
          {signIn.isPending ? "Verifying\u2026" : "I am the one who knocks"}
        </button>
      </form>
    </AuthShell>
  );
}
