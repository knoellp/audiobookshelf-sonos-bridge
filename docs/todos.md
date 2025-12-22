# Implementation Tasks

## Issue 1: Player Switching on Resume - COMPLETED

**Problem**: When user changes the Sonos player in the header, resume still plays on the original player. User must go back to library and restart playback.

**Solution**: Pass the currently selected Sonos UUID from the frontend when resuming, and switch players if it changed.

### Tasks:
- [x] **T1.1** Modify `resume()` in `player.html` to send `sonos_uuid` parameter
- [x] **T1.2** Modify `HandleResume` in `player.go` to accept optional `sonos_uuid` parameter
- [x] **T1.3** If `sonos_uuid` differs from stored session, stop on old device and start on new device
- [x] **T1.4** Update `PlaybackSession.SonosUUID` when player changes

---

## Issue 2: Volume Flickering - COMPLETED

**Problem**: When changing volume, the slider briefly jumps back to old value before updating to new value. This happens because status polling (every 1s) reads the device's volume and updates the UI while the new volume command is still processing.

**Solution**: Add a "user is adjusting" flag that prevents status updates from overwriting the volume slider.

### Tasks:
- [x] **T2.1** Add `volumeAdjusting` flag in `transport.html`
- [x] **T2.2** Set flag `true` on `oninput` (when user starts dragging)
- [x] **T2.3** Set flag `false` after debounced `setVolume()` completes (with small delay)
- [x] **T2.4** Modify `updateVolumeUI()` to skip update when `volumeAdjusting` is true

---

## Issue 3: Progress Bar with Draggable Seek Head - COMPLETED

**Problem**: Current progress bar only supports click-to-seek. User wants:
1. Draggable seek head (thumb)
2. Chapter markers for audiobooks with chapters
3. Chapter list to click and jump to specific chapters

**Solution**: Replace simple progress bar with interactive slider that shows chapters.

### Tasks:

#### 3A: Draggable Progress Bar
- [x] **T3.1** Replace `<div class="progress-bar">` with `<input type="range">` slider
- [x] **T3.2** Style the slider to look like a progress bar with a draggable thumb
- [x] **T3.3** Add `oninput` for live preview (update time display while dragging)
- [x] **T3.4** Add `onchange` to seek when user releases
- [x] **T3.5** Add `seekAdjusting` flag to prevent status updates overwriting position while dragging

#### 3B: Chapter Support - Backend
- [x] **T3.6** Modify `HandlePlayer` to pass `Item.Media.Chapters` to template
- [x] **T3.7** Ensure chapters are included in the template data

#### 3C: Chapter Support - Frontend
- [x] **T3.8** Add chapter markers on progress bar (visual indicators)
- [x] **T3.9** Add chapter list/dropdown below progress bar
- [x] **T3.10** Add click handler on chapter to seek to chapter start
- [x] **T3.11** Show current chapter name based on position

---

## Testing - COMPLETED

- [x] **T4.1** Test player switching: Start on Büro, pause, switch to Küche, resume - PASSED
- [x] **T4.2** Test volume: Changed from 69% to 40%, no flickering observed - PASSED
- [x] **T4.3** Test seek: Chapter markers work, progress bar is draggable - PASSED
- [x] **T4.4** Test chapters: Clicked Kapitel 10, seeked to 41:53 correctly - PASSED

---

## Summary

Issues 1-3 implemented and tested successfully on 2025-12-21.

---

## Issue 5: Chapter Navigation Improvements - COMPLETED

**Problem**:
1. Current chapter is not displayed on page load
2. No buttons to skip to previous/next chapter

**Solution**:
1. Initialize chapter display when page loads with current position
2. Add prev/next chapter buttons that only appear when chapters exist

### Tasks:
- [x] **T5.1** Initialize chapter display on page load (call updateCurrentChapterDisplay with initial position)
- [x] **T5.2** Add previous chapter button (skip to start of current or previous chapter)
- [x] **T5.3** Add next chapter button (skip to start of next chapter)
- [x] **T5.4** Only show chapter buttons when audiobook has chapters
- [x] **T5.5** Update getCurrentChapterIndex() function to get chapter index
- [x] **T5.6** Test chapter navigation - PASSED

