import type { RegisterInput } from "./types";

// The same rules as the backend (internal/auth). The server stays the authority: these checks
// only save a round trip and point at the field that needs attention.
export const LIMITS = {
  usernameMin: 3,
  usernameMax: 32,
  passwordMin: 8,
  passwordMaxBytes: 72, // bcrypt ignores everything after 72 bytes
  aliasMax: 64,
  emailMax: 254,
} as const;

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const USERNAME = /^[a-z0-9_.-]+$/;

export type FieldErrors<K extends string> = Partial<Record<K, string>>;
export type LoginField = "email" | "password";
export type RegisterField = "username" | "email" | "alias" | "password" | "confirm";

/** Characters as people count them (an emoji is one, although it takes several bytes). */
export function runeCount(text: string): number {
  return Array.from(text).length;
}

export function byteLength(text: string): number {
  return new TextEncoder().encode(text).length;
}

/** The first field, in the order given, that has an error. */
export function firstInvalid<K extends string>(errors: FieldErrors<K>, order: readonly K[]): K | undefined {
  return order.find((field) => errors[field]);
}

export function validateLogin(input: { email: string; password: string }): FieldErrors<LoginField> {
  const errors: FieldErrors<LoginField> = {};
  const email = input.email.trim();
  if (!email) errors.email = "Enter your email address.";
  else if (!EMAIL.test(email)) errors.email = "Enter a valid email address.";
  if (!input.password) errors.password = "Enter your password.";
  return errors;
}

export interface RegisterForm {
  username: string;
  email: string;
  alias: string;
  password: string;
  confirm: string;
}

export function validateRegistration(form: RegisterForm): FieldErrors<RegisterField> {
  const errors: FieldErrors<RegisterField> = {};

  const username = form.username.trim().toLowerCase();
  const length = runeCount(username);
  if (!username) errors.username = "Choose a username.";
  else if (length < LIMITS.usernameMin || length > LIMITS.usernameMax || !USERNAME.test(username)) {
    errors.username = `Username must be ${LIMITS.usernameMin}-${LIMITS.usernameMax} characters: letters, numbers, dot, dash or underscore.`;
  }

  const email = form.email.trim().toLowerCase();
  if (!email) errors.email = "Enter your email address.";
  else if (email.length > LIMITS.emailMax || !EMAIL.test(email)) errors.email = "Enter a valid email address.";

  if (runeCount(form.alias.trim()) > LIMITS.aliasMax) errors.alias = `Alias must be at most ${LIMITS.aliasMax} characters.`;

  if (runeCount(form.password) < LIMITS.passwordMin) {
    errors.password = `Password must be at least ${LIMITS.passwordMin} characters.`;
  } else if (byteLength(form.password) > LIMITS.passwordMaxBytes) {
    errors.password = `Password must be at most ${LIMITS.passwordMaxBytes} bytes (long passwords or unusual characters count for more than one byte each).`;
  }

  if (form.confirm !== form.password) errors.confirm = "The passwords do not match.";
  return errors;
}

/** What is sent to the server: trimmed, lower-cased where the server does the same, no empty alias. */
export function toRegisterInput(form: RegisterForm): RegisterInput {
  const alias = form.alias.trim();
  return {
    username: form.username.trim().toLowerCase(),
    email: form.email.trim().toLowerCase(),
    password: form.password,
    ...(alias ? { alias } : {}),
  };
}
