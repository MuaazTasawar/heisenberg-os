import axios, { AxiosError, type AxiosResponse } from "axios";
import type {
  CaseQuery,
  CookAdvice,
  CookResult,
  ConsultInput,
  DeaThreat,
  Empire,
  Envelope,
  EvidenceFile,
  FrontBusiness,
  FrontType,
  GameState,
  LaunderInput,
  LaunderResult,
  LedgerQuery,
  LegalCase,
  LoginInput,
  LoginResult,
  Page,
  RegisterInput,
  Stats,
  Transaction,
  User,
} from "./types";

export const API_URL = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1").replace(/\/+$/, "");

// ----------------------------------------------------------------------------- errors

/** Every failed request becomes an ApiError whose message is safe to show to the player. */
export class ApiError extends Error {
  /** HTTP status; 0 when there was no answer at all (offline, timeout). */
  readonly status: number;
  /** Seconds to wait, for 429 answers. */
  readonly retryAfter?: number;

  constructor(message: string, status: number, retryAfter?: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.retryAfter = retryAfter;
  }
}

const STATUS_MESSAGES: Record<number, string> = {
  400: "That request was not valid.",
  401: "Please sign in again.",
  403: "You are not allowed to do that.",
  404: "We could not find that.",
  409: "That conflicts with something that already exists.",
  413: "That request is too large.",
  415: "That request was not understood.",
};

/** Turns anything thrown by axios into an ApiError. */
export function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error;

  if (axios.isAxiosError(error)) {
    const response = error.response;
    if (!response) {
      if (error.code === "ECONNABORTED" || error.code === "ETIMEDOUT") {
        return new ApiError("The server took too long to answer. Please try again.", 0);
      }
      return new ApiError("Cannot reach the server. Check your connection and try again.", 0);
    }

    const status = response.status;
    const body = response.data as Partial<Envelope<unknown>> | string | undefined;
    const fromServer = typeof body === "object" && body !== null && typeof body.error === "string" ? body.error.trim() : "";

    if (status === 429) {
      const retryAfter = Number.parseInt(String(response.headers?.["retry-after"] ?? ""), 10);
      const seconds = Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : undefined;
      const wait = seconds ? ` Try again in ${seconds} second${seconds === 1 ? "" : "s"}.` : " Please wait a moment.";
      return new ApiError(`Too many requests.${wait}`, 429, seconds);
    }
    if (fromServer) return new ApiError(fromServer, status);
    if (status >= 500) return new ApiError("The server had a problem. Please try again.", status);
    return new ApiError(STATUS_MESSAGES[status] ?? `The request failed (HTTP ${status}).`, status);
  }

  return new ApiError("Something went wrong. Please try again.", 0);
}

/** A message for the player; anything that is not an ApiError gets `fallback`. */
export function errorMessage(error: unknown, fallback = "Something went wrong. Please try again."): string {
  return error instanceof ApiError ? error.message : fallback;
}

export function isUnauthorized(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401;
}

// -------------------------------------------------------------------------- the client

/**
 * Called when a request that needs a login is answered with 401 (the session ended). The
 * providers register a handler that clears the caches and goes to the login page.
 */
let unauthorizedHandler: (() => void) | null = null;

export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler;
}

// For these a 401 is an ordinary answer ("wrong password", "not signed in"), not a lost session.
const AUTH_PATHS = /^\/auth\/(login|register|logout|me)\/?$/;

/**
 * The player's session lives in an httpOnly cookie that the server sets at login, so the
 * browser sends it by itself (withCredentials) and no token is ever kept in JavaScript.
 */
export const http = axios.create({
  baseURL: API_URL,
  withCredentials: true,
  timeout: 15_000,
  headers: { "Content-Type": "application/json" },
});

http.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    const apiError = toApiError(error);
    const url = axios.isAxiosError(error) ? (error as AxiosError).config?.url ?? "" : "";
    if (apiError.status === 401 && !AUTH_PATHS.test(url)) unauthorizedHandler?.();
    return Promise.reject(apiError);
  },
);

/** @deprecated The pages written for the old API import this; the new pages use `api`. */
export default http;

async function unwrap<T>(request: Promise<AxiosResponse<Envelope<T>>>): Promise<T> {
  const response = await request;
  return response.data.data as T;
}

// -------------------------------------------------------------------------- endpoints

export const api = {
  auth: {
    register: (input: RegisterInput) => unwrap<User>(http.post("/auth/register", input)),
    login: (input: LoginInput) => unwrap<LoginResult>(http.post("/auth/login", input)),
    logout: async () => {
      await http.post("/auth/logout");
    },
    me: () => unwrap<User>(http.get("/auth/me")),
  },

  game: {
    state: () => unwrap<GameState>(http.get("/game/state")),
  },

  empire: {
    get: () => unwrap<Empire>(http.get("/empire")),
    cook: (effort: number) => unwrap<CookResult>(http.post("/empire/cook", { effort })),
    claimTerritory: () => unwrap<Empire>(http.post("/empire/territory", {})),
    advice: (effort?: number) =>
      unwrap<CookAdvice>(http.get("/empire/advice", { params: effort === undefined ? undefined : { effort } })),
    frontTypes: () => unwrap<FrontType[]>(http.get("/empire/front-types")),
    fronts: () => unwrap<FrontBusiness[]>(http.get("/empire/fronts")),
    openFront: (input: { name: string; type: string }) => unwrap<FrontBusiness>(http.post("/empire/front", input)),
    closeFront: (id: number) => unwrap<FrontBusiness>(http.delete(`/empire/front/${id}`)),
  },

  launder: {
    send: (input: LaunderInput) => unwrap<LaunderResult>(http.post("/launder", input)),
    transactions: (query: LedgerQuery = {}) => unwrap<Page<Transaction>>(http.get("/transactions", { params: query })),
  },

  dea: {
    threat: () => unwrap<DeaThreat>(http.get("/dea/threat")),
    history: (limit?: number) => unwrap<DeaThreat[]>(http.get("/dea/history", { params: limit === undefined ? undefined : { limit } })),
    evidence: () => unwrap<EvidenceFile>(http.get("/dea/evidence")),
  },

  legal: {
    consult: (input: ConsultInput) => unwrap<LegalCase>(http.post("/legal/consult", input)),
    cases: (query: CaseQuery = {}) => unwrap<Page<LegalCase>>(http.get("/legal/cases", { params: query })),
    case: (id: number) => unwrap<LegalCase>(http.get(`/legal/cases/${id}`)),
  },

  stats: {
    get: () => unwrap<Stats>(http.get("/stats")),
  },
};
