import { m } from "$lib/paraglide/messages.js";

export function statusLabel(status: string): string {
	switch (status) {
		case "entwurf":
			return m.abrechnung_status_entwurf();
		case "eingereicht":
			return m.abrechnung_status_eingereicht();
		case "bezahlt":
			return m.abrechnung_status_bezahlt();
		case "offen":
			return m.reise_status_offen();
		case "in_entwurf":
			return m.reise_status_in_entwurf();
		case "gesperrt":
			return m.reise_status_gesperrt();
		case "hochgeladen":
			return m.beleg_status_hochgeladen();
		case "in_aufbereitung":
			return m.beleg_status_in_aufbereitung();
		case "zur_bestaetigung":
			return m.beleg_status_zur_bestaetigung();
		case "fehlgeschlagen":
			return m.beleg_status_fehlgeschlagen();
		case "bestaetigt":
			return m.beleg_status_bestaetigt();
		case "storniert":
			return m.beleg_status_storniert();
		default:
			return status;
	}
}

export function dayTypeLabel(value: string | undefined): string {
	switch (value) {
		case "anreisetag":
			return m.day_anreisetag();
		case "zwischentag":
			return m.day_zwischentag();
		case "abreisetag":
			return m.day_abreisetag();
		case "eintaegig":
			return m.day_eintaegig();
		case "ueber_nacht":
			return m.day_ueber_nacht();
		default:
			return value ?? "";
	}
}

export function kostenartLabel(value: string): string {
	switch (value) {
		case "fahrtkosten":
			return m.kostenart_fahrtkosten();
		case "verpflegung":
			return m.kostenart_verpflegung();
		case "uebernachtung":
			return m.kostenart_uebernachtung();
		case "reisenebenkosten":
			return m.kostenart_reisenebenkosten();
		case "bewirtung":
			return m.kostenart_bewirtung();
		default:
			return value;
	}
}

export function warningLabel(code: string): string {
	const labels: Record<string, () => string> = {
		B01: m.warn_B01,
		B02: m.warn_B02,
		B03: m.warn_B03,
		B04: m.warn_B04,
		B05: m.warn_B05,
		B06: m.warn_B06,
		W01: m.warn_W01,
		W02: m.warn_W02,
		W03: m.warn_W03,
		W04: m.warn_W04,
		W05: m.warn_W05,
		W06: m.warn_W06,
		W07: m.warn_W07,
		W08: m.warn_W08,
		W09: m.warn_W09,
		W10: m.warn_W10,
		W11: m.warn_W11,
		W12: m.warn_W12,
		W13: m.warn_W13,
		W14: m.warn_W14,
		"H-UST-KURS": m.warn_H_UST_KURS,
	};
	return labels[code]?.() ?? code;
}
