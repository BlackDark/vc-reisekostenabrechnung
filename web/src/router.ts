import { createRouter } from "sv-router";
import Admin from "./routes/Admin.svelte";
import Home from "./routes/Home.svelte";
import Login from "./routes/Login.svelte";
import Profil from "./routes/Profil.svelte";
import Setup from "./routes/Setup.svelte";

export const { p, navigate, route } = createRouter({
	"/": Home,
	"/login": Login,
	"/setup": Setup,
	"/profil": Profil,
	"/admin": Admin,
	"/admin/aufbewahrung": () => import("./routes/Aufbewahrung.svelte"),
	"/arbeitgeber": () => import("./routes/Arbeitgeber.svelte"),
	"/arbeitgeber/:id": () => import("./routes/ArbeitgeberDetail.svelte"),
	"/taetigkeitsstaetten": () => import("./routes/Taetigkeitsstaetten.svelte"),
	"/satztabellen": () => import("./routes/Satztabellen.svelte"),
	"/satztabellen/:jahr": () => import("./routes/SatztabelleJahr.svelte"),
	"/about": () => import("./routes/About.svelte"),
	"/reisen": () => import("./routes/Reisen.svelte"),
	"/reisen/neu": () => import("./routes/ReiseNeu.svelte"),
	"/reisen/:id/ausgaben/neu": () => import("./routes/AusgabeForm.svelte"),
	"/reisen/:id": () => import("./routes/ReiseDetail.svelte"),
	"/ausgaben/:id": () => import("./routes/AusgabeForm.svelte"),
	"/vorschuesse": () => import("./routes/Vorschuesse.svelte"),
	"/belege": () => import("./routes/Belege.svelte"),
	"/belege/neu": () => import("./routes/BelegNeu.svelte"),
	"/belege/:id": () => import("./routes/BelegDetail.svelte"),
	"/abrechnungen/neu": () => import("./routes/AbrechnungNeu.svelte"),
	"/abrechnungen/:id": () => import("./routes/AbrechnungDetail.svelte"),
	"/abrechnungen": () => import("./routes/Abrechnungen.svelte"),
});
