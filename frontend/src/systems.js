// What each system lets the agent work with, in plain words.
const ABOUT = {
  crm: { label: "CRM", does: "Leads and their sales status" },
  ticketing: { label: "Ticketing", does: "Customer support tickets" },
  billing: { label: "Billing", does: "Invoices, refunds and credits" },
  calendar: { label: "Calendar", does: "Meetings and availability" },
  knowledge: { label: "Knowledge", does: "Policies, pricing and FAQs" },
  notifications: { label: "Notifications", does: "Slack, email and SMS" },
};

// A check slower than this is shown as "slow".
export const SLOW_MS = 250;

// Number of checks the backend keeps per server.
export const HISTORY = 60;

export const about = (name) => ABOUT[name] || { label: name, does: "" };

const neverChecked = (t) => !t || t.startsWith("0001");

/** @returns {"idle" | "up" | "slow" | "down"} */
export function stateOf(server) {
  if (neverChecked(server.checked_at)) return "idle";
  if (!server.healthy) return "down";
  return server.latency_ms > SLOW_MS ? "slow" : "up";
}

export const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

export function joinNames(names) {
  if (names.length <= 1) return names.join("");
  return `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
}

export { neverChecked };
