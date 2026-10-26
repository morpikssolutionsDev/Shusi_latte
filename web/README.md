# Sushi Latte — Web frontends

Three React + Vite + TypeScript apps, one per TRD §3 app:

- `apps/client-menu/` — Customer QR menu: table scan, join with
  name/restriction/consent, catalog, cart, dietary-warning
  acknowledgement, bill view, waiter call, rejoin, not-mine.
  Open at `/client-menu/?table=<qr_token>`.
- `apps/staff/` — Staff app: sign-in (Crew ID + PIN), Waiter
  (orders, claim/takeover/confirm, serve/cancel/void/remake,
  bill summary/settle/unlock/reprint, direct orders, my claims),
  Kitchen (per-Prep-Area queue, transitions, cancel), Cashier
  (payments, QRIS verify/write-off, close, PIN Override tokens,
  takeaway, drawer sessions, post-outage), Admin (catalog,
  tables/QR, display slides, reports, audit log), HRIS
  (employees, attendance, leave, export) and HRIS Clock
  (screen sign-in, then clock in/out). Open at `/staff/`.
- `apps/display/` — TV slideshow: polls
  `/public/display/version` every 10 seconds and refetches
  the feed only when it changes, looping slides by
  `duration_seconds`, marking Sold Out featured Products.
  Open at `/display/`.

## Build and run

```sh
# in each app
npm install
npm run build

# from the repo root, with the API server running
go run ./api/cmd/server   # serves /client-menu/, /staff/, /display/
```

For Vite dev servers with API proxying, run `npm run dev`
in an app (ports 5173 client-menu, 5174 staff, 5175 display).

## Notes and deviations from TRD

- Styling is plain CSS with semantic tokens, not Tailwind /
  shadcn/ui, and data fetching is plain fetch, not TanStack
  Router/Query. Motion animations and a PWA manifest are not
  included in this first frontend pass.
- Receipts are fetched as bill data and printed with the
  browser print dialog. Web Bluetooth (BLE) thermal printing
  (TR-PRN-01 to TR-PRN-08) is hardware-specific and is not
  claimed here; it needs a hardware spike before purchase.
- The printed QR code still points to the API scan endpoint
  `/public/tables/{qr_token}` (Phase 1, TR-AUTH-07). To open
  the menu from a QR, point the printed code at
  `/client-menu/?table={qr_token}` in a future QR change.
- Server-side role checks still enforce BRD §5.1; the UI
  only hides what a role cannot do.
