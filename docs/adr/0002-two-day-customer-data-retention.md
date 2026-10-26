# Customer names and dietary data are Anonymised two days after the Bill closes

Customers give a name (required) and may tick a Dietary Restriction (e.g. avoid Dairy) and add a Dietary Note, only to be served during one Table visit. Dietary Restrictions and Dietary Notes are health data, which UU PDP (Law 27/2022) Art. 4 classifies as specific personal data, and Art. 43 requires a controller to delete personal data once it is no longer needed for its processing purpose. We therefore Anonymise each Bill 48 hours after it closes: names become "Customer", and Dietary Restrictions and Dietary Notes are deleted. Dietary Warnings on Order Items survive, because they record only what the Product contained, not who avoided it. Two days covers same-day service, next-day complaints and Shift Closing disputes; beyond that there is no purpose that justifies keeping the data, and holding less lowers breach exposure.

UU PDP sets no fixed number of days. Two days is the Business's own judgement of necessity, and must be revisited if the purpose changes (for example, introducing loyalty profiles).

## Considered Options

- **30 days**: rejected; no business purpose needs a Customer's name or health note that long.
- **Keep indefinitely with Customer accounts**: rejected; no accounts or loyalty in scope, and it multiplies the obligations for specific personal data.

## Consequences

- Sales, Payment, Receipt and audit figures are kept 10 years as required by UU KUP Art. 28(11) — they need no Customer name, so Anonymisation satisfies both laws at once.
- The internal Receipt copy omits the Customer name, because paper cannot be Anonymised.
- Explicit consent is collected before a Dietary Note is saved.
- Logs must never contain names or dietary data.
- Backups keep their 30-day disaster-recovery window, so the two days are enforced by **crypto-shredding**: names and dietary data are encrypted with a per-day key, and keys are destroyed once every Bill using them is past 48 hours (TRD TR-RET-04). Shorter backups were rejected as weakening recovery; accepting 30-day exposure was rejected as breaking the two-day promise. As a result, Dietary Restrictions are stored as encrypted lists, not queryable join tables.
- The owner chose to show full Customer names to anyone scanning an occupied Table's QR, with no verification to Rejoin (BRD BR-CM-12, §11). This is an accepted exposure within the two-day window, not an exception to it.
