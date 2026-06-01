# Incident Manager Microservice

## Purpose

This document defines the lightweight incident management microservice that integrates with the existing `opsmuster` MSP core.

The microservice is intended to:

- receive incident alerts from external systems
- map incident data into the ticketing model
- create or update incident tickets in `opsmuster`
- keep incident state synchronized with core case management

## Architecture

The `incident-manager` service is designed as a small REST API that acts as a trusted ingestion point for incident alerts and a bridge to `opsmuster`.

Key components:

- `POST /api/v1/incidents` — receive new incident alerts
- `PUT /api/v1/incidents/{incident_id}` — update existing incident details
- `GET /api/v1/incidents/{incident_id}` — fetch incident metadata
- outgoing integration to `opsmuster` via its REST API
- optional webhook/callback support for incident lifecycle events

The microservice should be deployable independently, with its own codebase and CI pipeline, while depending on `opsmuster` only through HTTP API calls.

## Integration contract with `opsmuster`

### Ticket creation flow

When an incident alert arrives:

1. `incident-manager` validates the payload
2. It maps incident fields to `opsmuster` ticket fields
3. It calls `opsmuster` to create or update a ticket
4. It stores local incident metadata for tracking and reconciliation

### Minimum data to send to `opsmuster`

- `subject`
- `description`
- `category_id` or category slug
- `priority`
- `status`
- `customer_email` or requester identity
- `custom_fields` for incident-specific metadata
- `tags`

### Example create-ticket payload

```json
{
  "subject": "Network outage detected at Bonbeach YCW",
  "description": "Multiple sensors report loss of connectivity in Zone 3.",
  "category_id": "incidents-network",
  "priority": "high",
  "status": "open",
  "customer_email": "ops@bonbeachycw.local",
  "custom_fields": {
    "incident_id": "INC-2026-001",
    "severity": "critical",
    "service": "WiFi",
    "location": "Bonbeach YCW"
  },
  "tags": ["incident", "network", "bonbeach"]
}
```

## Field mapping guidance

Map external incident fields to help desk fields consistently.

| Incident field | `opsmuster` field | Notes |
|---|---|---|
| `id` | `incident_id` custom field | Primary incident identifier |
| `title` / `summary` | `subject` | Short one-line description |
| `details` / `description` | `description` | Full incident narrative |
| `severity` | `custom_fields.severity` | Critical / major / minor |
| `impact` | `custom_fields.impact` | Business impact note |
| `service` | `custom_fields.service` | Affected service or asset |
| `location` | `custom_fields.location` | Physical site or region |
| `detected_at` | `custom_fields.detected_at` | ISO8601 timestamp |
| `resolved_at` | `custom_fields.resolved_at` | ISO8601 timestamp |
| `status` | `status` | Mapped to ticket status or workflow |
| `priority` | `priority` | Ticket priority where available |
| `assigned_group` | `assigned_group_id` or tag | Routing into the right team |
| `tags` | `tags` | Add incident-specific tags |

## Deployment and operations

- The microservice should run in its own container.
- It should be configured with the `opsmuster` API base URL and an API key or bearer token.
- It should expose health and readiness endpoints.
- Logging must capture inbound alert IDs, mapped ticket IDs, and error details.

## Roadmap

1. Scaffold the microservice repository and CI pipeline.
2. Implement inbound incident webhook endpoint(s).
3. Add `opsmuster` API integration and ticket creation.
4. Add retry and error-handling logic for failed requests.
5. Add mapping configuration for Bonbeach-specific incident fields.
6. Add reconciliation support for incident updates.

## Notes

- Keep the microservice small and focused: it should not duplicate core ticketing workflows.
- All ticket state and business rules remain in `opsmuster`.
- The bridge should allow a clean rollback by disabling the incident webhook or redirecting it away from the service.
