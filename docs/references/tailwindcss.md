# Tailwind CSS Reference

Tailwind CSS is a utility-first CSS framework for rapidly building custom user interfaces. It works by scanning all of your HTML files, JavaScript components, and any other templates for class names, generating the corresponding styles and then writing them to a static CSS file.

## Installation

### Using Tailwind CLI

**Install Tailwind CSS:**

```bash
npm install tailwindcss @tailwindcss/cli
```

**Run the CLI build process:**

```bash
npx @tailwindcss/cli -i ./src/input.css -o ./src/output.css --watch
```

The `--watch` flag enables automatic rebuilding when files change.

### With PostCSS

```bash
npm install tailwindcss @tailwindcss/postcss postcss
```

### Framework-Specific Installation

**Laravel:**
```bash
laravel new my-project
cd my-project
npm install tailwindcss @tailwindcss/postcss postcss
```

**Angular:**
```bash
ng new my-project --style css
cd my-project
```

**Astro:**
```bash
npm create astro@latest my-project
cd my-project
```

**Gatsby:**
```bash
gatsby new my-project
cd my-project
```

---

## Core Concepts

### Utility-First Approach

Build components using pre-designed utility classes directly in your HTML:

```html
<div class="mx-auto flex max-w-sm items-center gap-x-4 rounded-xl bg-white p-6 shadow-lg">
  <img class="size-12 shrink-0" src="/img/logo.svg" alt="Logo" />
  <div>
    <div class="text-xl font-medium text-black">ChitChat</div>
    <p class="text-gray-500">You have a new message!</p>
  </div>
</div>
```

### Handling Class Conflicts

When utility classes conflict, the class appearing later in the stylesheet takes precedence:

```html
<div class="grid flex">
  <!-- 'flex' wins because it appears later in the CSS -->
</div>
```

Use conditional rendering to avoid conflicts:

```jsx
export function Example({ gridLayout }) {
  return <div className={gridLayout ? "grid" : "flex"}>{/* ... */}</div>;
}
```

### Force Priority with !important

Use the `!` prefix to generate an `!important` declaration:

```html
<div class="bg-teal-500 bg-red-500!">
  <!-- bg-red-500 takes precedence -->
</div>
```

---

## Responsive Design

Apply utilities conditionally at different breakpoints by prefixing with the breakpoint name:

```html
<div class="grid grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
  <!-- 3 columns on mobile, 4 on medium, 6 on large screens -->
</div>
```

### Default Breakpoints

| Prefix | Min Width | CSS |
|--------|-----------|-----|
| `sm` | 640px | `@media (min-width: 640px)` |
| `md` | 768px | `@media (min-width: 768px)` |
| `lg` | 1024px | `@media (min-width: 1024px)` |
| `xl` | 1280px | `@media (min-width: 1280px)` |
| `2xl` | 1536px | `@media (min-width: 1536px)` |

---

## State Variants

### Hover, Focus, and Other States

Apply utilities conditionally by adding a variant prefix:

```html
<button class="bg-sky-500 hover:bg-sky-700 focus:outline-none focus:ring-2">
  Button
</button>
```

### Common Variants

| Variant | Description |
|---------|-------------|
| `hover:` | On mouse hover |
| `focus:` | On focus |
| `active:` | On active/pressed |
| `disabled:` | When disabled |
| `first:` | First child |
| `last:` | Last child |
| `odd:` | Odd children |
| `even:` | Even children |
| `group-hover:` | When parent with `group` class is hovered |

### Starting Style Variant

Use `starting:` for initial animation states:

```html
<div class="starting:opacity-0 transition-opacity">
  <!-- Starts invisible, transitions to visible -->
</div>
```

---

## Dark Mode

Tailwind includes a `dark` variant for dark mode styling:

```html
<div class="bg-white dark:bg-slate-800">
  <div class="text-black dark:text-white">ChitChat</div>
  <p class="text-gray-500 dark:text-gray-400">You have a new message!</p>
</div>
```

By default, this uses `prefers-color-scheme`. You can also configure manual toggling.

---

## Layout

### Flexbox

```html
<div class="flex items-center justify-between gap-4">
  <div>Item 1</div>
  <div>Item 2</div>
  <div>Item 3</div>
</div>
```

### Grid

**Basic Grid:**
```html
<div class="grid grid-cols-2 gap-4">
  <div>01</div>
  <div>02</div>
  <div>03</div>
  <div>04</div>
</div>
```

**Independent Row/Column Gaps:**
```html
<div class="grid grid-cols-3 gap-x-8 gap-y-4">
  <div>01</div>
  <div>02</div>
  <div>03</div>
  <div>04</div>
  <div>05</div>
  <div>06</div>
</div>
```

**Grid Auto Flow:**
```html
<div class="grid grid-flow-row-dense grid-cols-3 grid-rows-3">
  <div class="col-span-2">01</div>
  <div class="col-span-2">02</div>
  <div>03</div>
</div>
```

**Responsive Grid:**
```html
<div class="grid grid-flow-col md:grid-flow-row">
  <!-- Column flow on mobile, row flow on medium+ -->
</div>
```

### Content Alignment

```html
<!-- Align to end -->
<div class="grid h-56 grid-cols-3 content-end gap-4">...</div>

<!-- Space between -->
<div class="grid h-56 grid-cols-3 content-between gap-4">...</div>

<!-- Space around -->
<div class="grid h-56 grid-cols-3 content-around gap-4">...</div>

<!-- Space evenly -->
<div class="grid h-56 grid-cols-3 content-evenly gap-4">...</div>
```

### Place Items

```html
<div class="grid h-56 grid-cols-3 place-items-stretch gap-4">
  <div>01</div>
  <div>02</div>
  <div>03</div>
</div>
```

---

## Typography

