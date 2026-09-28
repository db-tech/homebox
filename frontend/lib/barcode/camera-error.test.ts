import { describe, expect, test } from "vitest";
import { cameraErrorDetail, classifyCameraError } from "./camera-error";

function domError(name: string, message = "x"): Error {
  const e = new Error(message);
  e.name = name;
  return e;
}

describe("classifyCameraError", () => {
  test("a refused permission", () => {
    expect(classifyCameraError(domError("NotAllowedError"))).toBe("denied");
    // Older browsers still use the pre-spec name.
    expect(classifyCameraError(domError("PermissionDeniedError"))).toBe("denied");
  });

  test("no camera at all", () => {
    expect(classifyCameraError(domError("NotFoundError"))).toBe("missing");
    expect(classifyCameraError(domError("DevicesNotFoundError"))).toBe("missing");
  });

  test("another app is holding it", () => {
    // The case that is invisible from inside the browser, and the reason this
    // module exists: a kiosk app watching for motion owns the camera.
    expect(classifyCameraError(domError("NotReadableError"))).toBe("in_use");
    expect(classifyCameraError(domError("TrackStartError"))).toBe("in_use");
    expect(classifyCameraError(domError("AbortError"))).toBe("in_use");
  });

  test("the requested resolution is not available", () => {
    expect(classifyCameraError(domError("OverconstrainedError"))).toBe("unsupported_constraints");
  });

  test("anything else is unknown rather than guessed at", () => {
    expect(classifyCameraError(domError("WeirdError"))).toBe("unknown");
    expect(classifyCameraError("not an error")).toBe("unknown");
    expect(classifyCameraError(undefined)).toBe("unknown");
  });
});

describe("cameraErrorDetail", () => {
  test("carries the browser's own words", () => {
    expect(cameraErrorDetail(domError("NotReadableError", "Could not start video source"))).toBe(
      "NotReadableError: Could not start video source"
    );
  });

  test("and nothing when there are none", () => {
    expect(cameraErrorDetail("nope")).toBe("");
    expect(cameraErrorDetail(null)).toBe("");
  });
});
