package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/store"
)

// LibraryHandler handles library-related HTTP requests.
type LibraryHandler struct {
	authHandler *AuthHandler
	templates   *template.Template
	cacheStore  *store.CacheStore
}

// NewLibraryHandler creates a new library handler.
func NewLibraryHandler(authHandler *AuthHandler, templates *template.Template, cacheStore *store.CacheStore) *LibraryHandler {
	return &LibraryHandler{
		authHandler: authHandler,
		templates:   templates,
		cacheStore:  cacheStore,
	}
}

// HandleLibraries redirects to the first audiobook library (or shows list if no libraries).
// The UI handles library selection via the header dropdown.
func (h *LibraryHandler) HandleLibraries(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	libraries, err := absClient.GetLibraries(ctx)
	if err != nil {
		if err == abs.ErrUnauthorized {
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}
		http.Error(w, "Failed to fetch libraries", http.StatusInternalServerError)
		return
	}

	// Find first audiobook library
	selectedLibraryID := ""
	for _, lib := range libraries {
		if lib.MediaType == "book" {
			selectedLibraryID = lib.ID
			break
		}
	}
	if selectedLibraryID == "" && len(libraries) > 0 {
		selectedLibraryID = libraries[0].ID
	}

	// Auto-redirect to the first library's items page
	if selectedLibraryID != "" {
		http.Redirect(w, r, "/libraries/"+selectedLibraryID+"/items", http.StatusSeeOther)
		return
	}

	// Only show library list if no libraries exist (edge case)
	data := map[string]interface{}{
		"Title":             "Bibliotheken",
		"ShowHeader":        true,
		"Username":          session.ABSUsername,
		"Libraries":         libraries,
		"SelectedLibraryID": selectedLibraryID,
		"ActiveTab":         "",
	}

	h.render(w, "library.html", data)
}

// HandleLibraryItems renders the items in a library.
func (h *LibraryHandler) HandleLibraryItems(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	libraryID := r.PathValue("id")
	if libraryID == "" {
		http.Error(w, "Library ID required", http.StatusBadRequest)
		return
	}

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Parse query parameters
	query := r.URL.Query().Get("q")
	filter := r.URL.Query().Get("filter")
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "title-asc"
	}
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "grid"
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	// Map sort parameter to ABS sort field
	// Format: "field-direction" (e.g., "title-asc", "added-desc")
	sortField := "media.metadata.title"
	sortDesc := false

	// Parse sort parameter
	sortParts := strings.Split(sort, "-")
	sortType := sortParts[0]
	if len(sortParts) > 1 && sortParts[len(sortParts)-1] == "desc" {
		sortDesc = true
	}

	switch sortType {
	case "title":
		sortField = "media.metadata.title"
	case "author":
		sortField = "media.metadata.authorName"
	case "recent":
		sortField = "progress"
	case "added":
		sortField = "addedAt"
	case "duration":
		sortField = "media.duration"
	case "published":
		sortField = "media.metadata.publishedYear"
	}

	// Fetch items - use search endpoint if there's a query
	var itemsResp *abs.ItemsResponse

	if query != "" {
		// Use dedicated search endpoint for text queries
		itemsResp, err = absClient.SearchLibrary(ctx, libraryID, query, limit)
	} else {
		// Use regular items endpoint with sorting/filtering
		opts := abs.ItemsOptions{
			Limit:   limit,
			Page:    offset / limit,
			Sort:    sortField,
			Desc:    sortDesc,
			Filter:  filter,
			Include: "progress",
		}
		itemsResp, err = absClient.GetLibraryItems(ctx, libraryID, opts)
	}

	if err != nil {
		if err == abs.ErrUnauthorized {
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}
		http.Error(w, "Failed to fetch items", http.StatusInternalServerError)
		return
	}

	// Get libraries for navigation
	libraries, _ := absClient.GetLibraries(ctx)
	libraryName := "Library"
	for _, lib := range libraries {
		if lib.ID == libraryID {
			libraryName = lib.Name
			break
		}
	}

	// Convert items to simplified format
	items := make([]SimplifiedItem, len(itemsResp.Results))
	for i, item := range itemsResp.Results {
		items[i] = convertItem(&item)
	}

	hasMore := offset+len(items) < itemsResp.Total

	data := map[string]interface{}{
		"Title":             libraryName,
		"ShowHeader":        true,
		"Username":          session.ABSUsername,
		"LibraryID":         libraryID,
		"LibraryName":       libraryName,
		"Items":             items,
		"Total":             itemsResp.Total,
		"Query":             query,
		"Sort":              sort,
		"View":              view,
		"Limit":             limit,
		"Offset":            offset,
		"NextOffset":        offset + limit,
		"HasMore":           hasMore,
		"Libraries":         libraries,
		"SelectedLibraryID": libraryID,
		"ActiveTab":         "all",
	}

	// If htmx request for partial update
	if r.Header.Get("HX-Request") == "true" {
		// Check if this is an append request (Load More button)
		if r.URL.Query().Get("append") == "1" {
			h.renderPartial(w, "item-cards-append", data)
		} else {
			h.renderPartial(w, "item-grid", data)
		}
		return
	}

	h.render(w, "items.html", data)
}

