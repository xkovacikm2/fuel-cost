import { exec } from "node:child_process";
import { promisify } from "node:util";
import {
  createAdditionalCost,
  createLead,
  createNotification,
  createRefueling,
  createRule,
  createVehicle,
  query,
} from "../support/db";
import { daysAgo } from "../support/dates";
import { env } from "../support/env";
import { type Account, expect, newAccount, test } from "../support/fixtures";
import { messageText, messagesFor, waitForMessages } from "../support/mail";

const run = promisify(exec);

async function runReminders() {
  await run(env.remindersCmd, { cwd: env.remindersCwd, env: process.env, timeout: 60_000 });
}

async function notifications(userId: number) {
  return query(
    `SELECT n.notification_kind, n.trigger_condition, n.status, n.days_remaining, n.kilometres_remaining,
            l.days_before, l.kilometres_before, n.sent_at IS NOT NULL AS sent
     FROM maintenance_notifications n
     JOIN maintenance_reminder_rules r ON r.id = n.maintenance_reminder_rule_id
     JOIN vehicles v ON v.id = r.vehicle_id
     LEFT JOIN maintenance_reminder_leads l ON l.id = n.maintenance_reminder_lead_id
     WHERE v.user_id = $1 ORDER BY n.id`,
    [userId],
  );
}

test.describe.configure({ mode: "serial" });

test.describe("maintenance reminder job", () => {
  const users: Record<string, Account> = {};

  test.beforeAll(async () => {
    users.advance = await newAccount("advance");
    {
      const v = await createVehicle(users.advance.id, "Octavia");
      const rule = await createRule(v, { kind: "oil_change", intervalDays: 365 });
      await createLead(rule, { daysBefore: 30 });
      await createAdditionalCost(v, { on: daysAgo(340), kind: "oil_change", cost: 80 });
    }

    users.urgent = await newAccount("urgent");
    {
      const v = await createVehicle(users.urgent.id, "Superb");
      const rule = await createRule(v, { kind: "technical_inspection", intervalDays: 365 });
      await createLead(rule, { daysBefore: 60 });
      await createLead(rule, { daysBefore: 30 });
      await createAdditionalCost(v, { on: daysAgo(340), kind: "technical_inspection", cost: 40 });
    }

    users.mileage = await newAccount("mileage");
    {
      const v = await createVehicle(users.mileage.id, "Kodiaq");
      const rule = await createRule(v, { kind: "tire_change", intervalKm: 1000 });
      await createLead(rule, { kilometresBefore: 200 });
      await createAdditionalCost(v, { on: daysAgo(20), kind: "tire_change", cost: 60 });
      await createRefueling(v, { on: daysAgo(20), distance: 5000, amount: 300, cost: 500 });
      await createRefueling(v, { on: daysAgo(10), distance: 600, amount: 40, cost: 60 });
      await createRefueling(v, { on: daysAgo(5), distance: 500, amount: 35, cost: 55 });
    }

    users.silent = await newAccount("silent");
    {
      const v = await createVehicle(users.silent.id, "Fabia");
      await createRule(v, { kind: "oil_change", intervalDays: 10, active: false });
      await createAdditionalCost(v, { on: daysAgo(100), kind: "oil_change", cost: 80 });
      await createRule(v, { kind: "repair", intervalDays: 10 });
    }

    users.cycle = await newAccount("cycle");
    {
      const v = await createVehicle(users.cycle.id, "Kamiq");
      const rule = await createRule(v, { kind: "emissions_inspection", intervalDays: 365 });
      const lead = await createLead(rule, { daysBefore: 30 });
      const old = await createAdditionalCost(v, { on: daysAgo(705), kind: "emissions_inspection", cost: 50 });
      await createNotification({
        ruleId: rule,
        leadId: lead,
        costId: old,
        kind: "advance",
        trigger: "date",
        status: "sent",
        daysRemaining: 25,
        sentAt: "2025-01-01 08:00:00",
      });
      await createAdditionalCost(v, { on: daysAgo(340), kind: "emissions_inspection", cost: 50 });
    }

    await runReminders();
  });

  test("sends a date-based advance notice", async () => {
    const [mail] = await waitForMessages(users.advance.email, 1);
    expect(mail.Subject).toBe("Blíži sa údržba vozidla: Výmena oleja");
    const text = await messageText(mail.ID);
    expect(text).toContain("Blíži sa údržba Výmena oleja pre vozidlo Octavia.");
    expect(text).toContain(`Posledný zaznamenaný náklad bol ${daysAgo(340)}.`);
    expect(text).toContain("Do termínu zostáva 25 dní.");
    expect(text).not.toContain("Do kilometrového limitu");
    expect(await notifications(users.advance.id)).toEqual([
      {
        notification_kind: 0,
        trigger_condition: 0,
        status: 1,
        days_remaining: 25,
        kilometres_remaining: null,
        days_before: 30,
        kilometres_before: null,
        sent: true,
      },
    ]);
  });

  test("sends only the most urgent eligible lead and skips the others", async () => {
    const [mail] = await waitForMessages(users.urgent.email, 1);
    expect(mail.Subject).toBe("Blíži sa údržba vozidla: Technická kontrola");
    const rows = await notifications(users.urgent.id);
    expect(rows.map((r) => [r.days_before, r.status, r.sent])).toEqual(
      expect.arrayContaining([
        [30, 1, true],
        [60, 2, false],
      ]),
    );
    expect(rows).toHaveLength(2);
  });

  test("sends a due notice when mileage is exceeded, ignoring the refueling on the service day", async () => {
    const [mail] = await waitForMessages(users.mileage.email, 1);
    expect(mail.Subject).toBe("Údržba vozidla je potrebná: Prezutie pneumatík");
    const text = await messageText(mail.ID);
    expect(text).toContain("Údržba Prezutie pneumatík pre vozidlo Kodiaq je potrebná.");
    expect(text).toContain("Do kilometrového limitu zostáva -100 km.");
    expect(text).not.toContain("Do termínu zostáva");
    const rows = await notifications(users.mileage.id);
    expect(rows).toEqual([
      expect.objectContaining({ notification_kind: 0, status: 2, kilometres_before: "200.00", sent: false }),
      expect.objectContaining({
        notification_kind: 1,
        trigger_condition: 1,
        status: 1,
        days_remaining: null,
        kilometres_remaining: "-100.00",
        sent: true,
      }),
    ]);
  });

  test("ignores inactive reminders and reminders without a matching cost", async () => {
    expect(await messagesFor(users.silent.email)).toHaveLength(0);
    expect(await notifications(users.silent.id)).toHaveLength(0);
  });

  test("starts a new cycle after a newer matching cost", async () => {
    const [mail] = await waitForMessages(users.cycle.email, 1);
    expect(mail.Subject).toBe("Blíži sa údržba vozidla: Emisná kontrola");
    expect(await messageText(mail.ID)).toContain(`Posledný zaznamenaný náklad bol ${daysAgo(340)}.`);
  });

  test("does not notify twice on the next run", async () => {
    await runReminders();
    await new Promise((r) => setTimeout(r, 1_000));
    for (const key of ["advance", "urgent", "mileage", "cycle"]) {
      expect(await messagesFor(users[key].email), key).toHaveLength(1);
    }
    expect(await notifications(users.advance.id)).toHaveLength(1);
  });
});
