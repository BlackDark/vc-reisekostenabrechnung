# Svelte 5 statt SolidJS

Das Frontend ist eine Vite-SPA mit Svelte 5, `shadcn-svelte` und Tailwind 4. Gewünscht war SolidJS 2.0 mit `solid-ui`; im Oktober 2026 war Solid 2.0 noch RC mit laufenden Breaking Changes, `solid-ui` hatte keine Solid-2-Version, Kobalte 2 war Alpha, und TanStack Form/Table bauten nicht gegen Solid 2 (Spike: 33 Typfehler). Solid 1.9 hätte funktioniert, aber eine absehbare 2.0-Migration mitgebracht. Svelte kostet rund 20 bis 30 KB gzip mehr, was eine PWA mit Service-Worker-Cache kaum spürt.

## Consequences

- Kein ESLint (Vorgabe): Lint und Format über Biome mit `html.experimentalFullSupportEnabled`, Typprüfung über `svelte-check` auf TypeScript 6, bis `svelte-check` TypeScript 7 unterstützt.