### Test Results (2025-12-21):
- Kapitelanzeige zeigt korrektes Kapitel ("Kapitel 5" bei Position 17:46)
- "Vorheriges Kapitel" Button springt zum Kapitelanfang oder vorherigen Kapitel
- "Nächstes Kapitel" Button springt zum nächsten Kapitel
- Buttons nur sichtbar bei Hörbüchern mit Kapiteln

---

## Issue 6: Wrong Book Playback & Chapter Names - COMPLETED

### Problem 1: Wrong book played
When user opens player page for Book A while there's an existing paused session for Book B, clicking Play would resume Book B instead of starting Book A.

**Root Cause**: The `resume()` function only checked if `playbackActive` was true, but didn't verify if the playback session was for the currently displayed item.

**Solution**:
- Track `playbackItemId` from status updates
- Compare with current page's `data-item-id` before resuming
- If item IDs don't match, call `startPlayback()` instead of `resume()`
- Only update UI when playback session matches current item

### Tasks:
- [x] **T6.1** Add `playbackItemId` tracking in player.html
- [x] **T6.2** Modify `resume()` to check if item_id matches current page
- [x] **T6.3** Modify `updateStatus()` to only update UI for matching item

### Problem 2: Chapter names not displayed
Chapters were shown as "Kapitel: [title]" which was redundant when title was just "Kapitel X".

**Solution**:
- Show chapter number as "X / Y" (e.g., "5 / 15")
- Show chapter title only if it's not a generic "Kapitel X" pattern
- Improved chapter display layout with number above, title below

### Tasks:
- [x] **T6.4** Update chapter display HTML structure
- [x] **T6.5** Update `updateCurrentChapterDisplay()` to detect generic titles
- [x] **T6.6** Update CSS for new chapter display layout

---

## Issue 11: Progress Not Synced on Pause - COMPLETED

### Problem
Wenn der Benutzer pausiert und zur Bibliothek zurückkehrt, wird beim erneuten Abspielen nicht an der gleichen Stelle fortgesetzt.

### Ursache
`HandlePause` (player.go) hat:
1. Keine aktuelle Position von Sonos geholt
2. Nicht zu Audiobookshelf synchronisiert

Der Hintergrund-Syncer synchronisiert nur alle 30 Sekunden. Bei Pause gingen bis zu 30 Sekunden verloren.

### Lösung
`HandlePause` erweitert um dieselbe Logik wie `HandleStop`:
1. Position von Sonos holen (VOR dem Pausieren für Genauigkeit)
2. Lokale DB aktualisieren (mit korrekter Segment-Berechnung)
3. Sofort zu ABS synchronisieren

### Geänderte Dateien
- `internal/web/player.go` - HandlePause erweitert (Zeilen 336-420)

### Zusätzliche Analyse
- `HandlePlay` holt bereits frische Daten von ABS (Zeile 199-204) ✅
- ABS bleibt die Single Source of Truth ✅

---

## Issue 7: Player Switch & Stop Issues - IN PROGRESS

### Problem 1: Player switch doesn't start playback
When switching from one Sonos player to another (e.g., Küche → Kamin), playback doesn't start on the new device.

**Root Causes**:
1. Stream token might be expired (1 hour TTL) - switch reuses old token
2. Errors are logged to console but not shown to user
3. If switch fails, no user feedback

### Problem 2: Stop doesn't stop playback
After failed player switch, clicking Stop goes to library but audio keeps playing.

**Root Cause**: Stop only stops the device in playback.SonosUUID. If switch failed mid-way, audio might still be on old device but SonosUUID wasn't updated (or vice versa).

### Tasks:
- [x] **T7.1** Add error feedback in transportAction - show alert on failure
- [x] **T7.2** Regenerate stream token when switching players if token is old
- [x] **T7.3** Add more logging for player switch debugging
- [x] **T7.4** Fix UI state when playback item doesn't match - reset to play button
- [x] **T7.5** Improve stop robustness - stop on both playback device AND currently selected device
- [ ] **T7.6** Test player switch scenario

### Implementation Details:

**T7.1 - Error Feedback**: Added user-visible alerts in `transportAction()` for resume (404/other), stop, and general failures.

**T7.2 - Token Regeneration**: `HandleResume` now generates a fresh stream token when switching players, avoiding expired token issues.

