package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
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

type Link struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

const SITES_DIR = "./config/root"
const LINKS_FILE = "./config/links.json"
const SITES_FILE = "./config/websites.txt"
const SEARCH_CONTEXT = 100

var domains []Link

func main() {
	// Which port should we host on?
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}

	err := preloadSearchableDomains()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Start the server
	fmt.Println("Starting HTTP server on http://localhost:" + port)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/links.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, LINKS_FILE)
	})
	http.Handle("/", http.FileServer(http.Dir(SITES_DIR)))
	server := &http.Server{Addr: ":" + port}
	err = server.ListenAndServe()
	fmt.Println(err)
}

func preloadSearchableDomains() error {
	// Which domains do we have in the links section?
	configFile, err := os.Open(LINKS_FILE)
	if err != nil {
		return err
	}
	configBytes, _ := io.ReadAll(configFile)
	var links []Link
	err = json.Unmarshal(configBytes, &links)
	if err != nil {
		return err
	}
	for _, link := range links {
		domains = append(domains, link)
	}

	// Which domains do we have cached?
	file, err := os.Open(SITES_FILE)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Split(strings.ToLower(scanner.Text()), "://")
		if len(parts) != 2 {
			// That's a weird URL. Ignore it.
			continue
		}
		domains = append(domains, Link{
			URL:   "/" + parts[1],
			Title: parts[1],
		})
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	// Get the query term.
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	var results []SearchResult

	// Find matches with domains
	for _, domain := range domains {
		if idx := strings.Index(strings.ToLower(domain.Title), query); idx >= 0 {
			results = append(results, SearchResult{
				Title:   domain.Title,
				Path:    domain.URL,
				Snippet: "",
			})
			continue
		}
		if idx := strings.Index(strings.ToLower(domain.URL), query); idx >= 0 {
			results = append(results, SearchResult{
				Title:   domain.Title,
				Path:    domain.URL,
				Snippet: "",
			})
		}
	}

	// Walk through the directory recursively.
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
			start := idx - SEARCH_CONTEXT
			if start < 0 {
				start = 0
			}
			end := idx + SEARCH_CONTEXT + len(query)
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