// HandleCover proxies cover image requests to Audiobookshelf.
func (h *LibraryHandler) HandleCover(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	itemID := r.PathValue("id")
	if itemID == "" {
		http.Error(w, "Item ID required", http.StatusBadRequest)
		return
	}

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	body, contentType, err := absClient.GetCover(ctx, itemID)
	if err != nil {
		if err == abs.ErrNotFound {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch cover", http.StatusInternalServerError)
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 1 day
	io.Copy(w, body)
}

// HandleFilterData returns filter options for a library.
func (h *LibraryHandler) HandleFilterData(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	libraryID := r.PathValue("id")
	if libraryID == "" {
		http.Error(w, "Library ID required", http.StatusBadRequest)
		return
	}

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filterData, err := absClient.GetFilterData(ctx, libraryID)
	if err != nil {
		if err == abs.ErrUnauthorized {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Error(w, "Failed to fetch filter data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filterData)
}

// HandleRecent renders the "Recently Played" page with items from all libraries.
func (h *LibraryHandler) HandleRecent(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Fetch items in progress
	itemsInProgress, err := absClient.GetItemsInProgress(ctx, 50)
	if err != nil {
		if err == abs.ErrUnauthorized {
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}
		slog.Error("failed to fetch items in progress", "error", err)
		http.Error(w, "Failed to fetch recent items", http.StatusInternalServerError)
		return
	}

	// Get libraries for navigation and library name lookup
	libraries, _ := absClient.GetLibraries(ctx)
	libraryNameMap := make(map[string]string)
	for _, lib := range libraries {
		libraryNameMap[lib.ID] = lib.Name
	}

	// Set default library ID
	selectedLibraryID := ""
	for _, lib := range libraries {
		if lib.MediaType == "book" {
			selectedLibraryID = lib.ID
			break
		}
	}
	if selectedLibraryID == "" && len(libraries) > 0 {
		selectedLibraryID = libraries[0].ID
	}

	// Convert to RecentItem format
	items := make([]RecentItem, len(itemsInProgress))
	for i, item := range itemsInProgress {
		author := ""
		if len(item.Media.Metadata.Authors) > 0 {
			author = item.Media.Metadata.Authors[0].Name
		}
		items[i] = RecentItem{
			ID:          item.ID,
			Title:       item.Media.Metadata.Title,
			Author:      author,
			CoverURL:    fmt.Sprintf("/cover/%s", item.ID),
			DurationSec: int(item.Media.Duration),
			LibraryID:   item.LibraryID,
			LibraryName: libraryNameMap[item.LibraryID],
			LastPlayed:  item.ProgressLastUpdate,
		}
	}

	data := map[string]interface{}{
		"Title":             "Zuletzt gehört",
		"ShowHeader":        true,
		"Username":          session.ABSUsername,
		"Items":             items,
		"Libraries":         libraries,
		"SelectedLibraryID": selectedLibraryID,
		"ActiveTab":         "recent",
	}

	h.render(w, "recent.html", data)
}

// RecentItem represents an item in the "Recently Played" list.
type RecentItem struct {
	ID          string
	Title       string
	Author      string
	CoverURL    string
	DurationSec int
	LibraryID   string
	LibraryName string
	LastPlayed  int64 // Unix timestamp
}

// HandleSeries renders the series list page with composite covers.
func (h *LibraryHandler) HandleSeries(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get library ID from query parameter
	libraryID := r.URL.Query().Get("library")

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get libraries for navigation
	libraries, _ := absClient.GetLibraries(ctx)

	// Use first book library if none specified
	if libraryID == "" {
		for _, lib := range libraries {
			if lib.MediaType == "book" {
				libraryID = lib.ID
				break
			}
		}
		if libraryID == "" && len(libraries) > 0 {
			libraryID = libraries[0].ID
		}
	}

	// Get library name
	libraryName := "Bibliothek"
	for _, lib := range libraries {
		if lib.ID == libraryID {
			libraryName = lib.Name
			break
		}
	}

	// Get filter data for series list
	filterData, err := absClient.GetFilterData(ctx, libraryID)
	if err != nil {
		if err == abs.ErrUnauthorized {
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}
		slog.Error("failed to fetch filter data", "error", err)
		http.Error(w, "Failed to fetch series", http.StatusInternalServerError)
		return
	}


	// Build series items with composite covers (limit to first 4 book covers per series)
	seriesItems := make([]SeriesItem, 0, len(filterData.Series))
	for _, s := range filterData.Series {
		if s.Name == "" {
			continue
		}

		// Fetch books for this series (limit to 4 for composite cover)
		// ABS uses URL-safe base64 encoding for filter values
		encodedID := base64.URLEncoding.EncodeToString([]byte(s.ID))
		filter := fmt.Sprintf("series.%s", encodedID)

		booksResp, err := absClient.GetLibraryItems(ctx, libraryID, abs.ItemsOptions{
			Filter: filter,
			Limit:  4,
			Sort:   "media.metadata.title",
		})
		if err != nil {
			continue
		}

		// Build cover URLs for composite
		coverURLs := make([]string, 0, len(booksResp.Results))
		for _, book := range booksResp.Results {
			if book.Media.CoverPath != "" {
				coverURLs = append(coverURLs, fmt.Sprintf("/cover/%s", book.ID))
			}
		}

		seriesItems = append(seriesItems, SeriesItem{
			ID:        s.ID,
			Name:      s.Name,
			BookCount: booksResp.Total,
			CoverURLs: coverURLs,
		})
	}

	data := map[string]interface{}{
		"Title":             "Serien - " + libraryName,
		"ShowHeader":        true,
		"Username":          session.ABSUsername,
		"Series":            seriesItems,
		"Libraries":         libraries,
		"SelectedLibraryID": libraryID,
		"LibraryName":       libraryName,
		"ActiveTab":         "series",
	}

	h.render(w, "series.html", data)
}

// SeriesItem represents a series with composite cover data.
type SeriesItem struct {
	ID        string
	Name      string
	BookCount int
	CoverURLs []string // Up to 4 cover URLs for composite
}

// HandleSeriesDetail renders the detail page for a single series showing all books.
func (h *LibraryHandler) HandleSeriesDetail(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	seriesID := r.PathValue("id")
	if seriesID == "" {
		http.Error(w, "Series ID required", http.StatusBadRequest)
		return
	}

	libraryID := r.URL.Query().Get("library")

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get libraries for navigation
	libraries, _ := absClient.GetLibraries(ctx)

	// Use first book library if none specified
	if libraryID == "" {
		for _, lib := range libraries {
			if lib.MediaType == "book" {
				libraryID = lib.ID
				break
			}
		}
		if libraryID == "" && len(libraries) > 0 {
			libraryID = libraries[0].ID
		}
	}

	// Get library name
	libraryName := "Bibliothek"
	for _, lib := range libraries {
		if lib.ID == libraryID {
			libraryName = lib.Name
			break
		}
	}

	// Get series name from filter data
	filterData, err := absClient.GetFilterData(ctx, libraryID)
	if err != nil {
		if err == abs.ErrUnauthorized {
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}
		http.Error(w, "Failed to fetch series data", http.StatusInternalServerError)
		return
	}

	seriesName := "Serie"
	for _, s := range filterData.Series {
		if s.ID == seriesID {
			seriesName = s.Name
			break
		}
	}

	// Fetch all books in the series
	encodedID := base64.URLEncoding.EncodeToString([]byte(seriesID))
	filter := fmt.Sprintf("series.%s", encodedID)

	booksResp, err := absClient.GetLibraryItems(ctx, libraryID, abs.ItemsOptions{
		Filter: filter,
		Limit:  100,
		Sort:   "media.metadata.title",
	})
	if err != nil {
		if err == abs.ErrUnauthorized {
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}
		http.Error(w, "Failed to fetch series books", http.StatusInternalServerError)
		return
	}

	// Convert to template-friendly format with sequence info
	type SeriesBook struct {
		ID          string
		Title       string
		Author      string
		CoverURL    string
		DurationSec int
		Sequence    string
	}

	books := make([]SeriesBook, 0, len(booksResp.Results))
	for _, item := range booksResp.Results {
		author := ""
		if len(item.Media.Metadata.Authors) > 0 {
			author = item.Media.Metadata.Authors[0].Name
		}

		// Get sequence number from series metadata
		sequence := ""
		for _, s := range item.Media.Metadata.Series {
			if s.ID == seriesID {
				sequence = s.Sequence
				break
			}
		}

		books = append(books, SeriesBook{
			ID:          item.ID,
			Title:       item.Media.Metadata.Title,
			Author:      author,
			CoverURL:    fmt.Sprintf("/cover/%s", item.ID),
			DurationSec: int(item.Media.Duration),
			Sequence:    sequence,
		})
	}

	// Sort by sequence if available (simple numeric sort)
	sort.Slice(books, func(i, j int) bool {
		// Parse sequences as numbers for comparison
		seqI := parseSequence(books[i].Sequence)
		seqJ := parseSequence(books[j].Sequence)
		if seqI != seqJ {
			return seqI < seqJ
		}
		// Fall back to title sort
		return books[i].Title < books[j].Title
	})

	data := map[string]interface{}{
		"Title":             seriesName + " - " + libraryName,
		"ShowHeader":        true,
		"Username":          session.ABSUsername,
		"SeriesName":        seriesName,
		"SeriesID":          seriesID,
		"Books":             books,
		"BookCount":         len(books),
		"Libraries":         libraries,
		"SelectedLibraryID": libraryID,
		"LibraryName":       libraryName,
		"ActiveTab":         "series",
	}

	h.render(w, "series-detail.html", data)
}

// parseSequence converts a sequence string to a sortable float.
func parseSequence(seq string) float64 {
	if seq == "" {
		return 999999
	}
	f, err := strconv.ParseFloat(seq, 64)
	if err != nil {
		return 999999
	}
	return f
}

// AuthorItem represents an author with book count.
type AuthorItem struct {
	ID        string
	Name      string
	BookCount int
}

// HandleAuthors renders the authors list page.
func (h *LibraryHandler) HandleAuthors(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	libraryID := r.URL.Query().Get("library")

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	libraries, _ := absClient.GetLibraries(ctx)

	if libraryID == "" {
		for _, lib := range libraries {
			if lib.MediaType == "book" {
				libraryID = lib.ID
				break
			}
		}
		if libraryID == "" && len(libraries) > 0 {
			libraryID = libraries[0].ID
		}
	}

	libraryName := "Bibliothek"
	for _, lib := range libraries {
		if lib.ID == libraryID {
			libraryName = lib.Name
			break
		}
	}

	filterData, err := absClient.GetFilterData(ctx, libraryID)
	if err != nil {
		if err == abs.ErrUnauthorized {
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}
		slog.Error("failed to fetch filter data", "error", err)
		http.Error(w, "Failed to fetch authors", http.StatusInternalServerError)
		return
	}

	// Build author items sorted by name
	authors := make([]AuthorItem, 0, len(filterData.Authors))
	for _, a := range filterData.Authors {
		if a.Name == "" {
			continue
		}
		authors = append(authors, AuthorItem{
			ID:        a.ID,
			Name:      a.Name,
			BookCount: 0, // We don't have book counts in filter data
		})
	}

	// Sort by name
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].Name < authors[j].Name
	})

	data := map[string]interface{}{
		"Title":             "Autoren - " + libraryName,
		"ShowHeader":        true,
		"Username":          session.ABSUsername,
		"Authors":           authors,
		"Libraries":         libraries,
		"SelectedLibraryID": libraryID,
		"LibraryName":       libraryName,
		"ActiveTab":         "authors",
	}

	h.render(w, "authors.html", data)
}

// GenreItem represents a genre with book count.
type GenreItem struct {
	Name      string
	BookCount int
}

// HandleGenres renders the genres list page.
func (h *LibraryHandler) HandleGenres(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	libraryID := r.URL.Query().Get("library")

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	libraries, _ := absClient.GetLibraries(ctx)

	if libraryID == "" {
		for _, lib := range libraries {
			if lib.MediaType == "book" {
				libraryID = lib.ID
				break
			}
		}
		if libraryID == "" && len(libraries) > 0 {
			libraryID = libraries[0].ID
		}
	}

	libraryName := "Bibliothek"
	for _, lib := range libraries {
		if lib.ID == libraryID {
			libraryName = lib.Name
			break
		}
	}

	filterData, err := absClient.GetFilterData(ctx, libraryID)
	if err != nil {
		if err == abs.ErrUnauthorized {
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}
		slog.Error("failed to fetch filter data", "error", err)
		http.Error(w, "Failed to fetch genres", http.StatusInternalServerError)
		return
	}

	// Build genre items sorted by name
	genres := make([]GenreItem, 0, len(filterData.Genres))
	for _, g := range filterData.Genres {
		if g == "" {
			continue
		}
		genres = append(genres, GenreItem{
			Name:      g,
			BookCount: 0, // We don't have book counts in filter data
		})
	}

	// Sort by name
	sort.Slice(genres, func(i, j int) bool {
		return genres[i].Name < genres[j].Name
	})

	data := map[string]interface{}{
		"Title":             "Genres - " + libraryName,
		"ShowHeader":        true,
		"Username":          session.ABSUsername,
		"Genres":            genres,
		"Libraries":         libraries,
		"SelectedLibraryID": libraryID,
		"LibraryName":       libraryName,
		"ActiveTab":         "genres",
	}

	h.render(w, "genres.html", data)
}

// HandleItem renders the item detail page.
func (h *LibraryHandler) HandleItem(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	itemID := r.PathValue("id")
	if itemID == "" {
		http.Error(w, "Item ID required", http.StatusBadRequest)
		return
	}

	absClient, err := h.authHandler.GetABSClientForSession(session)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Fetch item details
	item, err := absClient.GetItem(ctx, itemID)
	if err != nil {
		if err == abs.ErrUnauthorized {
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}
		if err == abs.ErrNotFound {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to fetch item", http.StatusInternalServerError)
		return
	}

	// Get progress
	progress, _ := absClient.GetProgress(ctx, itemID)
	progressPercent := 0.0
	if progress != nil {
		progressPercent = progress.Progress
	}

	// Calculate total duration (fallback to AudioFiles sum if Media.Duration is 0)
	totalDuration := item.Media.Duration
	if totalDuration == 0 && len(item.Media.AudioFiles) > 0 {
		for _, af := range item.Media.AudioFiles {
			totalDuration += af.Duration
		}
	}

	// Calculate remaining time
	progressPct := int(progressPercent * 100)
	remainingSec := 0
	if totalDuration > 0 {
		remainingSec = int(totalDuration * (1.0 - progressPercent))
	}

	// Build simplified item with description
	simplifiedItem := DetailedItem{
		ID:           item.ID,
		LibraryID:    item.LibraryID,
		Title:        item.Media.Metadata.Title,
		Author:       getAuthorName(item),
		Description:  item.Media.Metadata.Description,
		CoverURL:     fmt.Sprintf("/cover/%s", item.ID),
		DurationSec:  int(totalDuration),
		Progress:     progressPercent,
		ProgressPct:  progressPct,
		RemainingSec: remainingSec,
	}

	data := map[string]interface{}{
		"Title":      item.Media.Metadata.Title,
		"ShowHeader": true,
		"Username":   session.ABSUsername,
		"Item":       simplifiedItem,
		"LibraryID":  item.LibraryID,
	}

	h.render(w, "item.html", data)
}

// DetailedItem is an item with full details for the detail page.
type DetailedItem struct {
	ID           string
	LibraryID    string
	Title        string
	Author       string
	Description  string
	CoverURL     string
	DurationSec  int
	Progress     float64
	ProgressPct  int // Progress as percentage (0-100)
	RemainingSec int // Remaining seconds to listen
}

func getAuthorName(item *abs.LibraryItem) string {
	if len(item.Media.Metadata.Authors) > 0 {
		return item.Media.Metadata.Authors[0].Name
	}
	return ""
}

// SimplifiedItem is a simplified item for templates.
type SimplifiedItem struct {
	ID          string
	Title       string
	Author      string
	CoverURL    string
	DurationSec int
	Progress    float64
}

func convertItem(item *abs.LibraryItem) SimplifiedItem {
	author := ""
	if len(item.Media.Metadata.Authors) > 0 {
		author = item.Media.Metadata.Authors[0].Name
	}

	return SimplifiedItem{
		ID:          item.ID,
		Title:       item.Media.Metadata.Title,
		Author:      author,
		CoverURL:    fmt.Sprintf("/cover/%s", item.ID),
		DurationSec: int(item.Media.Duration),
	}
}

func (h *LibraryHandler) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Parse templates fresh for each request to avoid Clone issues
	funcMap := template.FuncMap{
		"formatDuration": func(seconds int) string {
			hours := seconds / 3600
			minutes := (seconds % 3600) / 60
			if hours > 0 {
				if minutes > 0 {
					return fmt.Sprintf("%d hr %d min", hours, minutes)
				}
				return fmt.Sprintf("%d hr", hours)
			}
			if minutes > 0 {
				return fmt.Sprintf("%d min", minutes)
			}
			return "< 1 min"
		},
		"mult": func(a, b float64) float64 { return a * b },
		"progressPercent": func(position, duration int) float64 {
			if duration == 0 {
				return 0
			}
			return float64(position) / float64(duration) * 100
		},
		"plus1": func(i int) int { return i + 1 },
		"minus": func(a, b int) int { return a - b },
		"json": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("[]")
			}
			return template.JS(b)
		},
		"base64": func(s string) string {
			return base64.URLEncoding.EncodeToString([]byte(s))
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob("web/templates/layout.html")
	if err != nil {
		slog.Error("template parse error", "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	tmpl, err = tmpl.ParseGlob("web/templates/partials/*.html")
	if err != nil {
		slog.Error("template parse partials error", "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	tmpl, err = tmpl.ParseFiles("web/templates/" + name)
	if err != nil {
		slog.Error("template parse page error", "file", name, "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("template execute error", "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *LibraryHandler) renderPartial(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
