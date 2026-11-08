import { AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig } from "axios";
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { api, ApiError, API_URL, errorMessage, http, isUnauthorized, setUnauthorizedHandler, toApiError } from "../api";

type Reply = { status?: number; data?: unknown; headers?: Record<string, string> };

/** Replaces the network: records every request and answers with whatever `reply` returns. */
function fakeServer(reply: (config: InternalAxiosRequestConfig) => Reply | "offline" | "timeout") {
  const requests: InternalAxiosRequestConfig[] = [];
  const adapter: AxiosAdapter = async (config) => {
    requests.push(config);
    const answer = reply(config);
    if (answer === "offline") throw new AxiosError("Network Error", "ERR_NETWORK", config);
    if (answer === "timeout") throw new AxiosError("timeout of 15000ms exceeded", "ECONNABORTED", config);
    const response = { data: answer.data ?? {}, status: answer.status ?? 200, statusText: "", headers: answer.headers ?? {}, config };
    if (response.status >= 400) throw new AxiosError("Request failed", "ERR_BAD_REQUEST", config, null, response);
    return response;
  };
  http.defaults.adapter = adapter;
  return requests;
}

const ok = (data: unknown): Reply => ({ data: { success: true, message: "ok", data } });
const fail = (status: number, error?: string, headers?: Record<string, string>): Reply => ({
  status,
  data: error === undefined ? "<html>bad gateway</html>" : { success: false, message: "", error },
  headers,
});

const originalAdapter = http.defaults.adapter;
afterEach(() => {
  http.defaults.adapter = originalAdapter;
  setUnauthorizedHandler(null);
});

describe("how requests are made", () => {
  it("sends the session cookie and never an Authorization header or a stored token", async () => {
    const storage = vi.spyOn(Storage.prototype, "getItem");
    const requests = fakeServer(() => ok({ id: 1 }));
    await api.empire.get();

    const config = requests[0];
    expect(config.withCredentials).toBe(true);
    expect(config.timeout).toBeGreaterThan(0);
    expect(config.baseURL).toBe(API_URL);
    expect(config.headers.has("Authorization")).toBe(false);
    expect(storage).not.toHaveBeenCalled();
  });

  it("uses the documented API address by default", () => {
    expect(API_URL).toBe("http://localhost:8080/api/v1");
  });

  it("calls the right method and path with the right body for every endpoint", async () => {
    const requests = fakeServer(() => ok({}));
    await api.auth.register({ username: "walter", email: "w@example.com", password: "iamthedanger" });
    await api.auth.login({ email: "w@example.com", password: "iamthedanger" });
    await api.auth.me();
    await api.auth.logout();
    await api.game.state();
    await api.empire.cook(8);
    await api.empire.claimTerritory();
    await api.empire.advice();
    await api.empire.advice(6.5);
    await api.empire.frontTypes();
    await api.empire.fronts();
    await api.empire.openFront({ name: "Los Pollos", type: "restaurant" });
    await api.empire.closeFront(3);
    await api.launder.send({ amount: 60000, business_id: 3 });
    await api.launder.transactions();
    await api.launder.transactions({ limit: 20, offset: 40, flagged: true });
    await api.dea.threat();
    await api.dea.history();
    await api.dea.history(60);
    await api.dea.evidence();
    await api.legal.consult({ charge: "Fraud", evidence: "A receipt" });
    await api.legal.cases({ limit: 10, offset: 10 });
    await api.legal.case(5);
    await api.stats.get();

    const seen = requests.map((r) => ({
      call: `${(r.method ?? "get").toUpperCase()} ${r.url}`,
      body: typeof r.data === "string" ? JSON.parse(r.data) : r.data,
      params: r.params,
    }));
    expect(seen).toEqual([
      { call: "POST /auth/register", body: { username: "walter", email: "w@example.com", password: "iamthedanger" }, params: undefined },
      { call: "POST /auth/login", body: { email: "w@example.com", password: "iamthedanger" }, params: undefined },
      { call: "GET /auth/me", body: undefined, params: undefined },
      { call: "POST /auth/logout", body: undefined, params: undefined },
      { call: "GET /game/state", body: undefined, params: undefined },
      { call: "POST /empire/cook", body: { effort: 8 }, params: undefined },
      { call: "POST /empire/territory", body: {}, params: undefined },
      { call: "GET /empire/advice", body: undefined, params: undefined },
      { call: "GET /empire/advice", body: undefined, params: { effort: 6.5 } },
      { call: "GET /empire/front-types", body: undefined, params: undefined },
      { call: "GET /empire/fronts", body: undefined, params: undefined },
      { call: "POST /empire/front", body: { name: "Los Pollos", type: "restaurant" }, params: undefined },
      { call: "DELETE /empire/front/3", body: undefined, params: undefined },
      { call: "POST /launder", body: { amount: 60000, business_id: 3 }, params: undefined },
      { call: "GET /transactions", body: undefined, params: {} },
      { call: "GET /transactions", body: undefined, params: { limit: 20, offset: 40, flagged: true } },
      { call: "GET /dea/threat", body: undefined, params: undefined },
      { call: "GET /dea/history", body: undefined, params: undefined },
      { call: "GET /dea/history", body: undefined, params: { limit: 60 } },
      { call: "GET /dea/evidence", body: undefined, params: undefined },
      { call: "POST /legal/consult", body: { charge: "Fraud", evidence: "A receipt" }, params: undefined },
      { call: "GET /legal/cases", body: undefined, params: { limit: 10, offset: 10 } },
      { call: "GET /legal/cases/5", body: undefined, params: undefined },
      { call: "GET /stats", body: undefined, params: undefined },
    ]);
  });

  it("hands back the data of the envelope", async () => {
    fakeServer(() => ok({ id: 7, username: "walter" }));
    expect(await api.auth.me()).toEqual({ id: 7, username: "walter" });
  });
});

