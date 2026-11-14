import { describe, expect, it } from "vitest";
import { byteLength, firstInvalid, LIMITS, runeCount, toRegisterInput, validateLogin, validateRegistration } from "../validation";

const valid = { username: "walter", email: "walter@example.com", alias: "", password: "iamthedanger", confirm: "iamthedanger" };

describe("validateLogin", () => {
  it("accepts a plausible email and any non-empty password", () => {
    expect(validateLogin({ email: "walter@example.com", password: "x" })).toEqual({});
  });

  it("asks for what is missing and rejects a malformed email", () => {
    expect(validateLogin({ email: "", password: "" })).toEqual({ email: "Enter your email address.", password: "Enter your password." });
    expect(validateLogin({ email: "   ", password: "x" }).email).toBe("Enter your email address.");
    for (const bad of ["walter", "walter@", "@example.com", "walter@example", "wal ter@example.com", "a@b@c.com"]) {
      expect(validateLogin({ email: bad, password: "x" }).email, bad).toBe("Enter a valid email address.");
    }
  });

  it("does not judge the password, which only the server can check", () => {
    expect(validateLogin({ email: "a@b.co", password: "1" }).password).toBeUndefined();
  });
});

describe("validateRegistration", () => {
  it("accepts a good form", () => {
    expect(validateRegistration(valid)).toEqual({});
    expect(validateRegistration({ ...valid, alias: "Heisenberg", username: "Walter.White-1_" })).toEqual({});
  });

  it("checks the username the way the server does, after trimming and lower-casing", () => {
    expect(validateRegistration({ ...valid, username: "" }).username).toBe("Choose a username.");
    for (const bad of ["ab", "a".repeat(33), "walter white", "walter!", "w\u00e4lter", "walter@home"]) {
      expect(validateRegistration({ ...valid, username: bad }).username, bad).toMatch(/^Username must be 3-32 characters/);
    }
    for (const good of ["abc", "a".repeat(32), "  WALTER  ", "w.w-1_x"]) {
      expect(validateRegistration({ ...valid, username: good }).username, good).toBeUndefined();
    }
  });

  it("checks the email", () => {
    expect(validateRegistration({ ...valid, email: "" }).email).toBe("Enter your email address.");
    expect(validateRegistration({ ...valid, email: "nobody" }).email).toBe("Enter a valid email address.");
    expect(validateRegistration({ ...valid, email: `${"a".repeat(250)}@b.co` }).email).toBe("Enter a valid email address.");
    expect(validateRegistration({ ...valid, email: "  Walter@Example.COM " }).email).toBeUndefined();
  });

  it("counts password characters, not bytes, for the minimum, and bytes for the maximum", () => {
    expect(validateRegistration({ ...valid, password: "1234567", confirm: "1234567" }).password).toBe("Password must be at least 8 characters.");
    expect(validateRegistration({ ...valid, password: "12345678", confirm: "12345678" }).password).toBeUndefined();

    const exactly72 = "a".repeat(LIMITS.passwordMaxBytes);
    expect(validateRegistration({ ...valid, password: exactly72, confirm: exactly72 }).password).toBeUndefined();
    const tooLong = "a".repeat(LIMITS.passwordMaxBytes + 1);
    expect(validateRegistration({ ...valid, password: tooLong, confirm: tooLong }).password).toMatch(/at most 72 bytes/);

    // 20 emoji are 20 characters but 80 bytes
    const emoji = "\u{1F575}".repeat(20);
    expect(runeCount(emoji)).toBe(20);
    expect(byteLength(emoji)).toBe(80);
    expect(validateRegistration({ ...valid, password: emoji, confirm: emoji }).password).toMatch(/at most 72 bytes/);
  });

  it("does not trim the password", () => {
    expect(validateRegistration({ ...valid, password: "       a", confirm: "       a" }).password).toBeUndefined();
    expect(validateRegistration({ ...valid, password: "abcdefg ", confirm: "abcdefg" }).confirm).toBe("The passwords do not match.");
  });

  it("limits the alias but does not require one", () => {
    expect(validateRegistration({ ...valid, alias: "a".repeat(64) }).alias).toBeUndefined();
    expect(validateRegistration({ ...valid, alias: "a".repeat(65) }).alias).toBe("Alias must be at most 64 characters.");
    expect(validateRegistration({ ...valid, alias: "\u{1F575}".repeat(64) }).alias).toBeUndefined();
  });

  it("reports every problem at once", () => {
    const errors = validateRegistration({ username: "a", email: "x", alias: "", password: "1", confirm: "2" });
    expect(Object.keys(errors).sort()).toEqual(["confirm", "email", "password", "username"]);
  });
});

describe("toRegisterInput", () => {
  it("sends the same normalised values the server would store, and no empty alias", () => {
    expect(toRegisterInput({ ...valid, username: " Walter ", email: " Walter@Example.COM ", alias: "   " })).toEqual({
      username: "walter", email: "walter@example.com", password: "iamthedanger",
    });
    expect(toRegisterInput({ ...valid, alias: "  Heisenberg " })).toMatchObject({ alias: "Heisenberg" });
  });

  it("never touches the password", () => {
    expect(toRegisterInput({ ...valid, password: "  Spaces And CAPS  ", confirm: "  Spaces And CAPS  " }).password).toBe("  Spaces And CAPS  ");
  });
});

describe("firstInvalid", () => {
  it("picks the first field in the given order", () => {
    expect(firstInvalid({ password: "x", email: "y" }, ["email", "password"] as const)).toBe("email");
    expect(firstInvalid({}, ["email", "password"] as const)).toBeUndefined();
  });
});
