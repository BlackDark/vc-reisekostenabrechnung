# Features

German and English. Several people can use one instance; each person sees only their own trips. The interface uses shadcn-svelte. Dark is the default. Light and the system setting are in the header, and the choice is remembered.

- Trips at home and abroad, with the country for each day taken from the stops (**Ortswechsel**) and the year's **Satztabelle** (rate table).
- Meal, overnight, and mileage allowances, and a reduction when the employer provided a meal.
- Receipt photos, PDFs, and e-invoice XML. Photos are deskewed and stored as an archive image after you confirm them. Optional reading of a receipt only suggests fields, and only after you opt in.
- A claim as PDF/A-3b, plus ZIP, CSV, and JSON. The same snapshot renders the same bytes.
- **Aufbewahrung** (retention) until 31 December of year *J* + 8. An admin deletes files only after that date, with the **Ablaufhemmung** warning confirmed and a reason kept in the audit log.
- Backup with `reisekosten backup`, or Litestream as UID 65532.

Sign in with a password, with OIDC (Pocket ID is the example), or with a header from a reverse proxy you trust. See [Authentication](authentication.md).
