import { formatMoney } from "./format";
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

// ------------------------------------------------------------------- the game's own forms

export const FRONT_LIMITS = { nameMin: 2, nameMax: 40 } as const;
export const CONSULT_LIMITS = { chargeMax: 200, textMax: 2000 } as const;

/** Trims and collapses runs of white space, as the server does with a front's name. */
export function normalizeName(name: string): string {
  return name.trim().split(/\s+/).filter(Boolean).join(" ");
}

/** Keeps an effort inside the range the server allows, to two decimals. */
export function clampEffort(value: number, range: { min_effort: number; max_effort: number }): number {
  if (!Number.isFinite(value)) return range.min_effort;
  return Math.round(Math.min(Math.max(value, range.min_effort), range.max_effort) * 100) / 100;
}

export type FrontField = "name" | "type";

export function validateFront(input: { name: string; type: string }, validTypes: readonly string[]): FieldErrors<FrontField> {
  const errors: FieldErrors<FrontField> = {};
  const name = normalizeName(input.name);
  const length = runeCount(name);
  if (!name) errors.name = "Give the front a name.";
  else if (length < FRONT_LIMITS.nameMin || length > FRONT_LIMITS.nameMax) {
    errors.name = `The name must be ${FRONT_LIMITS.nameMin}-${FRONT_LIMITS.nameMax} characters.`;
  } else if (/[\u0000-\u001f\u007f<>]/.test(name)) {
    errors.name = "The name cannot contain < or > or control characters.";
  }
  if (!input.type) errors.type = "Pick a type of business.";
  else if (!validTypes.includes(input.type)) errors.type = "Pick one of the listed types.";
  return errors;
}

/** Reads "$1,234.50" or "1234" as a number of dollars; anything else (including 3 decimals) is null. */
export function parseAmount(text: string): number | null {
  const cleaned = text.trim().replace(/^\$/, "").replace(/,/g, "").trim();
  if (!/^\d+(\.\d{1,2})?$/.test(cleaned)) return null;
  const value = Number(cleaned);
  return Number.isFinite(value) ? value : null;
}

export type LaunderField = "amount" | "front";

export function validateLaunder(
  input: { amount: string; frontId: string },
  limits: { min: number; dirty: number },
  openFrontIds: readonly number[],
): FieldErrors<LaunderField> {
  const errors: FieldErrors<LaunderField> = {};
  const amount = parseAmount(input.amount);
  if (amount === null) errors.amount = "Enter an amount in dollars, for example 5000.";
  else if (amount < limits.min) errors.amount = `The minimum transfer is ${formatMoney(limits.min)}.`;
  else if (amount > limits.dirty) errors.amount = `You only have ${formatMoney(limits.dirty)} in dirty money.`;

  if (!input.frontId) errors.front = "Choose which front business to use.";
  else if (!openFrontIds.includes(Number(input.frontId))) errors.front = "Choose one of your open front businesses.";
  return errors;
}

export type ConsultField = "charge" | "evidence" | "context";

export function validateConsult(input: { charge: string; evidence: string; context: string }): FieldErrors<ConsultField> {
  const errors: FieldErrors<ConsultField> = {};
  const charge = input.charge.trim();
  if (!charge) errors.charge = "Describe the charge.";
  else if (runeCount(charge) > CONSULT_LIMITS.chargeMax) errors.charge = `The charge must be at most ${CONSULT_LIMITS.chargeMax} characters.`;
  if (runeCount(input.evidence.trim()) > CONSULT_LIMITS.textMax) errors.evidence = `The evidence must be at most ${CONSULT_LIMITS.textMax} characters.`;
  if (runeCount(input.context.trim()) > CONSULT_LIMITS.textMax) errors.context = `Your situation must be at most ${CONSULT_LIMITS.textMax} characters.`;
  return errors;
}
