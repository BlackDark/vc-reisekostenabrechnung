# Changelog

## [1.1.0](https://github.com/BlackDark/vc-reisekostenabrechnung/compare/v1.0.0...v1.1.0) (2026-10-10)


### Features

* **web:** add PWA install affordance and disable all forms while offline ([53cd3f3](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/53cd3f3b287d087e813cbed58583a0896da01b0f))
* **web:** create expenses in a side panel on the trip page ([eedb351](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/eedb3510e26ac4a5328a0ed659b6fead39ae785b))


### Bug Fixes

* **belege:** raise the archive quality and serve a readable full-size image ([82f00fe](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/82f00fe0af782c35f84bdd39568adfda16722b0e))
* grant called workflow permissions so a release can start ([#22](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/22)) ([95e560c](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/95e560c4a00743f70a9e97713252d386867cf756))
* pass --repo so release notes publish without a checkout ([#24](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/24)) ([30767b1](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/30767b1c351a071c4728b4c2bf6322deb436e1d3))


### Documentation

* add roadmap, docs index and email receipt intake design ([#30](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/30)) ([534fdd6](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/534fdd60f1b1331a410b00611762f30a81dd7efd))
* record spec audit gaps in roadmap ([#36](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/36)) ([19bf1ff](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/19bf1ff37466aa9da72ceadbd10ef33dcfc0994d))


### Miscellaneous

* **web:** regenerate the OpenAPI client for the Beleg image endpoint ([5fa9492](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/5fa94929cd59576730249a28eb2cc9e09062b546))

## 1.0.0 (2026-10-09)


### Features

* add Abrechnung lifecycle and PDF/A-3b export ([#10](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/10)) ([6396a4f](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/6396a4fe9a532329de69928a8e2531a74883a663))
* add retention, backup, and export polish ([#11](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/11)) ([d7041d8](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/d7041d8a7cc19bd03ba74630e029437465f85add))
* capture and archive Belege ([#7](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/7)) ([27998c6](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/27998c66482e94f10e65f6c1dec219f6ad60987d))
* M1 Walking Skeleton ([#3](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/3)) ([d09cdd1](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/d09cdd1993a319482baa9eec59caca445b52aa8f))
* M2 Arbeitgeber, Tätigkeitsstätten, Satztabellen ([#5](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/5)) ([222dcc7](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/222dcc745fa91cc3d00403eec3e5b35534d24110))
* M3 Reisen and Pauschalen ([#6](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/6)) ([181613c](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/181613c1cb8ecc8936b280d07f82b692ff461163))
* M9 CI, security and README polish ([#12](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/12)) ([7132085](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/7132085fadea2a28a42192b567810b5842f9ef5f))
* pack trip, claim, and home screens into dashboards ([#15](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/15)) ([fc6dee6](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/fc6dee671394e2bc59f19b1233101ae1b6550b6d))
* persist Ausgaben with VAT, FX, and Bewirtung ([#8](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/8)) ([25560fc](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/25560fc74c7082ff79251ef3006a65b71b592c86))
* rebuild the UI with shadcn-svelte and a dark default ([#13](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/13)) ([e79f9b4](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/e79f9b4cc7bc1b648b3f96fffdda123fdab1e4cf))
* suggest receipt fields from an optional KI endpoint ([#9](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/9)) ([560a830](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/560a83085a3b09c4f40cdd37bdc2ca94a0845603))


### Bug Fixes

* keep push CI green for WebKit screenshots and release-please ([#14](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/14)) ([889e429](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/889e4293e93cbf8ba194736c4e61e2c5c0e236f6))
* review pass with small consistency fixes and README landing page ([#16](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/16)) ([cc635ca](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/cc635ca690b44e24d0f9768234d2919d49213261))


### Documentation

* initiale Spezifikation, Glossar, ADRs und Recherche ([#1](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/1)) ([24ffe43](https://github.com/BlackDark/vc-reisekostenabrechnung/commit/24ffe43b50f2043dea87eb71119fd983bbb69518))

## [0.0.0] - unreleased

Walking skeleton. The first release-please release is 0.1.0.