### Text Wrapping

```html
<p class="text-wrap">Text wraps at logical points</p>
<p class="text-nowrap">Text does not wrap</p>
<p class="text-balance">Text balanced across lines</p>
```

### Pseudo-Element Content

```html
<a class="after:content-['_↗']" href="#">
  Link with arrow
</a>
```

---

## Theme Configuration

### Defining Theme Variables

Use the `@theme` directive to customize your design tokens:

```css
@theme {
  --color-regal-blue: #243c5a;
  --font-display: "Satoshi", "sans-serif";
  --breakpoint-3xl: 120rem;
}
```

Then use in HTML:

```html
<div class="bg-regal-blue">...</div>
```

### Custom Easing Functions

```css
@theme {
  --ease-in-expo: cubic-bezier(0.95, 0.05, 0.795, 0.035);
}
```

```html
<div class="ease-in-expo transition-transform">...</div>
```

### Custom Spacing

```css
@theme {
  --spacing: 1px;
}
```

### Disable Default Theme

Replace Tailwind's defaults entirely:

```css
@import "tailwindcss";

@theme {
  --*: initial;
  --spacing: 4px;
  --font-body: Inter, sans-serif;
  --color-lagoon: oklch(0.72 0.11 221.19);
  --color-coral: oklch(0.74 0.17 40.24);
}
```

### Using Prefix

```css
@import "tailwindcss" prefix(tw);

@theme {
  --font-display: "Satoshi", "sans-serif";
  --color-avocado-100: oklch(0.99 0 0);
}
```

---

## Custom Utilities

### Adding Custom Utilities

Use the `@utility` directive:

```css
@utility tab-4 {
  tab-size: 4;
}
```

### Theme-Based Custom Utilities

```css
@theme {
  --tab-size-2: 2;
  --tab-size-4: 4;
  --tab-size-github: 8;
}

@utility tab-* {
  tab-size: --value(--tab-size-*);
}
```

Use in HTML:

```html
<pre class="tab-4">...</pre>
<pre class="tab-github">...</pre>
```

---

## Detecting Classes in Source Files

### Safelist Specific Utilities

Force generation of specific classes:

```css
@import "tailwindcss";
@source inline("underline");
```

---

## Color Palette

Tailwind includes a comprehensive color palette with 11 shades per color:

| Color | Shades |
|-------|--------|
| slate | 50-950 |
| gray | 50-950 |
| zinc | 50-950 |
| neutral | 50-950 |
| stone | 50-950 |
| red | 50-950 |
| orange | 50-950 |
| amber | 50-950 |
| yellow | 50-950 |
| lime | 50-950 |
| green | 50-950 |
| emerald | 50-950 |
| teal | 50-950 |
| cyan | 50-950 |
| sky | 50-950 |
| blue | 50-950 |
| indigo | 50-950 |
| violet | 50-950 |
| purple | 50-950 |
| fuchsia | 50-950 |
| pink | 50-950 |
| rose | 50-950 |

**Usage:**
```html
<div class="bg-blue-500 text-white">Blue background</div>
<div class="bg-emerald-100 text-emerald-800">Emerald theme</div>
```

---

## SVG Utilities

### Fill Colors

```html
<svg class="fill-green-500">...</svg>
<svg class="fill-current">...</svg>
```

### Stroke Colors

```html
<svg class="stroke-blue-500 stroke-2">...</svg>
```

---

## Common Utility Classes

### Spacing

| Class | Property |
|-------|----------|
| `p-4` | padding: 1rem |
| `px-4` | padding-left/right: 1rem |
| `py-4` | padding-top/bottom: 1rem |
| `m-4` | margin: 1rem |
| `mx-auto` | margin-left/right: auto |
| `gap-4` | gap: 1rem |

### Sizing

| Class | Property |
|-------|----------|
| `w-full` | width: 100% |
| `w-screen` | width: 100vw |
| `h-12` | height: 3rem |
| `size-12` | width & height: 3rem |
| `max-w-sm` | max-width: 24rem |
| `min-h-screen` | min-height: 100vh |

### Display

| Class | Property |
|-------|----------|
| `block` | display: block |
| `inline-block` | display: inline-block |
| `flex` | display: flex |
| `grid` | display: grid |
| `hidden` | display: none |

### Position

| Class | Property |
|-------|----------|
| `relative` | position: relative |
| `absolute` | position: absolute |
| `fixed` | position: fixed |
| `sticky` | position: sticky |

### Border Radius

| Class | Property |
|-------|----------|
| `rounded` | border-radius: 0.25rem |
| `rounded-md` | border-radius: 0.375rem |
| `rounded-lg` | border-radius: 0.5rem |
| `rounded-xl` | border-radius: 0.75rem |
| `rounded-full` | border-radius: 9999px |

### Shadows

| Class | Description |
|-------|-------------|
| `shadow-sm` | Small shadow |
| `shadow` | Default shadow |
| `shadow-md` | Medium shadow |
| `shadow-lg` | Large shadow |
| `shadow-xl` | Extra large shadow |
| `shadow-none` | No shadow |

---

## Transitions & Animation

### Transition Properties

```html
<button class="transition-colors duration-300 ease-in-out hover:bg-blue-600">
  Hover me
</button>
```

### Custom Timing Functions

```css
@theme {
  --ease-in-expo: cubic-bezier(0.95, 0.05, 0.795, 0.035);
}
```

```html
<div class="ease-in-expo transition-transform">...</div>
```

---

## Resources

- [Tailwind CSS Documentation](https://tailwindcss.com/docs)
- [Tailwind CSS Playground](https://play.tailwindcss.com/)
- [Tailwind UI Components](https://tailwindui.com/)
- [Heroicons](https://heroicons.com/) - SVG icons by Tailwind team
