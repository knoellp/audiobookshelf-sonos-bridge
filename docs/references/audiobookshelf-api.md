# Audiobookshelf API Reference

Audiobookshelf is a self-hosted audiobook and podcast server. This document covers the REST API for interacting with the server.

## Authentication

Audiobookshelf uses a user's API token as a Bearer token for requests. For GET requests, the API token can optionally be passed as a query string.

### Getting Your API Token

1. Log into the Audiobookshelf web app as an admin
2. Go to config -> users page
3. Click on your account to find your API token

You can also get the API token programmatically using the Login endpoint.

### Request Header Format

```
Authorization: Bearer exJhbGciOiJI6IkpXVCJ9.eyJ1c2Vyi5NDEyODc4fQ.ZraBFohS4Tg39NszY
```

### Query String Format (GET requests only)

```
https://abs.example.com/api/items/li_asdfalwkerioa?token=YOUR_API_TOKEN
```

---

## Server Endpoints

### Login

**POST** `/login`

Logs in a client to the server, returning information about the user and server.

**Request Body:**
```json
{
  "username": "root",
  "password": "*****"
}
```

**Response:**
```json
{
  "user": {
    "id": "root",
    "username": "root",
    "type": "root",
    "token": "exJhbGciOiJI6IkpXVCJ9...",
    "mediaProgress": [...],
    "permissions": {
      "download": true,
      "update": true,
      "delete": true,
      "upload": true,
      "accessAllLibraries": true,
      "accessAllTags": true,
      "accessExplicitContent": true
    }
  },
  "userDefaultLibraryId": "lib_c1u6t4p45c35rf0nzd",
  "serverSettings": {...},
  "Source": "docker"
}
```

| Status | Meaning | Description |
|--------|---------|-------------|
| 200 | OK | Success |
| 401 | Unauthorized | Invalid username or password |

### Logout

**POST** `/logout`

Logs out a client from the server.

**Optional Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `socketId` | String | The ID of the connected socket |

### Check Server Status

**GET** `/status`

Reports the server's initialization status.

**Response:**
```json
{
  "isInit": true,
  "language": "en-us"
}
```

### Ping Server

**GET** `/ping`

Simple check to see if the server is operating.

**Response:**
```json
{
  "success": true
}
```

### Healthcheck

**GET** `/healthcheck`

Simple check to see if the server can respond.

---

## Libraries

### Create a Library

**POST** `/api/libraries`

**Request Body:**
```json
{
  "name": "Podcasts",
  "folders": [{"fullPath": "/podcasts"}],
  "icon": "podcast",
  "mediaType": "podcast",
  "provider": "itunes"
}
```

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `name` | String | **Required** | The name of the library |
| `folders` | Array | **Required** | The folders of the library. Only specify `fullPath` |
| `icon` | String | `database` | The icon of the library |
| `mediaType` | String | `book` | Must be `book` or `podcast` |
| `provider` | String | `google` | Preferred metadata provider |
| `settings` | Object | See below | Library settings |

**Library Settings:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `coverAspectRatio` | Integer | `1` | Square covers: `0` (false) or `1` (true) |
| `disableWatcher` | Boolean | `false` | Disable folder watcher |
| `skipMatchingMediaWithAsin` | Boolean | `false` | Skip matching books with ASIN |
| `skipMatchingMediaWithIsbn` | Boolean | `false` | Skip matching books with ISBN |
| `autoScanCronExpression` | String/null | `null` | Cron expression for auto-scan |

### Get All Libraries

**GET** `/api/libraries`

Returns all libraries accessible to the user.

### Get a Library

**GET** `/api/libraries/<ID>`

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `include` | String | Comma-separated list. Option: `filterdata` |

### Update a Library

**PATCH** `/api/libraries/<ID>`

### Delete a Library

**DELETE** `/api/libraries/<ID>`

### Get Library Items

**GET** `/api/libraries/<ID>/items`

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | Integer | Limit results per page (0 = no limit) |
| `page` | Integer | Page number (0 indexed) |
| `sort` | String | Sort field (e.g., `media.metadata.title`) |
| `desc` | Binary | Reverse sort order (`0` or `1`) |
| `filter` | String | Filter string (URL encoded) |
| `minified` | Binary | Request minified objects |
| `collapseseries` | Binary | Collapse series to single entry |
| `include` | String | Include options (e.g., `rssfeed`) |

### Search a Library

**GET** `/api/libraries/<ID>/search`

### Get Library Series

**GET** `/api/libraries/<ID>/series`

