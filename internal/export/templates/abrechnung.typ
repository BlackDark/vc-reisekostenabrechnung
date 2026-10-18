#let d = json("render.json")
#let accent = rgb("#1f3a5f")
#let soft = rgb("#f1f4f8")
#let rule = rgb("#cdd5df")
#let muted = rgb("#5b6675")

#set document(title: d.titel, author: d.nutzer)
#set page(
  paper: "a4",
  margin: (top: 1.8cm, bottom: 1.7cm, x: 1.5cm),
  header: if d.entwurf { align(center, text(fill: rgb("#b0b0b0"), size: 22pt)[ENTWURF]) } else { none },
  footer: context [
    #set text(size: 8pt, fill: muted)
    #line(length: 100%, stroke: 0.4pt + rule)
    #v(-2pt)
    #grid(columns: (1fr, auto), d.nummer + " v" + str(d.version) + " · " + d.fuss, counter(page).display("1") + "/" + str(counter(page).final().first()))
  ],
)
#set text(font: "Libertinus Serif", size: 10pt, lang: d.lang, features: ("tnum",))
#set par(spacing: 0.6em)
#show heading.where(level: 2): it => block(above: 1.2em, below: 0.6em, text(fill: accent, size: 13pt, it.body))
#show heading.where(level: 3): it => block(above: 1em, below: 0.4em, text(fill: accent, size: 10.5pt, it.body))

// Data table: header band, thin row rules, optional total row. Cells are strings.
#let tbl(columns, aligns, data, size: 8.5pt) = {
  set text(size: size)
  table(
    columns: columns,
    align: aligns,
    inset: (x: 4pt, y: 3.5pt),
    stroke: (x, y) => if y == 0 { none } else { (bottom: 0.4pt + rule) },
    fill: (x, y) => if y == 0 { accent } else { none },
    table.header(..data.head.map(h => text(fill: white, weight: "bold", h))),
    ..data.rows.flatten(),
    ..if data.foot.len() > 0 { data.foot.map(c => table.cell(fill: soft, strong(c))) } else { () },
  )
}

#let label(body) = text(size: 8pt, fill: muted, weight: "bold", upper(body))

// Cover
#block(width: 100%, fill: accent, inset: 14pt, radius: 3pt)[
  #set text(fill: white)
  #text(size: 19pt, weight: "bold")[#d.titel]
  #v(2pt)
  #text(size: 10pt)[#d.meta.join("  ·  ")]
]

#v(8pt)
#grid(
  columns: (1fr, 1fr), gutter: 12pt,
  block(width: 100%, fill: soft, inset: 9pt, radius: 3pt)[
    #label(d.label_arbeitnehmer)\
    #strong(d.nutzer_zeile)
  ],
  block(width: 100%, fill: soft, inset: 9pt, radius: 3pt)[
    #label(d.label_arbeitgeber)\
    #strong(d.arbeitgeber)\
    #d.anschrift
  ],
)

#v(6pt)
== #d.summen_titel
#table(
  columns: (1fr, auto),
  align: (left, right),
  inset: (x: 6pt, y: 4pt),
  stroke: (x, y) => (bottom: 0.4pt + rule),
  ..d.summen.map(r => (
    if r.sub { pad(left: 12pt, text(size: 9pt, fill: muted, r.label)) } else if r.strong { strong(r.label) } else { r.label },
    if r.sub { text(size: 9pt, fill: muted, r.betrag) } else if r.strong { strong(r.betrag) } else { r.betrag },
  )).flatten()
)
#v(4pt)
#block(width: 100%, fill: accent, inset: 10pt, radius: 3pt)[
  #set text(fill: white)
  #grid(columns: (1fr, auto), align: horizon, text(size: 11pt, weight: "bold", d.auszahlung_label), text(size: 16pt, weight: "bold", d.auszahlung))
]

#if d.steuer.len() > 0 [
  == #d.steuer_titel
  #tbl((1fr, auto, auto), (left, right, left), (head: d.steuer_head, rows: d.steuer, foot: ()))
]

