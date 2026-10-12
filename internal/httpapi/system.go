package httpapi

import (
	"net/http"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/belegpipe"
)

// GetAdminSystem reports version, storage and job counts for administrators.
func (a *App) GetAdminSystem(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	offen, erledigt, fehl, err := a.store.JobCounts(ctx)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	size, err := a.store.SizeBytes(ctx)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	nutzer, err := a.store.CountNutzer(ctx)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	admins, err := a.store.CountAktiveAdmins(ctx)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	location := a.cfg.StorageLocalPath
	if a.cfg.StorageBackend == "s3" {
		location = a.cfg.S3.Bucket
	}
	pipeline := belegpipe.Version
	writeJSON(w, http.StatusOK, api.SystemStatus{
		Version:         a.Version,
		Commit:          &a.Commit,
		PipelineVersion: &pipeline,
		StorageBackend:  a.cfg.StorageBackend,
		StorageLocation: &location,
		DbSizeBytes:     size,
		NutzerCount:     &nutzer,
		AdminCount:      &admins,
		JobsOpen:        &offen,
		JobsDone:        &erledigt,
		JobsFailed:      &fehl,
	})
}
