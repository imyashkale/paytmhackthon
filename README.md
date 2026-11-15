# AI Teammates

> AI that doesn't just respond — it gets the job done.

Custom **MCP servers in Golang** that let any MCP-compatible agent take real actions in internal systems for **Sales** and **Customer Service** workflows.

## Architecture

```
Agent (Claude, etc.) → MCP Client → Golang MCP Servers → CRM · Billing · Ticketing · …
```

## MCP Servers

| Server        | Actions                                   |
|---------------|-------------------------------------------|
| CRM           | Create/update leads, opportunities, contacts |
| Ticketing     | Open, update, resolve, assign tickets     |
| Billing       | Refunds, credits, invoice checks          |
| Calendar      | Schedule meetings, check availability     |
| Knowledge     | Query docs, policies, pricing             |
| Notifications | Slack, email, SMS                         |

## Example Workflows

**Refund (CS):** request → check order + policy → validate → issue refund → update ticket & notify → escalate only on exception.

**Inbound lead (Sales):** lead arrives → enrich via CRM → score fit & intent → book AE meeting → send follow-up → log activity.

## Guardrails

- Human approval for high-risk or high-value actions
- Auth, rate limits, approval thresholds
- Audit trail for every action

## Roadmap

1. **Core MCP** — CRM, Ticketing, Billing servers
2. **Agent wiring** — connect MCP-capable agents
3. **Guardrails** — auth, limits, logging
4. **Flows** — orchestration layer on top of MCP
5. **Scale** — more tools, multi-agent, analytics

## Why This Stack

- **MCP** — one standard protocol; any agent can use the same tools
- **Golang** — fast, concurrent, easy to deploy
- **Internal-only** — full control over auth, data, and logic
- **Composable** — add tools without changing agents
