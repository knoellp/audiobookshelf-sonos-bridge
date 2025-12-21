package abs

import "time"

// LoginRequest represents the login request body.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the login response from Audiobookshelf.
type LoginResponse struct {
	User                 User   `json:"user"`
	UserDefaultLibraryId string `json:"userDefaultLibraryId"`
}

// User represents a user from Audiobookshelf.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Type     string `json:"type"`
	Token    string `json:"token"`
}

// Library represents a library from Audiobookshelf.
type Library struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Folders     []Folder `json:"folders"`
	MediaType   string `json:"mediaType"`
	Provider    string `json:"provider"`
	CreatedAt   int64  `json:"createdAt"`
	LastUpdate  int64  `json:"lastUpdate"`
}

// Folder represents a folder in a library.
type Folder struct {
	ID       string `json:"id"`
	FullPath string `json:"fullPath"`
}

// LibrariesResponse represents the response from /api/libraries.
type LibrariesResponse struct {
	Libraries []Library `json:"libraries"`
}

// LibraryItem represents an audiobook or podcast item.
type LibraryItem struct {
	ID            string         `json:"id"`
	INo           string         `json:"ino"`
	LibraryID     string         `json:"libraryId"`
	FolderID      string         `json:"folderId"`
	Path          string         `json:"path"`
	RelPath       string         `json:"relPath"`
	IsFile        bool           `json:"isFile"`
	Mtimems       int64          `json:"mtimeMs"`
	CtimeMs       int64          `json:"ctimeMs"`
	BirthtimeMs   int64          `json:"birthtimeMs"`
	AddedAt       int64          `json:"addedAt"`
	UpdatedAt     int64          `json:"updatedAt"`
	IsMissing     bool           `json:"isMissing"`
	IsInvalid     bool           `json:"isInvalid"`
	MediaType     string         `json:"mediaType"`
	Media         BookMedia      `json:"media"`
	NumFiles      int            `json:"numFiles"`
	Size          int64          `json:"size"`
}

// BookMedia contains the book-specific media information.
type BookMedia struct {
	Metadata    BookMetadata `json:"metadata"`
	CoverPath   string       `json:"coverPath"`
	Tags        []string     `json:"tags"`
	AudioFiles  []AudioFile  `json:"audioFiles"`
	Chapters    []Chapter    `json:"chapters"`
	Duration    float64      `json:"duration"`
	Size        int64        `json:"size"`
	EbookFile   *EbookFile   `json:"ebookFile"`
}

// BookMetadata contains metadata about a book.
type BookMetadata struct {
	Title           string   `json:"title"`
	Subtitle        string   `json:"subtitle"`
	Authors         []Author `json:"authors"`
	Narrators       []string `json:"narrators"`
	Series          []Series `json:"series"`
	Genres          []string `json:"genres"`
	PublishedYear   string   `json:"publishedYear"`
	PublishedDate   string   `json:"publishedDate"`
	Publisher       string   `json:"publisher"`
	Description     string   `json:"description"`
	ISBN            string   `json:"isbn"`
	ASIN            string   `json:"asin"`
	Language        string   `json:"language"`
	Explicit        bool     `json:"explicit"`
}

// Author represents an author.
type Author struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Series represents a series with sequence number.
type Series struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Sequence string `json:"sequence"`
}

// AudioFile represents an audio file in a book.
type AudioFile struct {
	Index      int     `json:"index"`
	Ino        string  `json:"ino"`
	Metadata   FileMetadata `json:"metadata"`
	AddedAt    int64   `json:"addedAt"`
	UpdatedAt  int64   `json:"updatedAt"`
	TrackNumFromMeta   *int    `json:"trackNumFromMeta"`
	DiscNumFromMeta    *int    `json:"discNumFromMeta"`
	TrackNumFromFilename *int  `json:"trackNumFromFilename"`
	DiscNumFromFilename  *int  `json:"discNumFromFilename"`
	ManuallyVerified bool    `json:"manuallyVerified"`
	Invalid          bool    `json:"invalid"`
	Exclude          bool    `json:"exclude"`
	Error            string  `json:"error"`
	Format           string  `json:"format"`
	Duration         float64 `json:"duration"`
	BitRate          int     `json:"bitRate"`
	Language         string  `json:"language"`
	Codec            string  `json:"codec"`
	TimeBase         string  `json:"timeBase"`
	Channels         int     `json:"channels"`
	ChannelLayout    string  `json:"channelLayout"`
	MimeType         string  `json:"mimeType"`
}

