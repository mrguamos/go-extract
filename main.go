package main

import (
	"encoding/json"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"

	"github.com/nguyenthenguyen/docx"
)

const (
	maxFileSize = 10 << 20 // 10 MB
)

type Response struct {
	Content string `json:"content"`
}

func main() {
	http.HandleFunc("/extract", handleExtract)
	log.Printf("Server starting on port 8989...")
	log.Fatal(http.ListenAndServe(":8989", nil))
}

func handleExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form with max memory
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("document")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check mime type
	if !isValidWordDocument(header) {
		http.Error(w, "Invalid file type. Only Word documents are allowed", http.StatusBadRequest)
		return
	}

	// Create temporary file
	tempFile, err := os.CreateTemp("", "word-*.docx")
	if err != nil {
		http.Error(w, "Error processing file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy uploaded file to temp file
	fileBytes := make([]byte, header.Size)
	if _, err = file.Read(fileBytes); err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}
	if _, err = tempFile.Write(fileBytes); err != nil {
		http.Error(w, "Error processing file", http.StatusInternalServerError)
		return
	}

	// Read docx file
	docxFile, err := docx.ReadDocxFile(tempFile.Name())
	if err != nil {
		http.Error(w, "Error reading document: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer docxFile.Close()

	// Get editable document
	doc := docxFile.Editable()
	xmlContent := doc.GetContent()

	// Extract text between <w:t> tags
	var content string
	for _, match := range regexp.MustCompile(`<w:t[^>]*>([^<]+)</w:t>`).FindAllStringSubmatch(xmlContent, -1) {
		if len(match) > 1 {
			content += match[1] + " "
		}
	}

	// Prepare response
	w.Header().Set("Content-Type", "application/json")
	response := Response{
		Content: content,
	}

	// Send JSON response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

func isValidWordDocument(header *multipart.FileHeader) bool {
	validMimeTypes := map[string]bool{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"application/msword": true,
	}

	return validMimeTypes[header.Header.Get("Content-Type")]
}
