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
	"/arbeitgeber": () => import("./routes/Arbeitgeber.svelte"),
	"/arbeitgeber/:id": () => import("./routes/ArbeitgeberDetail.svelte"),
	"/taetigkeitsstaetten": () => import("./routes/Taetigkeitsstaetten.svelte"),
	"/satztabellen": () => import("./routes/Satztabellen.svelte"),
	"/satztabellen/:jahr": () => import("./routes/SatztabelleJahr.svelte"),
	"/about": () => import("./routes/About.svelte"),
	"/reisen": () => import("./routes/Reisen.svelte"),
	"/reisen/neu": () => import("./routes/ReiseNeu.svelte"),
	"/reisen/:id": () => import("./routes/ReiseDetail.svelte"),
	"/belege/neu": () => import("./routes/Placeholder.svelte"),
	"/abrechnungen": () => import("./routes/Placeholder.svelte"),
});