### Get Library Collections

**GET** `/api/libraries/<ID>/collections`

### Get Library Authors

**GET** `/api/libraries/<ID>/authors`

### Get Library Stats

**GET** `/api/libraries/<ID>/stats`

---

## Library Items

### Get a Library Item

**GET** `/api/items/<ID>`

### Delete a Library Item

**DELETE** `/api/items/<ID>`

### Update Library Item Media

**PATCH** `/api/items/<ID>/media`

### Get Library Item Cover

**GET** `/api/items/<ID>/cover`

### Upload Library Item Cover

**POST** `/api/items/<ID>/cover`

### Play a Library Item

**POST** `/api/items/<ID>/play`

Starts playback of a library item or podcast episode.

### Update Audio Tracks

**PATCH** `/api/items/<ID>/tracks`

### Update Chapters

**POST** `/api/items/<ID>/chapters`

### Batch Operations

- **DELETE** `/api/items/all` - Delete all library items
- **POST** `/api/items/batch/delete` - Batch delete items
- **POST** `/api/items/batch/update` - Batch update items
- **POST** `/api/items/batch/get` - Batch get items

---

## Users

### Create a User

**POST** `/api/users`

### Get All Users

**GET** `/api/users`

### Get a User

**GET** `/api/users/<ID>`

### Update a User

**PATCH** `/api/users/<ID>`

### Delete a User

**DELETE** `/api/users/<ID>`

### Get User Listening Sessions

**GET** `/api/users/<ID>/listening-sessions`

### Get User Listening Stats

**GET** `/api/users/<ID>/listening-stats`

---

## Me (Current User)

### Get Your User

**GET** `/api/me`

### Get Your Listening Sessions

**GET** `/api/me/listening-sessions`

### Get Your Listening Stats

**GET** `/api/me/listening-stats`

### Get Media Progress

**GET** `/api/me/progress/<libraryItemId>`

### Create/Update Media Progress

**PATCH** `/api/me/progress/<libraryItemId>`

Updates playback progress for a library item.

### Remove Media Progress

**DELETE** `/api/me/progress/<libraryItemId>`

### Bookmarks

- **POST** `/api/me/item/<ID>/bookmark` - Create bookmark
- **PATCH** `/api/me/item/<ID>/bookmark` - Update bookmark
- **DELETE** `/api/me/item/<ID>/bookmark/<time>` - Remove bookmark

### Change Password

**PATCH** `/api/me/password`

---

## Sessions

### Get All Sessions

**GET** `/api/sessions`

### Get an Open Session

**GET** `/api/session/<ID>`

### Sync an Open Session

**POST** `/api/session/<ID>/sync`

### Close an Open Session

**POST** `/api/session/<ID>/close`

### Delete a Session

**DELETE** `/api/sessions/<ID>`

---

## Collections

### Create a Collection

**POST** `/api/collections`

### Get All Collections

**GET** `/api/collections`

### Get a Collection

**GET** `/api/collections/<ID>`

### Update a Collection

**PATCH** `/api/collections/<ID>`

### Delete a Collection

**DELETE** `/api/collections/<ID>`

### Add/Remove Books

- **POST** `/api/collections/<ID>/book` - Add book
- **DELETE** `/api/collections/<ID>/book/<bookId>` - Remove book
- **POST** `/api/collections/<ID>/batch/add` - Batch add
- **POST** `/api/collections/<ID>/batch/remove` - Batch remove

---

## Playlists

### Create a Playlist

**POST** `/api/playlists`

### Get All User Playlists

**GET** `/api/playlists`

### Get/Update/Delete a Playlist

- **GET** `/api/playlists/<ID>`
- **PATCH** `/api/playlists/<ID>`
- **DELETE** `/api/playlists/<ID>`

### Add/Remove Items

- **POST** `/api/playlists/<ID>/item` - Add item
- **DELETE** `/api/playlists/<ID>/item/<itemId>` - Remove item

---

## Authors

### Get an Author

**GET** `/api/authors/<ID>`

### Update an Author

**PATCH** `/api/authors/<ID>`

### Match an Author

**POST** `/api/authors/<ID>/match`

### Get Author Image

**GET** `/api/authors/<ID>/image`

---

## Series

### Get a Series

**GET** `/api/series/<ID>`

### Update a Series

**PATCH** `/api/series/<ID>`

---

## Podcasts

### Create a Podcast

**POST** `/api/podcasts`

### Get Podcast Feed

**POST** `/api/podcasts/feed`

### Check for New Episodes