**T7.3 - Debug Logging**: Added detailed slog.Debug/Info statements throughout player switch flow in `HandleResume`.

**T7.4 - UI State Fix**: `updateStatus()` now only updates UI when `data.item_id === currentItemId`, otherwise shows play button.

**T7.5 - Stop Robustness**:
- Frontend `stop()` now sends `current_sonos_uuid` parameter
- Backend `HandleStop` stops both: (1) playback session's device, (2) currently selected device if different
- Handles failed player switches where audio continues on unexpected device

---

## Issue 8: ZP90 M4A File Size Limit - IN PROGRESS

### Problem
Der Sonos Connect (ZP90) hat ein RAM-Limit von ~128MB für M4A-Dateien. Größere Dateien werden akzeptiert (SetAVTransportURI OK, Play OK), aber die Wiedergabe startet nicht (TRANSITIONING → STOPPED).

### Testergebnisse (21.12.2025)
| Dauer | Größe | ZP90 Status |
|-------|-------|-------------|
| 2h | 55MB | ✅ PLAYING |
| 4h | 109MB | ✅ PLAYING |
| 4.5h | 122MB | ✅ PLAYING |
| 4h45m | 129MB | ❌ STOPPED |
| 5h | 136MB | ❌ STOPPED |
| 6h | 163MB | ❌ STOPPED |

**Grenze: ~125-128MB** (vermutlich 128MB RAM-Limit)

### Lösung: 2-Stunden-Segmente

Große Audiodateien werden in 2-Stunden-Segmente aufgeteilt:
- ~55MB pro Segment bei 63kbps AAC
- ~60% Puffer unter der ZP90-Grenze
- Sicher auch bei 128kbps (~110MB)
- Einfache Positionsberechnung: `segment_index * 7200 + local_position`

### Cache-Struktur

Aktuell:
```
/cache/{item_id}/audio.m4a
```

Neu:
```
/cache/{item_id}/
  segment_000.m4a  (0:00:00 - 2:00:00)
  segment_001.m4a  (2:00:00 - 4:00:00)
  segment_002.m4a  (4:00:00 - 6:00:00)
  ...
  metadata.json    (Segmentinfo)
```

### Implementation Tasks

#### Phase 1: Cache-Segmentierung
- [ ] **T8.1** `CacheEntry` erweitern: `SegmentCount`, `SegmentDurationSec` Felder
- [ ] **T8.2** `RemuxSegmented()` in transcoder.go: Datei in 2h-Segmente aufteilen
- [ ] **T8.3** `metadata.json` schreiben mit Segment-Info
- [ ] **T8.4** `GetEntry()` anpassen für Segment-Metadaten

#### Phase 2: Streaming
- [ ] **T8.5** `StreamHandler` anpassen: `/stream/{token}/segment_{index}.m4a`
- [ ] **T8.6** Token enthält Basis-Item-Info, Segment wird aus URL extrahiert

#### Phase 3: Nahtloser Playback
- [ ] **T8.7** Status-Polling: Segment-Ende erkennen (Position nahe 7200s)
- [ ] **T8.8** Automatischer Wechsel zum nächsten Segment
- [ ] **T8.9** Alternative: `SetNextAVTransportURI` für gapless playback prüfen

#### Phase 4: Seek & Navigation
- [ ] **T8.10** Seek über Segmentgrenzen: korrektes Segment laden + Position
- [ ] **T8.11** Kapitelnavigation: Segment aus Kapitelposition berechnen

#### Phase 5: Tests
- [ ] **T8.12** Test: Wiedergabe startet auf ZP90 für jedes Segment
- [ ] **T8.13** Test: Nahtloser Übergang zwischen Segmenten
- [ ] **T8.14** Test: Seek zu Kapitel in anderem Segment
- [ ] **T8.15** Test: Play:1 funktioniert weiterhin

### Positionsberechnung
```go
const SegmentDurationSec = 7200 // 2 Stunden

// Globale Position → Segment + lokale Position
func GlobalToSegment(globalPosSec int) (segmentIndex int, localPosSec int) {
    segmentIndex = globalPosSec / SegmentDurationSec
    localPosSec = globalPosSec % SegmentDurationSec
    return
}

// Segment + lokale Position → globale Position
func SegmentToGlobal(segmentIndex, localPosSec int) int {
    return segmentIndex * SegmentDurationSec + localPosSec
}
```

