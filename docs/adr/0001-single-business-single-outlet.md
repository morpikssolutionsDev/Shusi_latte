# Single business, single outlet — no tenant or outlet entity

Sushi Latte serves exactly one Business running exactly one outlet, so the data model has no Tenant or Outlet entity and no tenant/outlet column on any table. "Multi point" in the original goals means the platform has many Modules, not many outlets.

## Considered Options

- **Multi-tenant SaaS**: rejected; the product is not sold to other businesses.
- **Outlet entity from day one, launch with one outlet**: rejected by the owner; there is no plan to open branches, and the extra scoping on every query, report and permission has no payoff.

## Consequences

Adding a second outlet later means adding an outlet key to menus, prices, Tables, Bills, crew, shifts and reports, plus outlet-scoped permissions — a migration touching nearly every table. Role names keep "Outlet Admin" for familiarity even though there is only one outlet.
