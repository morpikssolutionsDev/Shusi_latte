# Table QR codes are static: each Table opens /table-NN

The owner wants a Table's printed QR to be permanent: scanning it opens `<menu origin>/table-01` (Meja 1), `/table-02`, and so on, and nothing ever needs reprinting. Decided 2026-10-28. The code is derived from the Table number (`table-` + at least two digits), stored in `dining_table.code`, and follows the number if a Table is renumbered. There is no "regenerate QR" any more; an inactive Table's QR shows "please ask staff". The old random `/t/<token>` links are removed.

## Considered Options

- **Random token per Table, regenerable (the previous design)**: rejected by the owner; printed cards would need replacing whenever a code was regenerated, and the URL is meaningless to staff.
- **Static path plus a hidden regenerable token**: rejected; the printed `/table-NN` would remain the weaker way in, so the token would protect nothing.

## Consequences

- The URL is guessable: someone who is not at the venue can open `/table-02`, see the names of the Customers on an open Bill and Rejoin as one of them. The owner accepts this together with the existing name-visibility decision (BRD BR-CM-12, BR-P-07, §11). Every Order still needs a Waiter Claim before the Kitchen sees it, and Rejoin signs the previous device out, so the real Customer notices.
- The mitigation "staff can regenerate a Table QR" no longer exists; a compromised Table can only be deactivated.
- Codes are stable across demo resets, so printed demo QR codes keep working after a reset; only the guest sessions end.
- `table-1` and `TABLE-01` are accepted as `table-01`.
