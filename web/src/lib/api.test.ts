import { afterEach, describe, expect, it, vi } from "vitest";
import { RequestError, messageOf, request } from "./api";
import { jsonResponse, noContent, stubApi } from "../testing";

describe("request", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("sends JSON bodies with the session cookie and parses the answer", async () => {
    const fetchMock = stubApi({ "POST /api/session": { user: { id: 1 } } });
    await expect(request("/api/session", "POST", { email_address: "a@b.sk" })).resolves.toEqual({ user: { id: 1 } });
    expect(fetchMock).toHaveBeenCalledWith("/api/session", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email_address: "a@b.sk" }),
    });
  });

  it("resolves to undefined when there is no content", async () => {
    stubApi({ "DELETE /api/vehicles/1": noContent() });
    await expect(request("/api/vehicles/1", "DELETE")).resolves.toBeUndefined();
  });

  it("turns an error answer into a RequestError carrying the API message and status", async () => {
    stubApi({ "/api/me": jsonResponse({ error: "Prihláste sa." }, 401), "/api/boom": jsonResponse({}, 500) });
    await expect(request("/api/me")).rejects.toMatchObject({ message: "Prihláste sa.", status: 401 });
    await expect(request("/api/me")).rejects.toBeInstanceOf(RequestError);
    await expect(request("/api/boom")).rejects.toMatchObject({ message: "Požiadavka zlyhala.", status: 500 });
  });
});

describe("messageOf", () => {
  it("uses an error's message and falls back to a generic one", () => {
    expect(messageOf(new Error("Chyba"))).toBe("Chyba");
    expect(messageOf("nie je chyba")).toBe("Požiadavka zlyhala.");
  });
});
