package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const defaultLibraryTTL = 5 * time.Minute

// LibraryEntry is one entry in the community template library index.
type LibraryEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	URL         string   `json:"url"` // raw URL to the template JSON
}

// libraryCache holds a short-lived in-memory cache for the index fetch so rapid
// re-opens of the library panel don't hammer the upstream server.
type libraryCache struct {
	mu        sync.Mutex
	entries   []LibraryEntry
	fetchedAt time.Time
	ttl       time.Duration
}

func newLibraryCache(ttl time.Duration) *libraryCache {
	return &libraryCache{ttl: ttl}
}

func (c *libraryCache) get() ([]LibraryEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil || time.Since(c.fetchedAt) > c.ttl {
		return nil, false
	}
	return c.entries, true
}

func (c *libraryCache) set(entries []LibraryEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = entries
	c.fetchedAt = time.Now()
}

// getLibraryIndex fetches the community template library index from the
// configured URL and returns the list of available templates.
//
// The handler is read-only and does not require the engine to be idle.
func (s *Server) getLibraryIndex(w http.ResponseWriter, r *http.Request) {
	if entries, ok := s.libCache.get(); ok {
		writeOK(w, entries)
		return
	}

	indexURL := s.cfg.Server.LibraryIndexURL
	if indexURL == "" {
		indexURL = "https://raw.githubusercontent.com/danieljustus/symvibe-templates/main/index.json"
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, indexURL, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "build request: "+err.Error())
		return
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "fetch library index: "+err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		writeErr(w, http.StatusBadGateway, "library index returned HTTP "+http.StatusText(resp.StatusCode))
		return
	}

	var entries []LibraryEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		writeErr(w, http.StatusBadGateway, "decode library index: "+err.Error())
		return
	}

	s.libCache.set(entries)
	writeOK(w, entries)
}
