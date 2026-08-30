import { describe, expect, test } from "vitest";
import { classifyScan } from "../lib/barcode/scan-target";

const HOME = "https://homebox.dbiber.de";

describe("Homebox labels", () => {
  test("an item link is followed", () => {
    expect(classifyScan(`${HOME}/item/0198f0d5-1a2b-4c3d-9e8f-112233445566`, HOME)).toEqual({
      kind: "internal",
      path: "/item/0198f0d5-1a2b-4c3d-9e8f-112233445566",
    });
  });

  test("an asset label is followed", () => {
    expect(classifyScan(`${HOME}/a/000-001`, HOME)).toEqual({ kind: "internal", path: "/a/000-001" });
  });

  test("a nested path survives", () => {
    expect(classifyScan(`${HOME}/item/abc-123/maintenance`, HOME)).toEqual({
      kind: "internal",
      path: "/item/abc-123/maintenance",
    });
  });

  test("a query string is dropped, the path is kept", () => {
    expect(classifyScan(`${HOME}/a/000-001?utm=print`, HOME)).toEqual({ kind: "internal", path: "/a/000-001" });
  });
});

describe("codes belonging to somebody else", () => {
  // The case that sent every scan back to the start page: packaging carries a
  // marketing QR code next to the barcode, and its path meant nothing here.
  test("a manufacturer URL is not followed", () => {
    const got = classifyScan("https://www.mutti-parma.com/produkte/passata", HOME);
    expect(got.kind).toBe("foreign");
  });

  test("a plain domain is not followed", () => {
    expect(classifyScan("https://example.com", HOME).kind).toBe("foreign");
  });

  test("a lookalike host is not followed", () => {
    expect(classifyScan("https://homebox.dbiber.de.evil.test/item/1", HOME).kind).toBe("foreign");
  });

  test("the same host on another scheme is not followed", () => {
    expect(classifyScan("http://homebox.dbiber.de/item/1", HOME).kind).toBe("foreign");
  });

  test("the same host on another port is not followed", () => {
    expect(classifyScan("https://homebox.dbiber.de:8443/item/1", HOME).kind).toBe("foreign");
  });

  // Our own root carries no information worth navigating for.
  test("our own start page is treated as foreign rather than navigated to", () => {
    expect(classifyScan(`${HOME}/`, HOME).kind).toBe("foreign");
    expect(classifyScan(HOME, HOME).kind).toBe("foreign");
  });

  test("a non-http URL is not followed", () => {
    expect(classifyScan("javascript:alert(1)", HOME).kind).toBe("foreign");
    expect(classifyScan("file:///etc/passwd", HOME).kind).toBe("foreign");
  });
});

describe("product codes", () => {
  test("an EAN is a code", () => {
    expect(classifyScan("4001234567890", HOME)).toEqual({ kind: "code", value: "4001234567890" });
  });

  test("surrounding whitespace is trimmed", () => {
    expect(classifyScan("  4001234567890  ", HOME)).toEqual({ kind: "code", value: "4001234567890" });
  });

  test("a non-URL text payload is a code, not a navigation", () => {
    expect(classifyScan("ABC-123-XYZ", HOME).kind).toBe("code");
  });
});

describe("path sanitising", () => {
  test("characters outside the safe set are stripped", () => {
    const got = classifyScan(`${HOME}/item/abc%20def`, HOME);
    expect(got.kind).toBe("internal");
    if (got.kind === "internal") {
      expect(got.path).not.toContain("%");
      expect(got.path).not.toContain(" ");
    }
  });

  test("a path that sanitises down to nothing is not navigated to", () => {
    // Nothing but characters outside the safe set, so only the slash survives.
    expect(classifyScan(`${HOME}/...`, HOME).kind).toBe("foreign");
  });

  // Percent escapes lose their percent sign and become harmless nonsense rather
  // than a decoded path. Worth pinning so nobody assumes decoding happens.
  test("percent escapes are stripped, not decoded", () => {
    const got = classifyScan(`${HOME}/item/a%2Fb`, HOME);
    expect(got.kind).toBe("internal");
    if (got.kind === "internal") {
      expect(got.path).toBe("/item/a2Fb");
    }
  });
});
