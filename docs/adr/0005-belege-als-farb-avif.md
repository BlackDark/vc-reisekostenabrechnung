---
status: proposed
---
# Belege als bereinigtes Farb-AVIF archivieren

Fotografierte Belege werden im Browser entzerrt und auf 300 dpi skaliert, auf dem Server von Schatten und Papierton bereinigt und als Farb-AVIF (q≈40, ~50–85 KB) als unveränderbarer Archivbeleg mit SHA-256 gespeichert; das Original-Foto verlässt das Handy nicht. Schwarz-Weiß (CCITT/JBIG2, ~20 KB) wurde als alleiniges Archiv verworfen, weil Stempel über Beträgen und verblasste Thermobons im Test unlesbar wurden und sich ein Bilevel-Bild später nicht neu aufbereiten lässt. Weil Typst kein AVIF einbettet, gibt es für den Export eine JPEG-Kopie. PDFs und E-Rechnungs-XML bleiben unverändert. Wird nach einem Test mit echten Belegen des Nutzers auf `accepted` gesetzt.
