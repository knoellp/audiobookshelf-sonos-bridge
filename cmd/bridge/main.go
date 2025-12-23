package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/cache"
	"audiobookshelf-sonos-bridge/internal/config"
	"audiobookshelf-sonos-bridge/internal/sonos"
	"audiobookshelf-sonos-bridge/internal/store"
	"audiobookshelf-sonos-bridge/internal/stream"
	"audiobookshelf-sonos-bridge/internal/version"
	"audiobookshelf-sonos-bridge/internal/web"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Setup structured logging
	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Initialize database
	db, err := store.New(cfg.DatabasePath())
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize stores
	sessionStore := store.NewSessionStore(db)
	cacheStore := store.NewCacheStore(db)
	deviceStore := store.NewDeviceStore(db)
	playbackStore := store.NewPlaybackStore(db)

	// Startup cleanup
	slog.Info("performing startup cleanup")

	// Reset in-progress cache entries to pending
	count, err := cacheStore.ResetInProgressToPending()
	if err != nil {
		slog.Warn("failed to reset in-progress cache entries", "error", err)
	} else if count > 0 {
		slog.Info("reset stale cache entries", "count", count)
	}

	// Stop all playback sessions that were left playing from previous run
	playbackCount, err := playbackStore.StopAllPlaying()
	if err != nil {
		slog.Warn("failed to stop stale playback sessions", "error", err)
	} else if playbackCount > 0 {
		slog.Info("stopped stale playback sessions", "count", playbackCount)
	}

	// Delete stale playback sessions (older than 24 hours)
	staleCount, err := playbackStore.DeleteStale(24 * time.Hour)
	if err != nil {
		slog.Warn("failed to delete stale playback sessions", "error", err)
	} else if staleCount > 0 {
		slog.Info("deleted stale playback sessions", "count", staleCount)
	}

	// Clean up old sessions (not used in 7 days)
	sessionCount, err := sessionStore.DeleteOlderThan(time.Now().Add(-7 * 24 * time.Hour))
	if err != nil {
		slog.Warn("failed to cleanup old sessions", "error", err)
	} else if sessionCount > 0 {
		slog.Info("cleaned up old sessions", "count", sessionCount)
	}

	// Initialize Audiobookshelf client
	absClient := abs.NewClient(cfg.ABSURL)

	// Initialize cache subsystem
	cacheIndex := cache.NewIndex(cacheStore, cfg.CacheDir)
	transcoder := cache.NewTranscoder()
	cacheWorker := cache.NewWorker(cacheIndex, transcoder, cfg.TranscodeWorkers)

	// Cleanup temp files on startup
	if err := cacheIndex.CleanupTempFiles(); err != nil {
		slog.Warn("failed to cleanup temp files", "error", err)
	}

	// Initialize stream token generator (1 hour TTL)
	tokenGen := stream.NewTokenGenerator(cfg.SessionSecret, time.Hour)
	streamHandler := stream.NewHandler(tokenGen, cacheIndex, cfg.PublicURL)

	// Initialize auth handler
	authHandler, err := web.NewAuthHandler(absClient, sessionStore, cfg.SessionSecret)
	if err != nil {
		slog.Error("failed to initialize auth handler", "error", err)
		os.Exit(1)
	}

	// Load templates
	templates, err := loadTemplates()
	if err != nil {
		slog.Error("failed to load templates", "error", err)
		os.Exit(1)
	}

	// Initialize Sonos discovery
	discovery := sonos.NewDiscovery(deviceStore)

	// Initialize handlers
	libraryHandler := web.NewLibraryHandler(authHandler, templates, cacheStore)
	sonosHandler := web.NewSonosHandler(discovery, templates)
	playerHandler := web.NewPlayerHandler(
		authHandler,
		cacheIndex,
		cacheWorker,
		tokenGen,
		cfg.PublicURL,
		templates,
		deviceStore,
		playbackStore,
		cfg.MapABSPathToLocal,
	)

	// Initialize progress syncer
	progressSyncer := web.NewProgressSyncer(absClient, playbackStore, sessionStore, deviceStore, authHandler)

	// Initialize sleep timer worker
	sleepTimerWorker := web.NewSleepTimerWorker(playbackStore, sessionStore, deviceStore, absClient, authHandler)

	// Initialize cache warmup job
	warmupJob := cache.NewWarmupJob(
		cacheIndex,
		cacheWorker,
		absClient,
		sessionStore,
		authHandler,
		cache.DefaultWarmupConfig,
	)

	// Setup HTTP router
	mux := http.NewServeMux()

	// Health endpoint (public)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Version endpoint (public)
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(version.Full())
	})

	// Static files (public)
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Login page (public)
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		errorParam := r.URL.Query().Get("error")
		data := map[string]interface{}{
			"Title":      "Login",
			"ShowHeader": false,
			"Error":      errorParam,
		}
		renderTemplate(w, templates, "login.html", data)
	})

	// Root redirect - serves a small page that checks localStorage for saved library
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		// Serve a minimal page that redirects based on localStorage
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html><head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Redirecting...</title>
<style>body{background:#121212;color:#fff;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;font-family:sans-serif}</style>
</head><body>
<p>Redirecting...</p>
<script>
const savedLibrary = localStorage.getItem('selectedLibraryID');
if (savedLibrary) {
    window.location.replace('/libraries/' + savedLibrary + '/items');
} else {
    window.location.replace('/library');
}
</script>
<noscript><meta http-equiv="refresh" content="0;url=/library"></noscript>
</body></html>`))
	})

	// Auth endpoints
	mux.HandleFunc("POST /auth/login", authHandler.HandleLogin)
	mux.HandleFunc("POST /auth/logout", authHandler.HandleLogout)

	// Streaming endpoint (token-protected, not session-protected)
	mux.HandleFunc("GET /stream/", streamHandler.HandleStream)
	mux.HandleFunc("HEAD /stream/", streamHandler.HandleStream)

	// Helper to wrap handlers with auth middleware
	auth := func(h http.HandlerFunc) http.Handler {
		return authHandler.RequireAuth(http.HandlerFunc(h))
	}

	// Library routes (protected)
	mux.Handle("GET /library", auth(libraryHandler.HandleLibraries))
	mux.Handle("GET /libraries", auth(libraryHandler.HandleLibraries))
	mux.Handle("GET /library/recent", auth(libraryHandler.HandleRecent))
	mux.Handle("GET /library/series", auth(libraryHandler.HandleSeries))
	mux.Handle("GET /library/series/{id}", auth(libraryHandler.HandleSeriesDetail))
	mux.Handle("GET /library/authors", auth(libraryHandler.HandleAuthors))
	mux.Handle("GET /library/genres", auth(libraryHandler.HandleGenres))
	mux.Handle("GET /libraries/{id}/items", auth(libraryHandler.HandleLibraryItems))
	mux.Handle("GET /libraries/{id}/filterdata", auth(libraryHandler.HandleFilterData))
	mux.Handle("GET /cover/{id}", auth(libraryHandler.HandleCover))
	mux.Handle("GET /item/{id}", auth(libraryHandler.HandleItem))

	// Sonos routes (protected)
	mux.Handle("GET /sonos/devices", auth(sonosHandler.HandleGetDevices))
	mux.Handle("POST /sonos/refresh", auth(sonosHandler.HandleRefreshDevices))
	mux.Handle("POST /sonos/quick-refresh", auth(sonosHandler.HandleQuickRefresh))
	mux.Handle("GET /sonos/poll-groups", auth(sonosHandler.HandlePollGroups))

	// Player routes (protected)
	mux.Handle("POST /play", auth(playerHandler.HandlePlay))
	mux.Handle("GET /player/{id}", auth(playerHandler.HandlePlayer))
	mux.Handle("GET /status", auth(playerHandler.HandleStatus))
	mux.Handle("GET /cache/status/{id}", auth(playerHandler.HandleCacheStatus))

	// Transport control routes (protected)
	mux.Handle("POST /transport/pause", auth(playerHandler.HandlePause))
	mux.Handle("POST /transport/resume", auth(playerHandler.HandleResume))
	mux.Handle("POST /transport/seek", auth(playerHandler.HandleSeek))
	mux.Handle("POST /transport/stop", auth(playerHandler.HandleStop))
	mux.Handle("POST /transport/volume", auth(playerHandler.HandleSetVolume))
	mux.Handle("POST /transport/mute", auth(playerHandler.HandleToggleMute))

	// Group volume control routes (protected)
	mux.Handle("GET /sonos/group-info", auth(playerHandler.HandleGetGroupInfo))
	mux.Handle("GET /volume/group", auth(playerHandler.HandleGetGroupVolume))
	mux.Handle("POST /volume/group", auth(playerHandler.HandleSetGroupVolume))
	mux.Handle("POST /volume/group/adjust", auth(playerHandler.HandleAdjustGroupVolume))

	// Individual member volume routes (protected)
	mux.Handle("GET /volume/members", auth(playerHandler.HandleGetMemberVolumes))
	mux.Handle("POST /volume/member", auth(playerHandler.HandleSetMemberVolume))

	// Group management routes (protected)
	mux.Handle("GET /sonos/all-players", auth(playerHandler.HandleGetAllPlayers))
	mux.Handle("POST /sonos/group/join", auth(playerHandler.HandleJoinGroup))
	mux.Handle("POST /sonos/group/leave", auth(playerHandler.HandleLeaveGroup))

	// Sleep timer routes (protected)
	mux.Handle("POST /sleep-timer", auth(playerHandler.HandleSetSleepTimer))
	mux.Handle("DELETE /sleep-timer", auth(playerHandler.HandleDeleteSleepTimer))
	mux.Handle("GET /sleep-timer", auth(playerHandler.HandleGetSleepTimer))

	// Wrap with logging middleware
	handler_http := web.LoggingMiddleware(logger)(mux)

	// Create server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler_http,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 300 * time.Second, // Longer for streaming
		IdleTimeout:  60 * time.Second,
	}

	// Create root context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background services
	cacheWorker.Start(ctx)
	progressSyncer.Start(ctx)
	sleepTimerWorker.Start(ctx)
	warmupJob.Start(ctx)

	// Log path mappings
	slog.Info("path mappings configured",
		"media_dir", cfg.MediaDir,
		"abs_media_prefix", cfg.ABSMediaPrefix,
		"additional_mappings", len(cfg.PathMappings),
	)
	for i, m := range cfg.PathMappings {
		slog.Info("path mapping", "index", i, "abs_prefix", m.ABSPrefix, "local_path", m.LocalPath)
	}

	// Start server in goroutine
	go func() {
		slog.Info("starting server",
			"version", version.Short(),
			"port", cfg.Port,
			"public_url", cfg.PublicURL,
			"cache_dir", cfg.CacheDir,
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")

	// Stop background services
	warmupJob.Stop()
	sleepTimerWorker.Stop()
	progressSyncer.Stop()
	cacheWorker.Stop()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

func loadTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"formatDuration": formatDuration,
		"mult":           func(a, b float64) float64 { return a * b },
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
	}

	// Parse base layout and partials only (shared templates)
	templates, err := template.New("").Funcs(funcMap).ParseGlob("web/templates/layout.html")
	if err != nil {
		return nil, err
	}

	// Parse partials
	templates, err = templates.ParseGlob("web/templates/partials/*.html")
	if err != nil {
		return nil, err
	}

	return templates, nil
}

// getFuncMap returns the template function map
func getFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatDuration": formatDuration,
		"mult":           func(a, b float64) float64 { return a * b },
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
	}
}

func renderTemplate(w http.ResponseWriter, _ *template.Template, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Parse templates fresh for each request to avoid Clone issues
	tmpl, err := template.New("").Funcs(getFuncMap()).ParseGlob("web/templates/layout.html")
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
		slog.Error("template execution error", "error", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func formatDuration(seconds int) string {
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
}
