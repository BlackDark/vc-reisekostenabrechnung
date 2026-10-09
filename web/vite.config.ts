import { fileURLToPath } from "node:url";
import { paraglideVitePlugin } from "@inlang/paraglide-js";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import { VitePWA } from "vite-plugin-pwa";
import { defineConfig } from "vitest/config";

export default defineConfig({
	plugins: [
		svelte(),
		tailwindcss(),
		paraglideVitePlugin({
			project: "./project.inlang",
			outdir: "./src/lib/paraglide",
			strategy: ["localStorage", "preferredLanguage", "baseLocale"],
			emitTsDeclarations: true,
		}),
		VitePWA({
			registerType: "prompt",
			manifest: {
				name: "Reisekostenabrechnung",
				short_name: "Reisekosten",
				start_url: "/?source=pwa",
				scope: "/",
				display: "standalone",
				lang: "de",
				theme_color: "#09090b",
				background_color: "#09090b",
				icons: [
					{ src: "/icon-192.png", sizes: "192x192", type: "image/png" },
					{ src: "/icon-512.png", sizes: "512x512", type: "image/png" },
					{
						src: "/icon-maskable-192.png",
						sizes: "192x192",
						type: "image/png",
						purpose: "maskable",
					},
					{
						src: "/icon-maskable-512.png",
						sizes: "512x512",
						type: "image/png",
						purpose: "maskable",
					},
				],
				shortcuts: [
					{ name: "Beleg erfassen", url: "/belege/neu?kamera=1" },
					{ name: "Neue Reise", url: "/reisen/neu" },
					{ name: "Abrechnungen", url: "/abrechnungen" },
				],
			},
			workbox: {
				navigateFallback: "/index.html",
				navigateFallbackDenylist: [
					/^\/api\//,
					/^\/healthz/,
					/^\/readyz/,
					/^\/version/,
					/^\/auth\//,
				],
				globIgnores: ["**/beleg-corners*"],
				runtimeCaching: [
					{
						urlPattern: /\/api\/v1\/belege\/[^/]+\/vorschau$/,
						handler: "CacheFirst",
						options: {
							cacheName: "beleg-vorschau",
							expiration: { maxEntries: 300, maxAgeSeconds: 30 * 24 * 60 * 60 },
						},
					},
					{
						urlPattern: /beleg-corners/,
						handler: "CacheFirst",
						options: {
							cacheName: "beleg-corners",
							expiration: { maxEntries: 4, maxAgeSeconds: 30 * 24 * 60 * 60 },
						},
					},
					{
						// Navigations such as the OIDC start redirect must be handled by the
						// browser. NetworkOnly follows the cross-origin 302 inside the worker
						// and the login page never appears.
						urlPattern: ({ request, url }) =>
							request.mode !== "navigate" && url.pathname.startsWith("/api/"),
						handler: "NetworkOnly",
					},
				],
			},
		}),
	],
	build: {
		rollupOptions: {
			output: {
				manualChunks(id) {
					if (id.includes("/beleg/geometry") || id.includes("/beleg/corners"))
						return "beleg-corners";
				},
			},
		},
	},
	resolve: {
		alias: { $lib: fileURLToPath(new URL("./src/lib", import.meta.url)) },
	},
	server: {
		proxy: {
			"/api": "http://127.0.0.1:8080",
			"/healthz": "http://127.0.0.1:8080",
			"/readyz": "http://127.0.0.1:8080",
			"/version": "http://127.0.0.1:8080",
		},
	},
	test: {
		environment: "jsdom",
		include: ["src/**/*.test.ts"],
	},
});
