package webapp

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ogefest/findex/app"
)

func (webapp *WebApp) download() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		fileIdStr := chi.URLParam(r, "id")
		fileId, err := strconv.ParseInt(fileIdStr, 10, 64)
		if err != nil {
			webapp.renderError(w, http.StatusBadRequest, "Invalid file ID provided.")
			return
		}
		index := chi.URLParam(r, "index")

		searcher, err := app.NewSearcher(webapp.IndexConfig)
		if err != nil {
			log.Printf("Unable to create searcher: %v\n", err)
			webapp.renderError(w, http.StatusInternalServerError, "")
			return
		}

		fileInfo, err := searcher.GetFileByID(index, fileId)
		if err != nil || fileInfo == nil {
			log.Printf("File not found: %v\n", err)
			webapp.renderError(w, http.StatusNotFound, "The requested file was not found in the index.")
			return
		}

		fullPath := fmt.Sprintf("%s/%s", fileInfo.Dir, fileInfo.Path)
		log.Printf("Download %s\n", fullPath)

		// Check if file is inside a zip archive (path contains "!/")
		if strings.Contains(fileInfo.Path, "!/") {
			webapp.downloadFromZip(w, fileInfo.Dir, fileInfo.Path, fileInfo.Name)
			return
		}

		file, err := os.Open(fullPath)
		if err != nil {
			log.Printf("Cannot open file: %v\n", err)
			webapp.renderError(w, http.StatusNotFound, "The file exists in the index but could not be found on disk.")
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

func (webapp *WebApp) downloadFromZip(w http.ResponseWriter, dir, path, filename string) {
	// Path format: archive.zip!/internal/path/file.txt
	parts := strings.SplitN(path, "!/", 2)
	if len(parts) != 2 {
		webapp.renderError(w, http.StatusBadRequest, "Invalid zip path format.")
		return
	}

	zipRelPath := parts[0]
	internalPath := parts[1]
	zipFullPath := fmt.Sprintf("%s/%s", dir, zipRelPath)

	log.Printf("Extracting %s from %s\n", internalPath, zipFullPath)

	reader, err := zip.OpenReader(zipFullPath)
	if err != nil {
		log.Printf("Cannot open zip file: %v\n", err)
		webapp.renderError(w, http.StatusNotFound, "The zip archive could not be opened.")
		return
	}
	defer reader.Close()

	// Find the file in the zip
	var targetFile *zip.File
	for _, f := range reader.File {
		if f.Name == internalPath || strings.TrimSuffix(f.Name, "/") == internalPath {
			targetFile = f
			break
		}
	}

	if targetFile == nil {
		log.Printf("File not found in zip: %s\n", internalPath)
		webapp.renderError(w, http.StatusNotFound, "The file was not found inside the zip archive.")
		return
	}

	rc, err := targetFile.Open()
	if err != nil {
		log.Printf("Cannot open file in zip: %v\n", err)
		webapp.renderError(w, http.StatusInternalServerError, "Could not read file from zip archive.")
		return
	}
	defer rc.Close()

	// Read first 512 bytes to detect MIME type
	buffer := make([]byte, 512)
	n, _ := rc.Read(buffer)
	mimeType := http.DetectContentType(buffer[:n])

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", targetFile.UncompressedSize64))

	// Write the buffer we already read
	w.Write(buffer[:n])

	// Copy the rest
	if _, err := io.Copy(w, rc); err != nil {
		log.Printf("Error sending file from zip: %v\n", err)
	}
}
