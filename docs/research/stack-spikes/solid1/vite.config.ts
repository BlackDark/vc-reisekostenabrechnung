import { defineConfig } from "vite"
import solid from "vite-plugin-solid"
import tailwindcss from "@tailwindcss/vite"
import { fileURLToPath } from "node:url"
export default defineConfig({ plugins: [solid(), tailwindcss()], resolve: { alias: { "~": fileURLToPath(new URL("./src", import.meta.url)) } } })