describe("errors", () => {
  it("shows the server's own message", async () => {
    fakeServer(() => fail(400, "password must be at least 8 characters"));
    const error = await api.auth.register({ username: "w", email: "x", password: "1" }).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 400, message: "password must be at least 8 characters" });
  });

  it("explains failures that have no message of their own", async () => {
    const table: [number, string][] = [
      [401, "Please sign in again."], [403, "You are not allowed to do that."], [404, "We could not find that."],
      [409, "That conflicts with something that already exists."], [413, "That request is too large."],
      [418, "The request failed (HTTP 418)."],
      [500, "The server had a problem. Please try again."], [502, "The server had a problem. Please try again."],
    ];
    for (const [status, message] of table) {
      fakeServer(() => fail(status));
      await expect(api.stats.get(), `HTTP ${status}`).rejects.toMatchObject({ status, message });
    }
  });

  it("says how long to wait when rate limited", async () => {
    fakeServer(() => fail(429, "rate limit exceeded", { "retry-after": "17" }));
    await expect(api.stats.get()).rejects.toMatchObject({ status: 429, retryAfter: 17, message: "Too many requests. Try again in 17 seconds." });
    fakeServer(() => fail(429, undefined, { "retry-after": "1" }));
    await expect(api.stats.get()).rejects.toMatchObject({ message: "Too many requests. Try again in 1 second." });
    fakeServer(() => fail(429, undefined, { "retry-after": "soon" }));
    await expect(api.stats.get()).rejects.toMatchObject({ retryAfter: undefined, message: "Too many requests. Please wait a moment." });
  });

  it("tells offline from slow", async () => {
    fakeServer(() => "offline");
    await expect(api.stats.get()).rejects.toMatchObject({ status: 0, message: "Cannot reach the server. Check your connection and try again." });
    fakeServer(() => "timeout");
    await expect(api.stats.get()).rejects.toMatchObject({ status: 0, message: "The server took too long to answer. Please try again." });
  });

  it("only ever shows ApiError messages, never raw internals", () => {
    expect(errorMessage(new ApiError("Readable.", 400))).toBe("Readable.");
    expect(errorMessage(new Error("TypeError: x is undefined at line 42"))).toBe("Something went wrong. Please try again.");
    expect(errorMessage("oops", "custom")).toBe("custom");
    expect(errorMessage(undefined)).toBe("Something went wrong. Please try again.");
    expect(toApiError(new Error("boom"))).toMatchObject({ status: 0, message: "Something went wrong. Please try again." });
    const existing = new ApiError("kept", 404);
    expect(toApiError(existing)).toBe(existing);
  });

  it("recognises a lost session", () => {
    expect(isUnauthorized(new ApiError("x", 401))).toBe(true);
    expect(isUnauthorized(new ApiError("x", 403))).toBe(false);
    expect(isUnauthorized(new Error("x"))).toBe(false);
  });
});

describe("a lost session", () => {
  let handler: Mock<() => void>;
  beforeEach(() => {
    handler = vi.fn<() => void>();
    setUnauthorizedHandler(handler);
  });

  it("is reported once when a protected request gets 401", async () => {
    fakeServer(() => fail(401, "authentication required"));
    await expect(api.empire.get()).rejects.toMatchObject({ status: 401 });
    await expect(api.legal.case(1)).rejects.toMatchObject({ status: 401 });
    expect(handler).toHaveBeenCalledTimes(2);
  });

  it("is not reported for the sign-in calls, where 401 is an ordinary answer", async () => {
    fakeServer(() => fail(401, "invalid email or password"));
    await expect(api.auth.login({ email: "a@b.co", password: "x" })).rejects.toMatchObject({ message: "invalid email or password" });
    await expect(api.auth.me()).rejects.toMatchObject({ status: 401 });
    await expect(api.auth.register({ username: "a", email: "a@b.co", password: "x" })).rejects.toMatchObject({ status: 401 });
    await expect(api.auth.logout()).rejects.toMatchObject({ status: 401 });
    expect(handler).not.toHaveBeenCalled();
  });

  it("is not reported for other failures", async () => {
    fakeServer(() => fail(403, "nope"));
    await expect(api.empire.get()).rejects.toMatchObject({ status: 403 });
    fakeServer(() => "offline");
    await expect(api.empire.get()).rejects.toMatchObject({ status: 0 });
    expect(handler).not.toHaveBeenCalled();
  });

  it("does nothing when nobody listens", async () => {
    setUnauthorizedHandler(null);
    fakeServer(() => fail(401, "authentication required"));
    await expect(api.empire.get()).rejects.toMatchObject({ status: 401 });
  });
});
