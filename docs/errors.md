# Code Verification Errors

## Overview
This document tracks issues found during code verification against reference documentation.

## Error Categories
- **REF**: Reference documentation mismatch
- **BUG**: Logic or implementation bug
- **SEC**: Security issue
- **PERF**: Performance issue
- **STYLE**: Code style issue

## Found Issues

### internal/abs/

#### ABS-001 [BUG] - types.go:14 - Incorrect JSON tag
**File**: `internal/abs/types.go`
**Line**: 14
**Description**: `LoginResponse.Token` had wrong field name. The JSON tag `userDefaultLibraryId` was correct but the field was named `Token` which was confusing.
**Impact**: Low - this struct is not used in current implementation.
**Status**: FIXED - Renamed field from `Token` to `UserDefaultLibraryId` to match JSON key.

#### ABS-002 [BUG] - client.go:442 - Filter/Search conflict
**File**: `internal/abs/client.go`
**Line**: 442
**Description**: In `ItemsOptions.ToQuery()`, if both Filter and Search are set, Search overwrites Filter because both use the "filter" key.
**Impact**: Medium - search functionality may not work correctly when combined with filters.
**Status**: FIXED - Reordered to check Search first (takes precedence), then Filter as else-if. Added clarifying comments.

### internal/sonos/

#### SONOS-001 [BUG] - sonos_test.go:294 - Test using wrong line endings
**File**: `internal/sonos/sonos_test.go`
**Line**: 294
**Description**: TestDiscovery_ExtractLocation test used LF line endings (`\n`) instead of CRLF (`\r\n`), causing the extractLocation regex to fail. SSDP/HTTP requires CRLF.
**Impact**: Test failure only - production code was correct.
**Status**: FIXED - Changed test input to use CRLF line endings.

### internal/cache/
*Verified - No issues found.*

### internal/stream/
*Verified - No issues found.*

### internal/web/

#### WEB-001 [BUG] - player.go:606 - Infinite loop in escapeXML
**File**: `internal/web/player.go`
**Line**: 606-623
**Description**: Custom `replaceAll` function caused infinite loop when escaping XML. When replacing `&` with `&amp;`, the replacement contains `&`, which would be found and replaced again indefinitely.
**Impact**: Critical - Test timeout, potential production hang when escaping XML with `&` characters.
**Status**: FIXED - Replaced custom replaceAll/indexOf with `strings.ReplaceAll` from stdlib.

### internal/store/

#### STORE-001 [BUG] - sessions.go:49-78 - Inconsistent UserID population
**File**: `internal/store/sessions.go`
**Line**: 49-78 (Get function)
**Description**: The `Get()` function did not populate `session.UserID` from `session.ABSUserID`, but `List()` and `ListActive()` do (lines 133 and 172). This inconsistency meant when using auth middleware (which calls `Get()`), `session.UserID` was empty while `session.ABSUserID` had the value.
**Impact**: Medium - affected token generation in player.go:134 which uses `session.UserID`.
**Status**: FIXED - Added `session.UserID = session.ABSUserID` after line 74 in the Get() function.

### web/templates/
*Verified - No issues found. htmx attributes used correctly.*

### cmd/bridge/
*Verified - No issues found.*

---
Last Updated: 2025-12-16
