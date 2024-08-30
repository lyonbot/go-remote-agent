package assets

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

func HandleWebAssets(w http.ResponseWriter, r *http.Request) {
	fsPrefix := "frontend"
	path := r.URL.Path

	if strings.HasSuffix(path, "/") {
		// access homepage of a directory
		path += "index.html"
	} else if _, err := StaticFs.ReadDir(fsPrefix + path); err == nil {
		// is a directory
		w.Header().Set("Location", path+"/")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	if p, err := StaticFs.ReadFile(fsPrefix + path); err == nil && len(p) > 0 {
		// is a file
		// log.Println("Requesting static file", path)
		ext := filepath.Ext(path)
		w.Header().Add("Content-Type", mime.TypeByExtension(ext))
		w.WriteHeader(http.StatusOK)
		w.Write(p)
		return
	}

	// 404
	http.Error(w, "404 Not Found", http.StatusNotFound)
}
