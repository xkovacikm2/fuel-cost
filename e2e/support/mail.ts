import { expect } from "@playwright/test";
import { env } from "./env";

export type MailSummary = { ID: string; Subject: string; To: { Address: string }[] };

export async function messagesFor(address: string): Promise<MailSummary[]> {
  const url = `${env.mailpitUrl}/api/v1/search?query=${encodeURIComponent(`to:"${address}"`)}`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Mailpit search failed: ${res.status}`);
  const body = (await res.json()) as { messages: MailSummary[] };
  return body.messages;
}

export async function waitForMessages(address: string, expected: number): Promise<MailSummary[]> {
  await expect.poll(async () => (await messagesFor(address)).length, { timeout: 15_000 }).toBe(expected);
  return messagesFor(address);
}

export async function messageText(id: string): Promise<string> {
  const res = await fetch(`${env.mailpitUrl}/api/v1/message/${id}`);
  if (!res.ok) throw new Error(`Mailpit message failed: ${res.status}`);
  const body = (await res.json()) as { Text: string };
  return body.Text;
}

export async function deleteAllMessages() {
  await fetch(`${env.mailpitUrl}/api/v1/messages`, { method: "DELETE" });
}
