#let d = json("render.json")
#set document(title: d.titel, author: d.nutzer)
#set page(
  paper: "a4",
  margin: (top: 1.8cm, bottom: 1.6cm, x: 1.5cm),
  header: if d.entwurf { align(center, text(fill: rgb("#b0b0b0"), size: 22pt)[ENTWURF]) } else { none },
  footer: context [
    #d.nummer v#d.version · #d.fuss · #counter(page).display()
  ],
)
#set text(size: 10pt, lang: d.lang)

= #d.titel

#d.arbeitgeber

#d.anschrift

#d.nutzer_zeile

#d.meta

#v(8pt)
#for row in d.summen [
  #grid(columns: (1fr, auto), row.label, row.betrag)
]

#v(6pt)
#text(weight: "bold")[#d.auszahlung_label: #d.auszahlung]

#d.hinweis

#v(8pt)
#d.bestaetigung

#v(1.2cm)
#d.freigabe

#if d.reisen.len() > 0 [
  #pagebreak()
  == #d.uebersicht_titel
  #for r in d.uebersicht [
    #grid(columns: (auto, 1fr, auto), gutter: 8pt, r.nr, r.text, r.summe)
  ]
]

#for r in d.reisen [
  #pagebreak()
  == #r.kopf
  #if r.orte != "" [
    #r.orte
    #v(4pt)
  ]
  #if r.tage.len() > 0 [
    === #d.tage_titel
    #for t in r.tage [
      #t
      #linebreak()
    ]
  ]
  #if r.fahrten.len() > 0 [
    === #d.fahrten_titel
    #for f in r.fahrten [
      #f
      #linebreak()
    ]
  ]
  #if r.ausgaben.len() > 0 [
    === #d.ausgaben_titel
    #for a in r.ausgaben [
      #a
      #linebreak()
    ]
  ]
  #text(weight: "bold")[#d.summe_label: #r.summe]
]

#if d.bewirtungen.len() > 0 [
  #pagebreak()
  == #d.bewirtung_titel
  #for b in d.bewirtungen [
    #b
    #v(4pt)
  ]
]

#if d.protokoll.len() > 0 [
  == #d.protokoll_titel
  #for p in d.protokoll [
    - #p
  ]
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
