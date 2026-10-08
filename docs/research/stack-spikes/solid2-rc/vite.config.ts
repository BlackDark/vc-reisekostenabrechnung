import { VitePWA } from "vite-plugin-pwa"
import { defineConfig } from "vite"
import solid from "@solidjs/vite-plugin"
import tailwindcss from "@tailwindcss/vite"
import { fileURLToPath } from "node:url"
export default defineConfig({ plugins: [solid(), tailwindcss(), VitePWA({ registerType: "autoUpdate", manifest: { name: "Reisekosten", short_name: "RK" } })], resolve: { alias: { "~": fileURLToPath(new URL("./src", import.meta.url)) } } })
