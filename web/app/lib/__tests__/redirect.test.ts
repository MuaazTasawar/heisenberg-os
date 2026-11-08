import { describe, expect, it } from "vitest";
import { isPublicPath, loginUrl, safeNextPath } from "../redirect";

describe("safeNextPath", () => {
  it("keeps paths on this site, with their query and hash", () => {
    expect(safeNextPath("/dashboard/empire")).toBe("/dashboard/empire");
    expect(safeNextPath("/dashboard/legal?case=5#top")).toBe("/dashboard/legal?case=5#top");
  });

  it("refuses everything that could leave the site", () => {
    for (const evil of [
      "https://evil.example/", "http://evil.example", "//evil.example", "///evil.example",
      "/\\evil.example", "\\\\evil.example", "javascript:alert(1)", "evil.example", "dashboard",
      "/dashboard\r\nSet-Cookie: a=b", "/dash\u0000board", "/\tevil", " /dashboard",
    ]) {
      expect(safeNextPath(evil), JSON.stringify(evil)).toBe("/dashboard");
    }
  });

  it("falls back for missing, empty or oversized values", () => {
    expect(safeNextPath(undefined)).toBe("/dashboard");
    expect(safeNextPath(null)).toBe("/dashboard");
    expect(safeNextPath("")).toBe("/dashboard");
    expect(safeNextPath("/" + "a".repeat(600))).toBe("/dashboard");
    expect(safeNextPath(undefined, "/")).toBe("/");
  });

  it("never sends the player back to the login pages", () => {
    expect(safeNextPath("/login")).toBe("/dashboard");
    expect(safeNextPath("/register?x=1")).toBe("/dashboard");
  });

  it("keeps encoded slashes as a harmless path", () => {
    expect(safeNextPath("/%2F%2Fevil.example")).toBe("/%2F%2Fevil.example");
  });
});

describe("loginUrl", () => {
  it("returns to the page afterwards", () => {
    expect(loginUrl("/dashboard/empire")).toBe("/login?next=%2Fdashboard%2Fempire");
    expect(loginUrl("/dashboard/legal?case=5")).toBe("/login?next=%2Fdashboard%2Flegal%3Fcase%3D5");
  });

  it("is plain /login when there is nowhere sensible to return to", () => {
    expect(loginUrl()).toBe("/login");
    expect(loginUrl("/")).toBe("/login");
    expect(loginUrl("https://evil.example")).toBe("/login");
    expect(loginUrl("/login")).toBe("/login");
  });
});

describe("isPublicPath", () => {
  it("knows the pages that need no login", () => {
    expect(["/", "/login", "/register"].every(isPublicPath)).toBe(true);
    expect(isPublicPath("/dashboard")).toBe(false);
    expect(isPublicPath("/login/extra")).toBe(false);
  });
});
