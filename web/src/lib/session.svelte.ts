import { api } from "$lib/api";
import { type Locale, pickLocale } from "$lib/locale";
import { getLocale, setLocale } from "$lib/paraglide/runtime.js";

export type Nutzer = {
	id: string;
	anzeigename: string;
	benutzername: string;
	ist_admin: boolean;
	sprache: string;
	version: number;
	aktiv: boolean;
	ki_erlaubt: boolean;
	email?: string | null;
	personalnummer?: string | null;
};

class Session {
	nutzer = $state<Nutzer | null>(null);
	online = $state(true);
	locale = $state<Locale>("de");
	ready = $state(false);

	async init() {
		if (typeof navigator !== "undefined") {
			this.online = navigator.onLine;
			addEventListener("online", () => {
				this.online = true;
			});
			addEventListener("offline", () => {
				this.online = false;
			});
		}
		const config = await api.GET("/api/v1/auth/config");
		const me = await api.GET("/api/v1/auth/me");
		if (me.response.ok && me.data) this.nutzer = me.data;
		const stored = localStorage.getItem("PARAGLIDE_LOCALE");
		const languages =
			typeof navigator === "undefined" ? [] : navigator.languages;
		this.applyLocale(
			pickLocale(
				this.nutzer?.sprache,
				stored,
				languages,
				config.data?.default_locale,
			),
		);
		this.ready = true;
	}

	applyLocale(next: Locale) {
		setLocale(next, { reload: false });
		this.locale = (getLocale() as Locale) || next;
		if (typeof document !== "undefined")
			document.documentElement.lang = this.locale;
	}
}

export const session = new Session();
