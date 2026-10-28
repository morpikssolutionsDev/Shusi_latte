---
status: accepted
---

# Online-only for the current version; local outlet server recorded as future architecture

> **Demo note:** the prototype runs on a single local host with PostgreSQL ([ADR-0007](0007-postgresql.md)), so in the demo "online" means "on the host's local network". This ADR still describes the intended real deployment.

The current version runs entirely from one cloud deployment, and every Module except Receipt printing requires internet. During an outage, staff fall back to paper order notes. We chose this because it is the simplest design to build and operate for one outlet, and the Client Menu must be in the cloud anyway, since Customers reach it from their phones over mobile data.

## Considered Options

- **(a) Cloud, printing-only offline** — chosen for the current version.
- **(b) Cloud with an offline queue on staff devices**: rejected; orders taken offline would not reach the Kitchen until reconnecting, and merging offline edits to Bills creates conflict cases worse than paper.
- **(c) Local outlet server** — recorded as the intended future architecture. A Go server and its database on a mini-PC on the outlet LAN serve all staff Modules with no internet dependency; a small cloud relay serves the Client Menu and Product Display content and syncs with the local server.

## Consequences

To keep (c) reachable, the current design avoids cloud-specific managed services, keeps the API a single Go binary that could run on a mini-PC, and makes mutations idempotent so a future sync agent can replay them. Moving to (c) is a deployment and sync project, not a rewrite.
