package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hnBaseURL = "https://hacker-news.firebaseio.com/v0"
	hnSearchURL = "https://hn.algolia.com/api/v1"
)

// Story represents a Hacker News story
type Story struct {
	ID           int    `json:"id"`
	By           string `json:"by,omitempty"`
	Descendants  int    `json:"descendants,omitempty"`
	Score        int    `json:"score,omitempty"`
	Time         int64  `json:"time,omitempty"`
	Title        string `json:"title,omitempty"`
	URL          string `json:"url,omitempty"`
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	Dead         bool   `json:"dead,omitempty"`
	Deleted      bool   `json:"deleted,omitempty"`
}

// Comment represents a nested comment
type Comment struct {
	ID       int        `json:"id"`
	Author   string     `json:"author,omitempty"`
	Time     int64      `json:"time,omitempty"`
	Content  string     `json:"content,omitempty"`
	Children []*Comment `json:"children,omitempty"`
}

// Cache with TTL
type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

var (
	cache     = make(map[string]*cacheEntry)
	cacheMu   sync.RWMutex
	cacheTTL  = 60 * time.Second
)

func getCache(key string) (interface{}, bool) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	entry, ok := cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func setCache(key string, data interface{}) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache[key] = &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(cacheTTL),
	}
}

// HTTP client with timeout
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// fetchHN fetches JSON from the HN Firebase API
func fetchHN(path string, target interface{}) error {
	url := hnBaseURL + path + ".json"
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HN API returned %d for %s", resp.StatusCode, url)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// fetchStory fetches a single story by ID
func fetchStory(id int) (*Story, error) {
	var story Story
	err := fetchHN(fmt.Sprintf("/item/%d", id), &story)
	if err != nil {
		return nil, err
	}
	return &story, nil
}

// fetchStories fetches multiple stories by IDs
func fetchStories(ids []int, allowJobs bool) ([]*Story, error) {
	stories := make([]*Story, 0, len(ids))

	type result struct {
		story *Story
	}

	sem := make(chan struct{}, 10)
	results := make(chan result, len(ids))

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var story Story
			if err := fetchHN(fmt.Sprintf("/item/%d", id), &story); err == nil {
				if !story.Deleted && !story.Dead && (story.Type == "story" || (allowJobs && story.Type == "job")) {
					results <- result{story: &story}
				}
			}
		}(id)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		stories = append(stories, r.story)
	}

	return stories, nil
}

// algoliaStory represents a story from Algolia with embedded comment tree
type algoliaStory struct {
	ID       int              `json:"id"`
	Children []algoliaComment `json:"children"`
}

type algoliaComment struct {
	ID        int              `json:"id"`
	Author    string           `json:"author"`
	CreatedAt string           `json:"created_at"`
	CreatedAtI int64          `json:"created_at_i"`
	Text      string           `json:"text"`
	Children  []algoliaComment `json:"children"`
}

