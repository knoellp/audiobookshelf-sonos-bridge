# HTMX Reference

htmx allows you to access AJAX, CSS Transitions, WebSockets, and Server Sent Events directly in HTML using attributes, enabling modern UIs with hypertext simplicity and power.

## Overview

When using htmx, on the server side you typically respond with **HTML**, not JSON. This keeps you firmly within the original web programming model, using Hypertext As The Engine Of Application State (HATEOAS).

### Key Concepts

- **Hypermedia-Driven Applications (HDA)**: A synthesis of MPA and SPA approaches
- **Progressive Enhancement**: Makes htmx applications more accessible
- **Server-Side Rendering**: Respond with HTML fragments

## Installation

### Via CDN (Latest Stable)

```html
<head>
    <script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.8/dist/htmx.min.js" integrity="sha384-/TgkGk7p307TH7EXJDuUlgG3Ce1UVolAOFopFekQkkXihi5u/6OCvVKyz1W+idaz" crossorigin="anonymous"></script>
</head>
```

### Via CDN (Beta)

```html
<script src="https://unpkg.com/htmx.org@2.0.0-beta1/dist/htmx.min.js"></script>
```

### Development Setup

```bash
npm install
npm run test
```

### Start Local Server

```bash
npx serve
```

## Code Examples

### Live Search Input

```html
<input type="text" name="q"
    hx-get="/trigger_delay"
    hx-trigger="keyup delay:500ms changed"
    hx-target="#search-results"
    placeholder="Search...">
<div id="search-results"></div>
```

### Basic hx-select-oob Swap

```html
<div>
   <div id="alert"></div>
    <button hx-get="/info"
            hx-select="#info-details"
            hx-swap="outerHTML"
            hx-select-oob="#alert">
        Get Info!
    </button>
</div>
```

### Demo with Mock Response

```html
<!-- load demo environment -->
<script src="https://demo.htmx.org"></script>

<!-- post to /foo -->
<button hx-post="/foo" hx-target="#result">
    Count Up
</button>
<output id="result"></output>

<!-- respond to /foo with some dynamic content in a template tag -->
<script>
    globalInt = 0;
</script>
<template url="/foo" delay="500">
    ${globalInt++}
</template>
```

### Server Response Example (JavaScript)

```javascript
let requestCount = 0;
this.server.respondWith("GET", "/demo", function(xhr){
  let randomStr = (Math.random() + 1).toString(36).substring(7);
  xhr.respond(200, {}, "Request #" + requestCount++ + " : " + randomStr)
});
```

## Extensions

### Preload Extension

```html
<head>
    <script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.8/dist/htmx.min.js" integrity="sha384-/TgkGk7p307TH7EXJDuUlgG3Ce1UVolAOFopFekQkkXihi5u/6OCvVKyz1W+idaz" crossorigin="anonymous"></script>
    <script src="https://cdn.jsdelivr.net/npm/htmx-ext-preload@2.1.2" integrity="sha384-PRIcY6hH1Y5784C76/Y8SqLyTanY9rnI3B8F3+hKZFNED55hsEqMJyqWhp95lgfk" crossorigin="anonymous"></script>
</head>
<body hx-ext="preload">
...
</body>
```

## HTTP Request Example

```txt
GET /accounts/12345 HTTP/1.1
Host: bank.example.com
```

## Security Considerations

- Basic grasp of web semantics required
- Don't create GET routes that alter backend state
- For standard websites (not hosting other websites)

## Resources

- [HTMX Documentation](https://htmx.org/docs/)
- [Hypermedia Systems Book](https://hypermedia.systems/)
- [Web Security Basics with htmx](https://htmx.org/essays/web-security-basics-with-htmx/)
- [Hypermedia-Driven Applications](https://htmx.org/essays/hypermedia-driven-applications/)
