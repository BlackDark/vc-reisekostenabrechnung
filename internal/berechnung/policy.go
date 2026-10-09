// Package berechnung holds calculation policy that later milestones implement.
package berechnung

// EineVerpflegungspauschaleProKalendertag is open question O2.
//
// true: at most one Verpflegungspauschale per Nutzer and calendar day, across
// every Arbeitgeber. false would allow one Pauschale per Arbeitgeber on the
// same day. The day rule itself stays in one place so the choice can change
// without scattering conditions through the calculator.
const EineVerpflegungspauschaleProKalendertag = true

// VerpflegungScope reports which objects share a calendar day's Pauschale.
// "nutzer" spans every Arbeitgeber. "arbeitgeber" would keep them separate.
func VerpflegungScope() string {
	if EineVerpflegungspauschaleProKalendertag {
		return "nutzer"
	}
	return "arbeitgeber"
}
