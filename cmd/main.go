package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

type SearchResult struct {
	Title   string `json:"title"`
	Path    string `json:"path"`
	Snippet string `json:"snippet"`
}

const SITES_DIR = "./config/root"
const CONTEXT = 100

func main() {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}
	fmt.Println("Starting HTTP server on http://localhost:" + port)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/links.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./config/links.json")
	})
	http.Handle("/", http.FileServer(http.Dir(SITES_DIR)))
	server := &http.Server{Addr: ":" + port}
	err := server.ListenAndServe()
	fmt.Println(err)
}

// searchHandler reads the "q" query parameter and searches through all files under ./sites.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	// Get the query term.
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	// Walk through the directory recursively.
	var results []SearchResult
	err := filepath.Walk(SITES_DIR, func(path string, info os.FileInfo, err error) error {
		if len(results) > 100 {
			// We've seen enough hits, stop searching
			return nil
		}
		if err != nil {
			// If there's an error accessing a path, skip it.
			return nil
		}
		// Only process regular files.
		if !info.Mode().IsRegular() {
			return nil
		}
		// Only process files with .html or .htm extension.
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".html" && ext != ".htm" {
			return nil
		}

		// Read the file's contents.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we cannot read.
		}

		// Parse the HTML.
		doc, err := html.Parse(strings.NewReader(string(data)))
		if err != nil {
			return nil
		}

		// Extract the visible text from the DOM.
		text := strings.ToLower(extractText(doc))
		if idx := strings.Index(text, query); idx >= 0 {
			// Build a snippet from the text content (not including markup).
			start := idx - CONTEXT
			if start < 0 {
				start = 0
			}
			end := idx + CONTEXT + len(query)
			if end > len(text) {
				end = len(text)
			}
			snippet := text[start:idx] + "<b>" + text[idx:idx+len(query)] + "</b>" + text[idx+len(query):end]

			title, ok := getTitle(doc)
			if !ok {
				title = path[len(SITES_DIR)-2:]
			}

			results = append(results, SearchResult{
				Title:   title,
				Path:    path[len(SITES_DIR)-2:],
				Snippet: snippet,
			})
		}
		return nil
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the search results as JSON.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// extractText recursively extracts and concatenates text from HTML nodes.
func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	var sb strings.Builder
	// Recursively traverse child nodes.
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractText(c))
		// Add a space between nodes to separate words.
		sb.WriteString(" ")
	}
	return strings.TrimSpace(sb.String())
}

func isTitleElement(n *html.Node) bool {
	return n.Type == html.ElementNode && n.Data == "title"
}

func getTitle(n *html.Node) (string, bool) {
	if isTitleElement(n) {
		return n.FirstChild.Data, true
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result, ok := getTitle(c)
		if ok {
			return result, ok
		}
	}

	return "", false
}
