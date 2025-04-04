package assets

import (
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"remote-agent/biz"
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
		w.Header().Add("Server", biz.UserAgent)
		w.Header().Add("Content-Type", mime.TypeByExtension(ext))
		w.WriteHeader(http.StatusOK)
		w.Write(p)
		return
	} else {
		fmt.Printf("Error reading file %s: %v\n", path, err)
	}

	// 404
	w.Header().Add("Server", biz.UserAgent)
	http.Error(w, "404 Not Found", http.StatusNotFound)
}