// fetchCommentsFromAlgolia fetches the full comment tree from Algolia
func fetchCommentsFromAlgolia(storyID int) []*Comment {
	url := fmt.Sprintf("%s/items/%d", hnSearchURL, storyID)
	resp, err := httpClient.Get(url)
	if err != nil {
		log.Printf("Algolia fetch error for story %d: %v", storyID, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Algolia returned %d for story %d", resp.StatusCode, storyID)
		return nil
	}

	var story algoliaStory
	if err := json.NewDecoder(resp.Body).Decode(&story); err != nil {
		log.Printf("Algolia decode error for story %d: %v", storyID, err)
		return nil
	}

	return convertAlgoliaComments(story.Children)
}

// convertAlgoliaComments converts Algolia comments to our Comment format
func convertAlgoliaComments(comments []algoliaComment) []*Comment {
	result := make([]*Comment, 0, len(comments))
	for _, c := range comments {
		converted := &Comment{
			ID:      c.ID,
			Author:  c.Author,
			Time:    c.CreatedAtI,
			Content: formatAlgoliaCommentText(c.Text),
		}
		if len(c.Children) > 0 {
			converted.Children = convertAlgoliaComments(c.Children)
		}
		result = append(result, converted)
	}
	return result
}

// formatAlgoliaCommentText handles Algolia's pre-formatted HTML comments
func formatAlgoliaCommentText(text string) string {
	if text == "" {
		return ""
	}

	// Algolia returns HTML with escaped entities
	// Convert HTML entities back to characters for proper display
	text = strings.ReplaceAll(text, "&#x2F;", "/")
	text = strings.ReplaceAll(text, "&#x27;", "'")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")

	// Clean up Algolia's nofollow links
	text = strings.ReplaceAll(text, `rel="nofollow"`, `target="_blank" rel="noopener"`)

	return text
}

// formatCommentText converts HN's text format to HTML
func formatCommentText(text string) string {
	if text == "" {
		return ""
	}

	// If it's already HTML (HN sometimes returns HTML)
	if strings.HasPrefix(strings.TrimSpace(text), "<") {
		return text
	}

	// Escape HTML entities
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	// Convert code blocks (lines starting with 4+ spaces)
	lines := strings.Split(text, "\n")
	var result []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		leadingSpaces := len(line) - len(trimmed)

		if leadingSpaces >= 4 && trimmed != "" {
			if !inCodeBlock {
				result = append(result, "<pre><code>")
				inCodeBlock = true
			}
			result = append(result, strings.TrimLeft(line, " \t"))
		} else {
			if inCodeBlock {
				result = append(result, "</code></pre>")
				inCodeBlock = false
			}
		}
	}
	if inCodeBlock {
		result = append(result, "</code></pre>")
	}

	text = strings.Join(result, "\n")

	// Convert inline code (backticks)
	convertInlineCode := func(s string) string {
		parts := strings.Split(s, "`")
		if len(parts) <= 1 {
			return s
		}
		var b strings.Builder
		for i, part := range parts {
			if i%2 == 1 {
				b.WriteString("<code>")
				b.WriteString(part)
				b.WriteString("</code>")
			} else {
				b.WriteString(part)
			}
		}
		return b.String()
	}
	text = convertInlineCode(text)

	// Convert URLs to links
	text = convertURLsToLinks(text)

	// Wrap in paragraphs
	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) > 1 {
		var wrapped strings.Builder
		for _, p := range paragraphs {
			p = strings.TrimSpace(p)
			if p != "" && !strings.HasPrefix(p, "<pre") {
				wrapped.WriteString("<p>")
				wrapped.WriteString(strings.ReplaceAll(p, "\n", "<br>"))
				wrapped.WriteString("</p>")
			} else if p != "" {
				wrapped.WriteString(p)
			}
		}
		text = wrapped.String()
	} else {
		text = strings.ReplaceAll(text, "\n", "<br>")
	}

	return text
}

// convertURLsToLinks converts bare URLs to anchor tags
func convertURLsToLinks(text string) string {
	var result strings.Builder
	i := 0
	for i < len(text) {
		if strings.HasPrefix(text[i:], "http://") || strings.HasPrefix(text[i:], "https://") {
			start := i
			for i < len(text) && !isURLDelimiter(text[i]) {
				i++
			}
			urlStr := text[start:i]
			// Clean trailing punctuation
			for len(urlStr) > 0 && strings.Contains(".)>,;:!", string(urlStr[len(urlStr)-1])) {
				i--
				urlStr = urlStr[:len(urlStr)-1]
			}
			result.WriteString(fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener">%s</a>`, urlStr, urlStr))
		} else {
			result.WriteByte(text[i])
			i++
		}
	}
	return result.String()
}

func isURLDelimiter(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t' || b == '"' || b == '\'' || b == '<'
}

// Handler returns an HTTP handler for the API routes
func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/stories", handleStories)
	mux.HandleFunc("/api/story/", handleStory)
	mux.HandleFunc("/api/comments/", handleComments)
	mux.HandleFunc("/api/search", handleSearch)

	return mux
}

