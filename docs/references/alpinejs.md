# Alpine.js Reference

Alpine.js is a rugged, minimal JavaScript framework for composing behavior directly in your markup, offering a lightweight alternative for adding interactivity to the modern web.

## Overview

Alpine.js provides a declarative and reactive approach to building JavaScript-powered interfaces. It's designed to be used directly in your HTML markup.

## Installation

### Via CDN

```html
<!DOCTYPE html>
<html>
<head>
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
</head>
<body>
    <h1 x-data="{ message: 'I love Alpine' }" x-text="message"></h1>
</body>
</html>
```

### Via NPM

```bash
npm install alpinejs
```

```javascript
import Alpine from 'alpinejs'

window.Alpine = Alpine

Alpine.start()
```

> **Note**: Extensions should be registered before calling `Alpine.start()`. Calling `Alpine.start()` multiple times can lead to issues.

### CSP Build (Content Security Policy)

```bash
npm install @alpinejs/csp
```

```javascript
import Alpine from '@alpinejs/csp'

window.Alpine = Alpine

Alpine.start()
```

**CSP Example HTML:**
```html
<html>
    <head>
        <meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'nonce-a23gbfz9e'">
        <script defer nonce="a23gbfz9e" src="https://cdn.jsdelivr.net/npm/@alpinejs/csp@3.x.x/dist/cdn.min.js"></script>
    </head>
    <body>
        <div x-data="{ count: 0, message: 'Hello' }">
            <button x-on:click="count++">Increment</button>
            <button x-on:click="count = 0">Reset</button>

            <span x-text="count"></span>
            <span x-text="message + ' World'"></span>
            <span x-show="count > 5">Count is greater than 5!</span>
        </div>
    </body>
</html>
```

## Code Examples

### Counter Component

```html
<div x-data="{ count: 0 }">
    <button x-on:click="count++">Increment</button>
    <span x-text="count"></span>
</div>
```

### Event Listening with x-on

```html
<button x-on:click="count++">Increment</button>
```

> **Tip**: You will often see `@` instead of `x-on:`. This is a shorter, friendlier syntax that many prefer.

### Search Input with Filtering

```html
<div
    x-data="{
        search: '',
        items: ['foo', 'bar', 'baz'],
        get filteredItems() {
            return this.items.filter(
                i => i.startsWith(this.search)
            )
        }
    }"
>
    <input x-model="search" placeholder="Search...">

    <ul>
        <template x-for="item in filteredItems" :key="item">
            <li x-text="item"></li>
        </template>
    </ul>
</div>
```

### Looping Elements with x-for

```html
<div x-data="{ statuses: ['open', 'closed', 'archived'] }">
    <template x-for="status in statuses">
        <div x-text="status"></div>
    </template>
</div>
```

### Using $el Magic Property

```html
<div x-data>
    <button @click="$el.textContent = 'Hello World!'">
        Replace me with "Hello World!"
    </button>
</div>
```

## Plugins

### Focus Plugin

```bash
npm install @alpinejs/focus
```

```javascript
import Alpine from 'alpinejs'
import focus from '@alpinejs/focus'

Alpine.plugin(focus)
```

## Development

### Build from Source

```bash
npm install
npm run build
```

## Key Directives

| Directive | Description |
|-----------|-------------|
| `x-data` | Declares a new Alpine component and its data |
| `x-text` | Sets the text content of an element |
| `x-html` | Sets the inner HTML of an element |
| `x-show` | Toggles element visibility |
| `x-if` | Conditionally renders an element |
| `x-for` | Loops over an array |
| `x-on` / `@` | Listens for DOM events |
| `x-model` | Two-way data binding for inputs |
| `x-bind` / `:` | Dynamically binds attributes |
| `x-init` | Runs code when component initializes |
| `x-effect` | Re-runs code when dependencies change |

## Magic Properties

| Property | Description |
|----------|-------------|
| `$el` | Reference to the current DOM element |
| `$refs` | Access to elements marked with `x-ref` |
| `$store` | Access to global Alpine stores |
| `$watch` | Watch a piece of data for changes |
| `$dispatch` | Dispatch custom events |
| `$nextTick` | Execute code after Alpine updates the DOM |
| `$root` | Reference to the root element of the component |
| `$data` | Access to the component's data object |

## Upgrading from V2

### Deprecated APIs

> **Note**: Prefer `Alpine.data()` to global Alpine function data providers. You need to define `Alpine.data()` extensions BEFORE you call `Alpine.start()`.

## Resources

- [Alpine.js Documentation](https://alpinejs.dev/)
- [Alpine.js Router](https://github.com/shaunlee/alpinejs-router)
- [Alpine Ajax](https://github.com/imacrayon/alpine-ajax)
