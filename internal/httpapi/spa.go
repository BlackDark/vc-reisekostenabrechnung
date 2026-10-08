package httpapi

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
)

func spaHandler(dist fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		f, err := dist.Open(name)
		if err != nil {
			serveIndex(w, r, dist)
			return
		}
		defer func() { _ = f.Close() }()
		st, err := f.Stat()
		if err != nil || st.IsDir() {
			serveIndex(w, r, dist)
			return
		}
		http.ServeContent(w, r, st.Name(), st.ModTime(), f.(io.ReadSeeker))
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, dist fs.FS) {
	f, err := dist.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", st.ModTime(), f.(io.ReadSeeker))
}
