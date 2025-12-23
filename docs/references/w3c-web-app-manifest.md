# Web Application Manifest (W3C)

> Source: https://www.w3.org/TR/appmanifest/
> W3C Working Draft, 27 November 2025
> Copyright 2025 World Wide Web Consortium

## Overview

A JSON-based file format that provides developers with a centralized place to put metadata associated with a web application. This metadata includes:
- Application name and icons
- Preferred URL to open when launched
- Default screen orientation
- Display mode (fullscreen, standalone, etc.)
- Navigation scope

## Linking to a Manifest

```html
<!doctype html>
<html>
<head>
    <title>My App</title>
    <!-- Link to manifest -->
    <link rel="manifest" href="manifest.webmanifest">

    <!-- Fallback for legacy browsers -->
    <meta name="application-name" content="My App">
    <link rel="icon" sizes="16x16 32x32 48x48" href="lo_def.ico">
    <link rel="icon" sizes="512x512" href="hi_def.png">
</head>
</html>
```

**File extension:** `.webmanifest` (recommended) or `.json`

## Manifest Members

### Required/Recommended Members

| Member | Type | Description |
|--------|------|-------------|
| `name` | string | Full name of the application |
| `short_name` | string | Short version for limited space |
| `start_url` | string | URL to open when app is launched |
| `display` | string | Preferred display mode |
| `icons` | array | Icons for various contexts |

### All Available Members

| Member | Type | Description |
|--------|------|-------------|
| `background_color` | string | Background color for splash screen |
| `dir` | string | Text direction (`ltr`, `rtl`, `auto`) |
| `display` | string | Display mode |
| `icons` | array | Application icons |
| `id` | string | Unique identifier for the app |
| `lang` | string | Primary language (BCP47 tag) |
| `name` | string | Full application name |
| `orientation` | string | Default screen orientation |
| `scope` | string | Navigation scope URL |
| `short_name` | string | Short name |
| `shortcuts` | array | App shortcuts/jump list |
| `start_url` | string | Start URL |
| `theme_color` | string | Default theme color |

## Typical Manifest Example

```json
{
  "lang": "en",
  "dir": "ltr",
  "name": "Super Racer 3000",
  "short_name": "Racer3K",
  "icons": [
    {
      "src": "icon/lowres.webp",
      "sizes": "64x64",
      "type": "image/webp"
    },
    {
      "src": "icon/lowres.png",
      "sizes": "64x64"
    },
    {
      "src": "icon/hd_hi",
      "sizes": "128x128"
    }
  ],
  "scope": "/",
  "id": "superracer",
  "start_url": "/start.html",
  "display": "fullscreen",
  "orientation": "landscape",
  "theme_color": "aliceblue",
  "background_color": "red"
}
```

## Display Modes

| Value | Description |
|-------|-------------|
| `fullscreen` | All available display area, no browser UI |
| `standalone` | Looks like a native app, no browser UI (URL bar hidden) |
| `minimal-ui` | Minimal browser UI (back/forward, reload) |
| `browser` | Standard browser tab (default) |

The browser uses a fallback chain: if the preferred mode isn't supported, it falls back to the next mode.

**Fallback order:** `fullscreen` → `standalone` → `minimal-ui` → `browser`

## Icons

```json
{
  "icons": [
    {
      "src": "icon/lowres.webp",
      "sizes": "48x48",
      "type": "image/webp"
    },
    {
      "src": "icon/lowres",
      "sizes": "48x48"
    },
    {
      "src": "icon/hd_hi.ico",
      "sizes": "72x72 96x96 128x128 256x256"
    },
    {
      "src": "icon/hd_hi.svg",
      "type": "image/svg+xml"
    }
  ]
}
```

### Icon Properties

| Property | Description |
|----------|-------------|
| `src` | URL to the icon file |
| `sizes` | Space-separated list of sizes (e.g., `48x48`, `72x72 96x96`) |
| `type` | MIME type (e.g., `image/png`, `image/webp`) |
| `purpose` | `any`, `maskable`, or `monochrome` |

