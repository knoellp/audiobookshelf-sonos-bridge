package abs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Login_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req.Username != "testuser" || req.Password != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp := struct {
			User User `json:"user"`
		}{
			User: User{
				ID:       "user-123",
				Username: "testuser",
				Type:     "admin",
				Token:    "auth-token-xyz",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	user, err := client.Login(context.Background(), "testuser", "testpass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.ID != "user-123" {
		t.Errorf("expected user ID 'user-123', got '%s'", user.ID)
	}
	if user.Token != "auth-token-xyz" {
		t.Errorf("expected token 'auth-token-xyz', got '%s'", user.Token)
	}
}

func TestClient_Login_InvalidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Login(context.Background(), "wrong", "wrong")

	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestClient_GetLibraries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp := LibrariesResponse{
			Libraries: []Library{
				{ID: "lib-1", Name: "Audiobooks", MediaType: "book"},
				{ID: "lib-2", Name: "Podcasts", MediaType: "podcast"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL).WithToken("test-token")
	libs, err := client.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(libs) != 2 {
		t.Errorf("expected 2 libraries, got %d", len(libs))
	}
	if libs[0].Name != "Audiobooks" {
		t.Errorf("expected first library 'Audiobooks', got '%s'", libs[0].Name)
	}
}

func TestClient_GetLibraryItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/libraries/lib-1/items") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Check query params
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", r.URL.Query().Get("limit"))
		}

		resp := ItemsResponse{
			Results: []LibraryItem{
				{
					ID: "item-1",
					Media: BookMedia{
						Metadata: BookMetadata{
							Title: "Test Book",
							Authors: []Author{{Name: "Test Author"}},
						},
						Duration: 3600,
					},
				},
			},
			Total: 1,
			Limit: 10,
			Page:  0,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL).WithToken("test-token")
	items, err := client.GetLibraryItems(context.Background(), "lib-1", ItemsOptions{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items.Results) != 1 {
		t.Errorf("expected 1 item, got %d", len(items.Results))
	}
	if items.Results[0].Media.Metadata.Title != "Test Book" {
		t.Errorf("expected title 'Test Book', got '%s'", items.Results[0].Media.Metadata.Title)
	}
}

func TestClient_GetFilterData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/libraries/lib-1/filterdata") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := FilterData{
			Authors:   []FilterAuthor{{ID: "auth-1", Name: "Author One"}},
			Series:    []FilterSeries{{ID: "ser-1", Name: "Series One"}},
			Genres:    []string{"Fiction", "Fantasy"},
			Tags:      []string{"favorite"},
			Narrators: []string{"Narrator One"},
			Languages: []string{"English"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL).WithToken("test-token")
	data, err := client.GetFilterData(context.Background(), "lib-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.Authors) != 1 {
		t.Errorf("expected 1 author, got %d", len(data.Authors))
	}
	if len(data.Genres) != 2 {
		t.Errorf("expected 2 genres, got %d", len(data.Genres))
	}
}

func TestClient_GetProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/me/progress/item-123") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := Progress{
			ID:            "progress-1",
			LibraryItemID: "item-123",
			Duration:      3600,
			Progress:      0.5,
			CurrentTime:   1800,
			IsFinished:    false,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL).WithToken("test-token")
	progress, err := client.GetProgress(context.Background(), "item-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if progress.CurrentTime != 1800 {
		t.Errorf("expected current time 1800, got %f", progress.CurrentTime)
	}
	if progress.Progress != 0.5 {
		t.Errorf("expected progress 0.5, got %f", progress.Progress)
	}
}

func TestClient_GetProgress_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL).WithToken("test-token")
	progress, err := client.GetProgress(context.Background(), "item-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty progress, not error
	if progress.CurrentTime != 0 {
		t.Errorf("expected current time 0, got %f", progress.CurrentTime)
	}
}

func TestClient_UpdateProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/me/progress/item-123") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var update ProgressUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if update.CurrentTime != 1800 {
			t.Errorf("expected current time 1800, got %f", update.CurrentTime)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL).WithToken("test-token")
	err := client.UpdateProgress(context.Background(), "item-123", ProgressUpdate{
		Duration:    3600,
		CurrentTime: 1800,
		Progress:    0.5,
		IsFinished:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL).WithToken("invalid-token")
	_, err := client.GetLibraries(context.Background())

	if err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestItemsOptions_ToQuery(t *testing.T) {
	tests := []struct {
		name     string
		opts     ItemsOptions
		expected string
	}{
		{
			name:     "empty options",
			opts:     ItemsOptions{},
			expected: "",
		},
		{
			name:     "limit only",
			opts:     ItemsOptions{Limit: 50},
			expected: "limit=50",
		},
		{
			name:     "limit and page",
			opts:     ItemsOptions{Limit: 50, Page: 2},
			expected: "limit=50&page=2",
		},
		{
			name:     "sort ascending",
			opts:     ItemsOptions{Sort: "title"},
			expected: "sort=title",
		},
		{
			name:     "sort descending",
			opts:     ItemsOptions{Sort: "title", Desc: true},
			expected: "desc=1&sort=title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opts.ToQuery()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestLibraryItem_ToSimplified(t *testing.T) {
	item := LibraryItem{
		ID: "item-123",
		Media: BookMedia{
			Metadata: BookMetadata{
				Title:   "Test Book",
				Authors: []Author{{Name: "Test Author"}},
			},
			CoverPath: "/covers/item-123.jpg",
			Duration:  3600,
		},
	}

	simplified := item.ToSimplified("http://localhost:8080")

	if simplified.ID != "item-123" {
		t.Errorf("expected ID 'item-123', got '%s'", simplified.ID)
	}
	if simplified.Title != "Test Book" {
		t.Errorf("expected title 'Test Book', got '%s'", simplified.Title)
	}
	if simplified.Author != "Test Author" {
		t.Errorf("expected author 'Test Author', got '%s'", simplified.Author)
	}
	if simplified.DurationSec != 3600 {
		t.Errorf("expected duration 3600, got %d", simplified.DurationSec)
	}
	if simplified.CoverURL != "http://localhost:8080/api/items/item-123/cover" {
		t.Errorf("unexpected cover URL: %s", simplified.CoverURL)
	}
}
