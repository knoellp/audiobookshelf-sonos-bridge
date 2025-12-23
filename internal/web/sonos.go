package web

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"audiobookshelf-sonos-bridge/internal/sonos"
)

// SonosHandler handles Sonos-related HTTP requests.
type SonosHandler struct {
	discovery *sonos.Discovery
	templates *template.Template
}

// NewSonosHandler creates a new Sonos handler.
func NewSonosHandler(discovery *sonos.Discovery, templates *template.Template) *SonosHandler {
	return &SonosHandler{
		discovery: discovery,
		templates: templates,
	}
}

// HandleGetDevices returns the list of Sonos devices as HTML for htmx.
// If no devices are in the store, it triggers automatic discovery.
// Also refreshes group info from Sonos to ensure group sizes are current.
func (h *SonosHandler) HandleGetDevices(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	devices, err := h.discovery.GetDevices()
	if err != nil {
		http.Error(w, "Failed to get devices", http.StatusInternalServerError)
		return
	}

	// If no devices found, trigger discovery automatically
	if len(devices) == 0 {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		discoveredDevices, err := h.discovery.Discover(ctx, 5*time.Second)
		if err == nil && len(discoveredDevices) > 0 {
			// Refresh devices from store after discovery
			devices, _ = h.discovery.GetDevices()
		}
	} else {
		// Always refresh group info to ensure current group sizes
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := h.discovery.RefreshGroupInfo(ctx); err != nil {
			slog.Warn("HandleGetDevices: RefreshGroupInfo failed", "error", err)
		}
		// Re-fetch devices after refresh to get updated group sizes
		devices, _ = h.discovery.GetDevices()
	}

	// Convert to template format
	deviceList := make([]DeviceResponse, len(devices))
	for i, d := range devices {
		groupSize := d.GroupSize
		if groupSize == 0 {
			groupSize = 1 // Default to standalone if not set
		}
		deviceList[i] = DeviceResponse{
			UUID:        d.UUID,
			Name:        d.Name,
			IPAddress:   d.IPAddress,
			Model:       d.Model,
			IsReachable: d.IsReachable,
			GroupSize:   groupSize,
		}
	}

	// Sort devices alphabetically by name
	sort.Slice(deviceList, func(i, j int) bool {
		return deviceList[i].Name < deviceList[j].Name
	})

	// Render HTML template for htmx
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"Devices": deviceList,
	}
	if err := h.templates.ExecuteTemplate(w, "sonos-device-list", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// HandleQuickRefresh quickly updates group info from Sonos and returns updated HTML.
// This is much faster than HandleRefreshDevices as it doesn't do SSDP discovery.
func (h *SonosHandler) HandleQuickRefresh(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// Quick refresh group info from ZoneGroupTopology
	if err := h.discovery.RefreshGroupInfo(ctx); err != nil {
		slog.Warn("HandleQuickRefresh: RefreshGroupInfo failed", "error", err)
	} else {
		slog.Debug("HandleQuickRefresh: RefreshGroupInfo completed successfully")
	}

	// Get updated devices from store
	devices, err := h.discovery.GetDevices()
	if err != nil {
		http.Error(w, "Failed to get devices", http.StatusInternalServerError)
		return
	}

	// Convert to template format
	deviceList := make([]DeviceResponse, len(devices))
	for i, d := range devices {
		groupSize := d.GroupSize
		if groupSize == 0 {
			groupSize = 1
		}
		deviceList[i] = DeviceResponse{
			UUID:        d.UUID,
			Name:        d.Name,
			IPAddress:   d.IPAddress,
			Model:       d.Model,
			IsReachable: d.IsReachable,
			GroupSize:   groupSize,
		}
	}

	// Sort devices alphabetically by name
	sort.Slice(deviceList, func(i, j int) bool {
		return deviceList[i].Name < deviceList[j].Name
	})

	// Render HTML template for htmx
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"Devices": deviceList,
	}
	if err := h.templates.ExecuteTemplate(w, "sonos-device-list", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// HandleRefreshDevices triggers a new device discovery and returns updated HTML.
func (h *SonosHandler) HandleRefreshDevices(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Perform discovery
	devices, err := h.discovery.Discover(ctx, 5*time.Second)
	if err != nil {
		http.Error(w, "Discovery failed", http.StatusInternalServerError)
		return
	}

	// Convert to template format
	deviceList := make([]DeviceResponse, len(devices))
	for i, d := range devices {
		groupSize := d.GroupSize
		if groupSize == 0 {
			groupSize = 1 // Default to standalone if not set
		}
		deviceList[i] = DeviceResponse{
			UUID:        d.UUID,
			Name:        d.Name,
			IPAddress:   d.IPAddress,
			Model:       d.Model,
			IsReachable: d.IsReachable,
			GroupSize:   groupSize,
		}
	}

	// Sort devices alphabetically by name
	sort.Slice(deviceList, func(i, j int) bool {
		return deviceList[i].Name < deviceList[j].Name
	})

	// Render HTML template for htmx
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"Devices": deviceList,
	}
	if err := h.templates.ExecuteTemplate(w, "sonos-device-list", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// DeviceResponse is the API response for a Sonos device.
type DeviceResponse struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	IPAddress   string `json:"ip_address"`
	Model       string `json:"model"`
	IsReachable bool   `json:"is_reachable"`
	GroupSize   int    `json:"group_size"` // Number of players in this group (>1 for grouped coordinator)
}