### Purpose Values

| Value | Description |
|-------|-------------|
| `any` | Default, can be used anywhere |
| `maskable` | Safe zone for adaptive icons (Android) |
| `monochrome` | Single-color icon for badges |

## Orientation

| Value | Description |
|-------|-------------|
| `any` | Any orientation |
| `natural` | Device's natural orientation |
| `landscape` | Landscape (primary or secondary) |
| `portrait` | Portrait (primary or secondary) |
| `portrait-primary` | Portrait, right-side up |
| `portrait-secondary` | Portrait, upside down |
| `landscape-primary` | Landscape, buttons on right |
| `landscape-secondary` | Landscape, buttons on left |

## Shortcuts (Jump List)

```json
{
  "shortcuts": [
    {
      "name": "Play Later",
      "description": "View saved podcasts",
      "url": "/play-later",
      "icons": [
        {
          "src": "/icons/play-later.svg",
          "type": "image/svg+xml"
        }
      ]
    },
    {
      "name": "Subscriptions",
      "description": "View your subscriptions",
      "url": "/subscriptions?sort=desc"
    }
  ]
}
```

## Scope

The `scope` member defines which URLs are considered part of the application.

```json
{
  "scope": "/app/",
  "start_url": "/app/index.html"
}
```

- `{"scope": "/"}` - Entire origin
- `{"scope": "/racer/"}` - Only `/racer/` path and subdirectories

**Important:** The `start_url` must be within the `scope`.

## Theme and Background Colors

```json
{
  "theme_color": "#4285f4",
  "background_color": "#ffffff"
}
```

- **theme_color**: Affects browser UI elements (address bar, task switcher)
- **background_color**: Splash screen background while app loads

Can also be set via meta tag:
```html
<meta name="theme-color" content="#4285f4">
```

## Complete PWA Example

```json
{
  "name": "Audiobook Streamer",
  "short_name": "Audiobooks",
  "description": "Stream audiobooks to Sonos devices",
  "start_url": "/",
  "scope": "/",
  "id": "audiobook-streamer",
  "display": "standalone",
  "orientation": "portrait",
  "theme_color": "#1a1a2e",
  "background_color": "#1a1a2e",
  "lang": "en",
  "dir": "ltr",
  "icons": [
    {
      "src": "/icons/icon-48.png",
      "sizes": "48x48",
      "type": "image/png"
    },
    {
      "src": "/icons/icon-72.png",
      "sizes": "72x72",
      "type": "image/png"
    },
    {
      "src": "/icons/icon-96.png",
      "sizes": "96x96",
      "type": "image/png"
    },
    {
      "src": "/icons/icon-144.png",
      "sizes": "144x144",
      "type": "image/png"
    },
    {
      "src": "/icons/icon-192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "any"
    },
    {
      "src": "/icons/icon-512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "any"
    },
    {
      "src": "/icons/icon-maskable-192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "maskable"
    },
    {
      "src": "/icons/icon-maskable-512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "maskable"
    }
  ],
  "shortcuts": [
    {
      "name": "Library",
      "url": "/library",
      "icons": [{"src": "/icons/library.png", "sizes": "96x96"}]
    },
    {
      "name": "Now Playing",
      "url": "/player",
      "icons": [{"src": "/icons/player.png", "sizes": "96x96"}]
    }
  ]
}
```

## Browser Support

- Chrome/Edge: Full support
- Firefox: Partial (no `shortcuts`, limited install prompt)
- Safari: Partial (uses Apple-specific meta tags alongside manifest)

See: https://caniuse.com/web-app-manifest

## Related Specifications

- [Image Resource](https://www.w3.org/TR/image-resource/) - Icon format details
- [Screen Orientation API](https://www.w3.org/TR/screen-orientation/) - Orientation control
- [Content Security Policy](https://www.w3.org/TR/CSP3/) - Security considerations
