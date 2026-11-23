import type { Vehicle } from "./types";

export class RequestError extends Error {
  constructor(message: string, readonly status: number) { super(message); }
}

export async function request<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (response.status === 204) return undefined as T;
  const data = await response.json();
  if (!response.ok) throw new RequestError(data.error ?? "Požiadavka zlyhala.", response.status);
  return data as T;
}

export function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : "Požiadavka zlyhala.";
}

export async function fetchVehicles(): Promise<Vehicle[]> {
  return (await request<{ vehicles: Vehicle[] }>("/api/vehicles")).vehicles;
}
