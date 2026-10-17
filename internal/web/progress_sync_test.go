package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"audiobookshelf-sonos-bridge/internal/abs"
	"audiobookshelf-sonos-bridge/internal/store"
)

type fakeDecrypter struct{}

func (fakeDecrypter) DecryptToken([]byte) (string, error) { return "tok", nil }

func TestSyncSession_BackwardsGuard(t *testing.T) {
	var mu sync.Mutex
	var patched []float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			var u abs.ProgressUpdate
			json.NewDecoder(r.Body).Decode(&u)
			mu.Lock()
			patched = append(patched, u.CurrentTime)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db, err := store.New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sessions := store.NewSessionStore(db)
	if err := sessions.Create(&store.Session{ID: "s1", ABSTokenEnc: []byte("x"), ABSUserID: "u", ABSUsername: "n", CreatedAt: time.Now(), LastUsedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	devices := store.NewDeviceStore(db)
	devices.Upsert(&store.SonosDevice{UUID: "uuid:1", Name: "d", IPAddress: "127.0.0.1", DiscoveredAt: time.Now(), LastSeenAt: time.Now()})
	playbacks := store.NewPlaybackStore(db)
	if err := playbacks.Create(&store.PlaybackSession{
		ID: "p1", SessionID: "s1", ItemID: "item", SonosUUID: "uuid:1", DurationSec: 1000,
		StartedAt: time.Now(), LastPositionUpdate: time.Now(), ABSProgressSyncedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	s := NewProgressSyncer(abs.NewClient(srv.URL), playbacks, sessions, devices, fakeDecrypter{}, "http://bridge", nil)
	ctx := context.Background()
	sync := func(pos int) {
		s.syncSession(ctx, &store.PlaybackSession{ID: "p1", SessionID: "s1", ItemID: "item", DurationSec: 1000, PositionSec: pos})
	}
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(patched) }

	sync(500)
	sync(600)
	if count() != 2 {
		t.Fatalf("expected 2 patches, got %d", count())
	}
	sync(100) // stale/reset position must not reach ABS
	if count() != 2 {
		t.Fatalf("backwards position was synced: %v", patched)
	}
	s.AllowBackwards("p1") // user seeked
	sync(100)
	if count() != 3 || patched[2] != 100 {
		t.Fatalf("backwards sync after seek expected, got %v", patched)
	}
	sync(50) // permission was consumed
	if count() != 3 {
		t.Fatalf("permission should be consumed: %v", patched)
	}
	s.Forget("p1")
	sync(10) // fresh state after Forget
	if count() != 4 {
		t.Fatalf("expected sync after Forget: %v", patched)
	}
}
