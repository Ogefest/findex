package webapp

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ogefest/findex/app"
)

func (webapp *WebApp) download() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		fileIdStr := chi.URLParam(r, "id")
		fileId, err := strconv.ParseInt(fileIdStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid ID", http.StatusBadRequest)
			return
		}
		index := chi.URLParam(r, "index")

		searcher, err := app.NewSearcher(webapp.IndexConfig)
		if err != nil {
			log.Printf("Unable to create searcher: %v\n", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		fileInfo, err := searcher.GetFileByID(index, fileId)
		if err != nil {
			log.Printf("File not found: %v\n", err)
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		fullPath := fmt.Sprintf("%s/%s", fileInfo.Dir, fileInfo.Path)
		log.Printf("Download %s\n", fullPath)

		file, err := os.Open(fullPath)
		if err != nil {
			log.Printf("Cannot open file: %v\n", err)
			http.Error(w, "cannot open file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		buffer := make([]byte, 512)
		n, _ := file.Read(buffer)
		mimeType := http.DetectContentType(buffer[:n])

		file.Seek(0, 0)
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", fileInfo.Name))

		if _, err := io.Copy(w, file); err != nil {
			log.Printf("Error sending file: %v\n", err)
		}
	}
}
