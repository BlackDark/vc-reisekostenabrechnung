# Tax rules, in short

**Not tax advice.** This is a map of what the app implements, so an operator can see the shape of the rules. It does not replace the employer's review or a tax adviser. The person who files an **Abrechnung** (expense claim) is responsible for it. Sources and the longer German notes are in [docs/research/steuer-reisekosten.md](research/steuer-reisekosten.md). The numbers live in the **Satztabellen** (rate tables) shipped with the app, including admin overrides.

The app is aimed at an employee who is reimbursed by an **Arbeitgeber** (employer) under the German income-tax rules for business travel. It is not a tool for the self-employed, and it does not compute the commute allowance (**Entfernungspauschale**).

## What a trip is

A **Reise** (business trip) is time away from home and the first place of work, from departure to return. Each **Reisetag** (day of the trip) records how long the person was away, the country that sets the allowance, and meals the employer provided. An **Ortswechsel** (a stop: arrival time, country, and means of transport) is how a trip that crosses borders picks the country for each day. The return is the end of the trip, not an extra stop, unless the way home enters another country before midnight.

## Allowances

A **Pauschale** (flat allowance) is a statutory amount that does not need a receipt:

- **Verpflegungspauschale** (meal allowance), for a day trip or for arrival, full, and departure days. One meal allowance per calendar day across all employers.
- **Mahlzeitenkürzung** (meal reduction) when breakfast, lunch, or dinner was provided by or for the employer, including a breakfast included in a reimbursed hotel rate.
- **Übernachtungspauschale** (overnight allowance) when the night was not paid as a receipt and the stay qualifies.
- **Kilometerpauschale** (mileage allowance) for a private vehicle. The app does not call a routing service; the kilometres are entered.

The rate for a day is the rate table of that calendar year. Foreign rates follow the country and, where the table has one, the city (**Satzort**).

## Receipts and VAT

A **Beleg** (receipt) is the file that proves an **Ausgabe** (an amount the person paid). Photos become an archive image after the user confirms them. PDFs and e-invoice XML are kept as received. An **Eigenbeleg** (substitute receipt) is the user's own note when no receipt exists.

German VAT on a foreign-currency receipt is a hint in v1, not a second exchange-rate feed. Entertainment (**Bewirtung**) keeps the occasion, the place, and the people present.

## The claim

An Abrechnung freezes a set of trips for one employer and one period. The **Erstattungsbetrag** (amount to reimburse) is the sum of the trips. A **Vorschuss** (advance) already paid is subtracted. The **Auszahlungsbetrag** (amount to pay out) is what remains; a negative number is money to return to the employer.

Submitting writes a PDF/A-3b plus a ZIP with CSV and JSON. The PDF states that the tax-free treatment follows § 3 no. 16 EStG where the lines are marked that way. Warnings can be acknowledged. Blockers (a missing receipt where one is required, for example) stop submission until they are fixed.

Rate tables for a later year are imported by an admin when the Federal Ministry of Finance publishes them. They are not guessed.