#if d.uebersicht.len() > 0 [
  == #d.uebersicht_titel
  #tbl((auto, 1fr, auto, auto), (right, left, left, right), (head: d.uebersicht_head, rows: d.uebersicht, foot: ()))
]

#v(6pt)
#text(size: 9pt, fill: muted)[#d.hinweis]
#if d.eingereicht [
  #linebreak()
  #text(size: 9pt, fill: muted)[#d.bestaetigung]
]

#v(1fr)
#line(length: 55%, stroke: 0.5pt + muted)
#text(size: 8.5pt, fill: muted)[#d.freigabe]

// Trips
#for r in d.reisen [
  #pagebreak()
  == #r.kopf
  #text(size: 9pt, fill: muted)[
    #r.zeitraum
    #if r.projekt != "" [ · #d.projekt_label: #r.projekt ]
    #if r.orte != "" [ · #d.stationen_label: #r.orte ]
  ]
  #if r.tage.rows.len() > 0 [
    === #d.tage_titel
    #tbl((auto, auto, 1fr, auto, auto, auto, auto, auto, auto), (left, left, left, right, right, right, right, right, right), r.tage)
    #if r.regeln != "" [
      #text(size: 8pt, fill: muted)[#d.regeln_label: #r.regeln]
    ]
  ]
  #if r.fahrten.rows.len() > 0 [
    === #d.fahrten_titel
    #tbl((auto, 1fr, auto, auto, auto, auto), (left, left, left, right, right, right), r.fahrten)
  ]
  #if r.ausgaben.rows.len() > 0 [
    === #d.ausgaben_titel
    #tbl((auto, auto, 1fr, auto, auto, auto, auto, auto), (left, left, left, left, left, right, right, right), r.ausgaben)
  ]
  #v(6pt)
  #align(right, block(fill: soft, inset: 8pt, radius: 3pt)[#strong[#d.summe_label: #r.summe]])
]

#pagebreak()
#if d.bewirtungen.len() > 0 [
  == #d.bewirtung_titel
  #tbl((auto, auto, 1fr, auto, 1fr, auto, auto, auto), (left, left, left, left, left, left, right, right), (head: d.bewirtung_head, rows: d.bewirtungen, foot: ()))
]

#if d.eigenbelege.len() > 0 [
  == #d.eigenbeleg_titel
  #tbl((auto, auto, 1fr, auto, auto), (left, left, left, left, right), (head: d.eigenbeleg_head, rows: d.eigenbelege, foot: ()))
]

== #d.ust_titel
#if d.ust.len() == 0 [
  #d.keine_ust
] else [
  #tbl((auto, 1fr, 1fr, 1fr, 1fr), (left, right, right, right, right), (head: d.ust_head, rows: d.ust, foot: ()))
]
#if d.ust_ausland.len() > 0 [
  === #d.ust_ausland_titel
  #tbl((auto, auto, 1fr, 1fr, 1fr), (left, left, right, right, right), (head: d.ust_ausland_head, rows: d.ust_ausland, foot: ()))
]
#v(4pt)
#text(size: 9pt, fill: muted)[#d.ust_hinweis #d.ust_pauschale]

== #d.hinweise_titel
#if d.warnungen.len() == 0 [
  #d.keine_warnung
] else [
  #for w in d.warnungen [
    - #w
  ]
]

== #d.quellen_titel
#for q in d.quellen [
  - #q
]

== #d.protokoll_titel
#if d.protokoll.len() == 0 [
  #d.kein_protokoll
] else [
  #tbl((auto, 1fr, 1fr), (left, left, left), (head: d.protokoll_head, rows: d.protokoll, foot: ()), size: 8pt)
]

#for b in d.bilder [
  #pagebreak()
  #text(weight: "bold")[#b.kopf]
  #v(4pt)
  #if b.art == "text" [
    #b.text
  ] else [
    #image(b.pfad, alt: b.alt, width: 100%)
  ]
]

#for f in d.dateien [
  #pdf.attach(f.pfad, description: f.beschreibung, mime-type: f.mime, relationship: "source")
]