**GET** `/api/podcasts/<ID>/checknew`

### Download Episodes

**POST** `/api/podcasts/<ID>/download-episodes`

### Get/Update/Delete Episode

- **GET** `/api/podcasts/<ID>/episode/<episodeId>`
- **PATCH** `/api/podcasts/<ID>/episode/<episodeId>`
- **DELETE** `/api/podcasts/<ID>/episode/<episodeId>`

---

## Search

### Search for Covers

**GET** `/api/search/covers`

### Search for Books

**GET** `/api/search/books`

### Search for Podcasts

**GET** `/api/search/podcast`

### Search for Authors

**GET** `/api/search/authors`

---

## Miscellaneous

### Upload Files

**POST** `/api/upload`

### Update Server Settings

**PATCH** `/api/settings`

### Get Authorized User and Server Info

**GET** `/api/authorize`

### Tags

- **GET** `/api/tags` - Get all tags
- **POST** `/api/tags/rename` - Rename a tag
- **DELETE** `/api/tags/<tag>` - Delete a tag

### Genres

- **GET** `/api/genres` - Get all genres
- **POST** `/api/genres/rename` - Rename a genre
- **DELETE** `/api/genres/<genre>` - Delete a genre

---

## WebSocket Events

Audiobookshelf uses Socket.IO for real-time communication.

### Client Events

Events sent from the client to the server.

### Server Events

Events broadcast from the server to clients.

### Categories

- **User Events**: User login/logout, updates
- **Stream Events**: Playback start/stop/progress
- **Library Events**: Library create/update/delete
- **Library Item Events**: Item create/update/delete
- **Author Events**: Author updates
- **Series Events**: Series updates
- **Collection Events**: Collection changes
- **Playlist Events**: Playlist changes
- **Backup Events**: Backup status

---

## Metadata Providers

### Books

- Google Books
- Audible
- iTunes
- Open Library
- Fantlab

### Podcasts

- iTunes

---

## Filtering

Filters are URL-encoded strings in the format: `filterType.base64EncodedValue`

**Filter Types:**

- `authors` - Filter by author ID
- `genres` - Filter by genre
- `tags` - Filter by tag
- `series` - Filter by series ID
- `narrators` - Filter by narrator
- `languages` - Filter by language
- `progress` - Filter by progress (finished, in-progress, not-started)
- `missing` - Filter by missing metadata
- `issues` - Filter items with issues
- `feed-open` - Filter items with open RSS feeds

---

## Library Icons

Available icons for libraries:

- `database`
- `audiobookshelf`
- `books-1`
- `books-2`
- `book-1`
- `microphone-1`
- `microphone-3`
- `radio`
- `podcast`
- `rss`
- `headphones`
- `music`
- `file-picture`
- `rocket`
- `power`
- `star`
- `heart`

---

## Key Schemas

### Library Item

```json
{
  "id": "li_8gch9ve09orgn4fdz8",
  "ino": "649641337522215266",
  "libraryId": "main",
  "folderId": "audiobooks",
  "path": "/audiobooks/Author/Book",
  "relPath": "Author/Book",
  "isFile": false,
  "mtimeMs": 1650621074299,
  "ctimeMs": 1650621074299,
  "birthtimeMs": 0,
  "addedAt": 1650621073750,
  "updatedAt": 1650621110769,
  "isMissing": false,
  "isInvalid": false,
  "mediaType": "book",
  "media": {...},
  "numFiles": 3,
  "size": 96335771
}
```

### Media Progress

```json
{
  "id": "li_bufnnmp4y5o2gbbxfm-ep_lh6ko39pumnrma3dhv",
  "libraryItemId": "li_bufnnmp4y5o2gbbxfm",
  "episodeId": "ep_lh6ko39pumnrma3dhv",
  "duration": 1454.18449,
  "progress": 0.434998929881311,
  "currentTime": 632.568697,
  "isFinished": false,
  "hideFromContinueListening": false,
  "lastUpdate": 1668586015691,
  "startedAt": 1668120083771,
  "finishedAt": null
}
```

### User Permissions

```json
{
  "download": true,
  "update": true,
  "delete": true,
  "upload": true,
  "accessAllLibraries": true,
  "accessAllTags": true,
  "accessExplicitContent": true
}
```

---

## Resources

- [Audiobookshelf GitHub](https://github.com/advplyr/audiobookshelf)
- [API Documentation Source](https://github.com/advplyr/audiobookshelf-slate)
- [Official Website](https://www.audiobookshelf.org/)