### Offene Fragen
1. **SetNextAVTransportURI**: Sonos unterstützt Queue für gapless playback - ZP90 testen
2. **Cache-Invalidierung**: Einzelne Segmente neu erstellen bei Beschädigung?
3. **Rückwärts-Kompatibilität**: Alte Cache-Einträge (einzelne Dateien) migrieren?

---

## Issue 9: Sonos Stereo Pair Filtering - COMPLETED

### Problem
Stereo pairs show as duplicate entries in the Sonos device picker. This happens because SSDP discovery finds all physical speakers individually, including stereo pair slaves.

### Solution
Use Sonos ZoneGroupTopology service to identify and filter invisible players (stereo pair slaves).

### Technical Details

#### ZoneGroupState XML Structure
```xml
<ZoneGroupState>
  <ZoneGroups>
    <ZoneGroup Coordinator="RINCON_XXX">
      <ZoneGroupMember UUID="RINCON_XXX" ZoneName="Living Room" Invisible="0"/>
      <ZoneGroupMember UUID="RINCON_YYY" ZoneName="Living Room" Invisible="1"/>
    </ZoneGroup>
  </ZoneGroups>
</ZoneGroupState>
```

#### Key Attributes
- `Invisible="1"`: Stereo pair slave (should be filtered)
- `Invisible="0"` or missing: Visible player (should be shown)
- `Coordinator`: UUID of group coordinator

### Implementation Tasks

- [x] **T9.1** Add ZoneGroupTopology types to `internal/sonos/types.go`
- [x] **T9.2** Implement GetZoneGroupState in `internal/sonos/zonegroupstate.go`
- [x] **T9.3** Parse ZoneGroupState XML to extract visibility info
- [x] **T9.4** Modify discovery.go to filter invisible players
- [x] **T9.5** Test with Playwright in headed mode

### Test Results (2025-12-21)
- Discovery found 10 devices total
- 1 invisible player (stereo pair slave) filtered: `RINCON_949F3E048C9601400` (Schlafzimmer)
- 9 visible devices shown in the picker
- Schlafzimmer stereo pair now shows as single entry

### Files to Modify/Create
1. `internal/sonos/types.go` - Add XML structs
2. `internal/sonos/zonegroupstate.go` - New file for ZoneGroupTopology
3. `internal/sonos/discovery.go` - Integrate filtering

---

## Issue 10: Item-Detail zeigt falsche Dauer - COMPLETED

### Problem
Auf der Item-Detail-Seite wird "< 1 min" angezeigt, obwohl das Buch (z.B. "Herzfluch" von Andreas Gruber) über 16 Stunden lang ist.

### Ursache
In `library.go:343` wird `item.Media.Duration` direkt verwendet ohne Fallback auf AudioFiles.

### Lösung
1. Duration-Berechnung mit Fallback auf AudioFiles-Summe wenn Media.Duration = 0
2. Neue Felder `ProgressPct` und `RemainingMin` in DetailedItem struct
3. Template zeigt bei Fortschritt: "16h 48m · 45% · 9h 2m verbleibend"

### Tasks

- [x] **T10.1** Analysieren: Ursache für falsche Duration gefunden
- [x] **T10.2** Fix: Duration-Berechnung in `library.go` korrigieren (Fallback auf AudioFiles)
- [x] **T10.3** Enhancement: Verbleibende Zeit berechnen und anzeigen
- [x] **T10.4** Template anpassen: Neue Zeitanzeige implementieren
- [x] **T10.5** Test: Manuell prüfen mit "Herzfluch" - zeigt jetzt "16 hr 48 min" korrekt

### Test Results (2025-12-21)
- "Herzfluch" zeigt jetzt korrekt "16 hr 48 min" statt "< 1 min"
- Verbleibende Zeit wird nur bei Fortschritt > 0% angezeigt
- Cache-Status weiterhin korrekt ("Ready to play" / "Not cached")

### Geänderte Dateien
- `internal/web/library.go` - Duration-Berechnung und DetailedItem mit RemainingMin/ProgressPct
- `web/templates/item.html` - Anzeige der Zeitinformationen mit verbleibender Zeit