// FileMetadata contains file system metadata.
type FileMetadata struct {
	Filename    string `json:"filename"`
	Ext         string `json:"ext"`
	Path        string `json:"path"`
	RelPath     string `json:"relPath"`
	Size        int64  `json:"size"`
	Mtimems     int64  `json:"mtimeMs"`
	Ctimems     int64  `json:"ctimeMs"`
	Birthtimems int64  `json:"birthtimeMs"`
}

// Chapter represents a chapter in a book.
type Chapter struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Title string  `json:"title"`
}

// EbookFile represents an ebook file.
type EbookFile struct {
	Ino         string       `json:"ino"`
	Metadata    FileMetadata `json:"metadata"`
	EbookFormat string       `json:"ebookFormat"`
	AddedAt     int64        `json:"addedAt"`
	UpdatedAt   int64        `json:"updatedAt"`
}

// ItemsResponse represents the response from /api/libraries/{id}/items.
type ItemsResponse struct {
	Results   []LibraryItem `json:"results"`
	Total     int           `json:"total"`
	Limit     int           `json:"limit"`
	Page      int           `json:"page"`
	SortBy    string        `json:"sortBy"`
	SortDesc  bool          `json:"sortDesc"`
	FilterBy  string        `json:"filterBy"`
	MediaType string        `json:"mediaType"`
	Minified  bool          `json:"minified"`
	Collapseseries bool     `json:"collapseseries"`
	Include   string        `json:"include"`
}

// FilterData represents the filter data for a library.
type FilterData struct {
	Authors    []FilterAuthor    `json:"authors"`
	Series     []FilterSeries    `json:"series"`
	Genres     []string          `json:"genres"`
	Tags       []string          `json:"tags"`
	Narrators  []string          `json:"narrators"`
	Languages  []string          `json:"languages"`
	Publishers []string          `json:"publishers"`
}

// FilterAuthor represents an author in filter data.
type FilterAuthor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FilterSeries represents a series in filter data.
type FilterSeries struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Progress represents playback progress for an item.
type Progress struct {
	ID                  string  `json:"id"`
	LibraryItemID       string  `json:"libraryItemId"`
	EpisodeID           string  `json:"episodeId"`
	Duration            float64 `json:"duration"`
	Progress            float64 `json:"progress"` // 0.0 to 1.0
	CurrentTime         float64 `json:"currentTime"`
	IsFinished          bool    `json:"isFinished"`
	HideFromContinueListening bool `json:"hideFromContinueListening"`
	LastUpdate          int64   `json:"lastUpdate"`
	StartedAt           int64   `json:"startedAt"`
	FinishedAt          *int64  `json:"finishedAt"`
}

// ProgressUpdate represents the request body for updating progress.
type ProgressUpdate struct {
	Duration        float64 `json:"duration"`
	CurrentTime     float64 `json:"currentTime"`
	Progress        float64 `json:"progress"`
	IsFinished      bool    `json:"isFinished"`
}

// SimplifiedItem is a simplified view of a library item for UI purposes.
type SimplifiedItem struct {
	ID          string
	Title       string
	Author      string
	CoverURL    string
	DurationSec int
	Progress    float64
}

// ToSimplified converts a LibraryItem to SimplifiedItem.
func (item *LibraryItem) ToSimplified(baseURL string) SimplifiedItem {
	author := ""
	if len(item.Media.Metadata.Authors) > 0 {
		author = item.Media.Metadata.Authors[0].Name
	}

	coverURL := ""
	if item.Media.CoverPath != "" {
		coverURL = baseURL + "/api/items/" + item.ID + "/cover"
	}

	return SimplifiedItem{
		ID:          item.ID,
		Title:       item.Media.Metadata.Title,
		Author:      author,
		CoverURL:    coverURL,
		DurationSec: int(item.Media.Duration),
	}
}

// GetPrimaryAudioFile returns the primary audio file for an item.
func (item *LibraryItem) GetPrimaryAudioFile() *AudioFile {
	if len(item.Media.AudioFiles) == 0 {
		return nil
	}
	return &item.Media.AudioFiles[0]
}

// GetTotalDuration returns the total duration in seconds.
func (item *LibraryItem) GetTotalDuration() time.Duration {
	return time.Duration(item.Media.Duration * float64(time.Second))
}
