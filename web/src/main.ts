import "@fontsource-variable/inter/wght.css";
import { mount } from "svelte";
import App from "./App.svelte";
import "./app.css";
import "./router";

const target = document.getElementById("app");
if (!target) throw new Error("missing #app");

export default mount(App, { target });