// handleStories returns a list of stories for a given type
func handleStories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	storyType := r.URL.Query().Get("type")
	if storyType == "" {
		storyType = "top"
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Get story IDs
	var ids []int
	cacheKey := fmt.Sprintf("ids:%s", storyType)
	if cached, ok := getCache(cacheKey); ok {
		ids = cached.([]int)
	} else {
		var endpoint string
		switch storyType {
		case "top":
			endpoint = "/topstories"
		case "new":
			endpoint = "/newstories"
		case "show":
			endpoint = "/showstories"
		case "ask":
			endpoint = "/beststories"
		case "jobs":
			endpoint = "/jobstories"
		default:
			endpoint = "/topstories"
		}

		err := fetchHN(endpoint, &ids)
		if err != nil {
			log.Printf("Error fetching %s stories: %v", storyType, err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ids":     []int{},
				"stories": []*Story{},
				"hasMore": false,
			})
			return
		}
		setCache(cacheKey, ids)
	}

	// Paginate
	offset := (page - 1) * limit
	end := offset + limit
	if offset >= len(ids) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":     []int{},
			"stories": []*Story{},
			"hasMore": false,
		})
		return
	}

	if end > len(ids) {
		end = len(ids)
	}

	pageIDs := ids[offset:end]
	stories, _ := fetchStories(pageIDs, storyType == "jobs")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"ids":     pageIDs,
		"stories": stories,
		"hasMore": end < len(ids),
	})
}

// handleStory returns a single story by ID
func handleStory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	idStr := strings.TrimPrefix(r.URL.Path, "/api/story/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid story ID", http.StatusBadRequest)
		return
	}

	cacheKey := fmt.Sprintf("story:%d", id)
	if cached, ok := getCache(cacheKey); ok {
		json.NewEncoder(w).Encode(cached)
		return
	}

	story, err := fetchStory(id)
	if err != nil {
		log.Printf("Error fetching story %d: %v", id, err)
		http.Error(w, "Story not found", http.StatusNotFound)
		return
	}

	setCache(cacheKey, story)
	json.NewEncoder(w).Encode(story)
}

// handleComments returns the comment tree for a story
func handleComments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	idStr := strings.TrimPrefix(r.URL.Path, "/api/comments/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid story ID", http.StatusBadRequest)
		return
	}

	cacheKey := fmt.Sprintf("comments:%d", id)
	if cached, ok := getCache(cacheKey); ok {
		json.NewEncoder(w).Encode(cached)
		return
	}

	comments := fetchCommentsFromAlgolia(id)

	response := map[string]interface{}{
		"comments": comments,
	}
	if comments == nil {
		response["comments"] = []*Comment{}
	}

	setCache(cacheKey, response)
	json.NewEncoder(w).Encode(response)
}

// handleSearch searches for stories using Algolia HN API
type algoliaResponse struct {
	Hits []algoliaHit `json:"hits"`
}

type algoliaHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url,omitempty"`
	Author      string `json:"author,omitempty"`
	Points      int    `json:"points,omitempty"`
	NumComments int    `json:"num_comments,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stories": []*Story{},
		})
		return
	}

	// Check cache
	cacheKey := fmt.Sprintf("search:%s", url.QueryEscape(query))
	if cached, ok := getCache(cacheKey); ok {
		json.NewEncoder(w).Encode(cached)
		return
	}

	// Use Algolia HN API
	apiURL := fmt.Sprintf("%s/search?query=%s&tags=story&hitsPerPage=30", hnSearchURL, url.QueryEscape(query))
	resp, err := httpClient.Get(apiURL)
	if err != nil {
		log.Printf("Search error: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stories": []*Story{},
		})
		return
	}
	defer resp.Body.Close()

	var algoliaResp algoliaResponse
	if err := json.NewDecoder(resp.Body).Decode(&algoliaResp); err != nil {
		log.Printf("Search decode error: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stories": []*Story{},
		})
		return
	}

	// Convert to our Story format
	stories := make([]*Story, 0, len(algoliaResp.Hits))
	for _, hit := range algoliaResp.Hits {
		id, _ := strconv.Atoi(hit.ObjectID)

		// Parse created_at to unix timestamp
		var ts int64
		if t, err := time.Parse("2006-01-02T15:04:05ZX", hit.CreatedAt); err == nil {
			ts = t.Unix()
		}

		stories = append(stories, &Story{
			ID:          id,
			By:          hit.Author,
			Descendants: hit.NumComments,
			Score:       hit.Points,
			Time:        ts,
			Title:       hit.Title,
			URL:         hit.URL,
			Type:        "story",
		})
	}

	response := map[string]interface{}{
		"stories": stories,
	}
	if stories == nil {
		response["stories"] = []*Story{}
	}

	setCache(cacheKey, response)
	json.NewEncoder(w).Encode(response)
}
