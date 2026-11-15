package market

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		pieces := strings.Split(strings.TrimSpace(part), ";")
		if pieces[0] != "gzip" {
			continue
		}
		quality := 1.0
		for _, param := range pieces[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
			if ok && key == "q" {
				var err error
				quality, err = strconv.ParseFloat(value, 64)
				if err != nil {
					quality = 0
				}
			}
		}
		return quality > 0 && quality <= 1
	}
	return false
}

type compressedResponse struct {
	http.ResponseWriter
	writer io.Writer
}

func (w compressedResponse) Write(p []byte) (int, error) { return w.writer.Write(p) }

// Compress at the origin so large histories do not cross the tunnel as raw JSON.
func compressedState(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if r.Method != "GET" || r.URL.Path != "/api/market/state" || !acceptsGzip(r.Header.Get("Accept-Encoding")) {
		next.ServeHTTP(w, r)
		return
	}
	w.Header().Add("Vary", "Accept-Encoding")
	w.Header().Set("Content-Encoding", "gzip")
	writer, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
	defer writer.Close()
	next.ServeHTTP(compressedResponse{w, writer}, r)
}
