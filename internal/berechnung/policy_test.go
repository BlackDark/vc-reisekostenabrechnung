package berechnung

import "testing"

func TestVerpflegungScopeIsPerNutzer(t *testing.T) {
	if !EineVerpflegungspauschaleProKalendertag {
		t.Fatal("O2 default is one Pauschale per calendar day across Arbeitgeber")
	}
	if VerpflegungScope() != "nutzer" {
		t.Fatalf("scope %s", VerpflegungScope())
	}
}
