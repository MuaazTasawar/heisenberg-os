const PUBLIC_PATHS = ["/", "/login", "/register"];

/** True for pages that need no login. */
export function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATHS.includes(pathname);
}

/**
 * Returns `raw` only when it is a safe place to send the player after login: a path on this
 * site. Anything else (another site, a protocol-relative "//host", a backslash trick, control
 * characters, the login pages themselves) gives `fallback`, so "?next=" can never be used as an
 * open redirect.
 */
export function safeNextPath(raw: string | null | undefined, fallback = "/dashboard"): string {
  if (typeof raw !== "string" || raw === "" || raw.length > 512) return fallback;
  if (!raw.startsWith("/") || raw.startsWith("//") || raw.includes("\\")) return fallback;
  if (/[\u0000-\u001f\u007f]/.test(raw)) return fallback;

  let parsed: URL;
  try {
    parsed = new URL(raw, "http://site.invalid");
  } catch {
    return fallback;
  }
  if (parsed.origin !== "http://site.invalid") return fallback;
  if (isPublicPath(parsed.pathname) && parsed.pathname !== "/") return fallback;
  return parsed.pathname + parsed.search + parsed.hash;
}

/** The login page with a "next" that comes back to `path` afterwards. */
export function loginUrl(path?: string): string {
  const next = safeNextPath(path, "");
  return next === "" || next === "/" ? "/login" : `/login?next=${encodeURIComponent(next)}`;
}
