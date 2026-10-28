<p align="center"><img src="docs/brand/sushi-latte-lockup-animated.svg" alt="Sushi Latte" width="520"></p>

# Sushi Latte

A HORECA web platform for a sushi-and-coffee outlet: Customers order from a QR code at their Table, Waiters claim and confirm Orders, the Kitchen and Bar work from live displays, and Cashiers settle Bills and close the drawer. Management configures the menu, prices and discounts; HRIS handles shifts, attendance and leave.

**Status:** Phases 0–8 complete (foundation, Admin Panel, Product Display, Client Menu, Waiter, Kitchen, Cashier, HRIS, demo polish); only the printer and Kitchen-screen hardware checks wait for the venue; progress in [todolist.md](todolist.md). This build is a **prototype and showcase** that runs on one local machine with fictional demo data.

## Modules

| Module | Who | Device |
|---|---|---|
| Client Menu | Customer | Phone (QR per Table) |
| Waiter | Waiter | 8″ tablet |
| Cashier | Cashier | Tablet or PC |
| Kitchen | Kitchen / Bar crew | Large display per Prep Area |
| Product Display | Customers in the outlet | TV |
| Admin Panel | Management, Outlet Admin | Desktop |
| HRIS | Everyone | Shared clock-in tablet, desktop |

## Stack

Go (chi, sqlc, goose) · PostgreSQL · WebSockets · React + Vite + TypeScript · TanStack Router/Query · Tailwind · shadcn/ui · Motion · PWA with Web Bluetooth receipt printing.

## Documentation

| Document | Contents |
|---|---|
| [GLOSSARY.md](GLOSSARY.md) | Domain vocabulary |
| [docs/BRD.md](docs/BRD.md) | Business requirements and rules |
| [docs/TRD.md](docs/TRD.md) | Technical requirements |
| [docs/OVERVIEW-AND-FLOWS.md](docs/OVERVIEW-AND-FLOWS.md) | Document map, operational and backend flowcharts |
| [docs/DEMO-AND-SEED.md](docs/DEMO-AND-SEED.md) | Demo mode and seed scenarios |
| [docs/adr/](docs/adr/) | Architecture decision records |
| [docs/design/](docs/design/) | Design system: philosophy, tokens, colour, type, motion, components ([style guide](docs/design/styleguide.html)) |
| [docs/brand/](docs/brand/) | Logo and brand assets |
| [docs/images/](docs/images/) | Demo product, staff and marketing photos |
| [project goals.md](project%20goals.md) | The owner's original brief |

## Development

Requirements: Go 1.27+, Node 24+, pnpm 12, a local PostgreSQL 17+ server with an empty database (e.g. `sushi_latte`), Python 3.10+ (design tooling only), Windows PowerShell for the tool fetch script.

The API, seed and Go tests read the database from `POSTGRE_URL` ([ADR-0007](docs/adr/0007-postgresql.md)); the demo lives in schema `app`, tests use throwaway `test_…` schemas. The `pnpm` commands load it from a git-ignored `.env` at the repo root; [.env.example](.env.example) lists every variable with its default.

```sh
powershell -ExecutionPolicy Bypass -File tools/fetch-tools.ps1   # once: pinned sqlc + Caddy into .tools/
pnpm install
cp .env.example .env   # once: set POSTGRE_URL
pnpm dev     # API in Demo mode + Client Menu :5173, Staff :5174/staff/, Display :5175/display/
```

| Command | Does |
|---|---|
| `pnpm dev` | Generates API types and tokens, seeds the clean start if the database is empty, runs the API and all Vite dev servers |
| `pnpm seed` | Rebuilds the demo in PostgreSQL as the clean start (menu, staff, Tables, discounts, Display, settings); `pnpm seed --story` adds the scripted demo story. Stop the API first; while it runs, use Admin → Pengaturan → Reset demo |
| `pnpm demo` | Builds the apps and API, then serves everything over LAN HTTPS through Caddy (prints the URLs and the CA download link) |
| `pnpm gen` | Regenerates sqlc code (`api/internal/store`), API client types and design tokens |
| `pnpm test` | Web tests and `go test ./...` |
| `python tools/design/build.py` | Regenerates design tokens, `COLOR.md` and the style guide |

Demo logins are listed on the staff login screen (Crew ID + PIN, e.g. `M001` / `111111`). The launcher at `http://localhost:5173/demo` has every Table's QR code; the six guided tours ([DEMO-AND-SEED §10](docs/DEMO-AND-SEED.md)) appear once the demo story is loaded (`pnpm seed --story`, or "Muat cerita demo" on Admin → Pengaturan → Reset demo). The demo resets itself to the clean start at 03.00.

**LAN demo devices:** open `http://<host-ip>:8081/ca.crt` on each phone or tablet once and install it as a trusted CA. Web Bluetooth printing and installing the staff app require HTTPS ([TRD TR-ARC-05](docs/TRD.md)).
