# HAM: HTML As Modules

HAM is a lightweight static site framework that lets you build websites using modular, reusable HTML components. Instead of duplicating headers, footers, and navigation across every page, you define them once and compose them together. HAM compiles everything into plain HTML files ready for deployment anywhere.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
  - [Layouts](#layouts)
  - [Pages](#pages)
  - [Partials](#partials)
- [How Compilation Works](#how-compilation-works)
- [Template Variables](#template-variables)
- [TypeScript Support](#typescript-support)
- [Commands](#commands)
- [Project Structure](#project-structure)
- [Reverse Proxy](#reverse-proxy)

## Installation

**Requires**: [Go 1.21+](https://go.dev/doc/install)

```bash
go install github.com/fobilow/ham/cmd/ham@latest
```

To install a specific version, replace `@latest` with the version number (e.g. `@v1.0.0`).

**Build from source:**

```bash
git clone https://github.com/fobilow/ham.git
cd ham
make install
```

Verify the installation:

```bash
ham version
```

## Quick Start

Create a new project, start the dev server, and open it in your browser:

```bash
ham init mysite
cd mysite
npm install
ham serve -w ./src -p 4120
```

Open [http://localhost:4120](http://localhost:4120) to see your site. The dev server watches for file changes and automatically reloads the browser.

When you're ready to deploy, build the production files:

```bash
ham build -w ./src -o ./public
```

Your compiled site is now in the `public/` directory, ready to host on any static file server.

## Core Concepts

HAM has three building blocks: **Layouts**, **Pages**, and **Partials**. They fit together like this:

```
Layout (page skeleton)
  └── Page (content for a specific route)
        └── Partial (reusable HTML fragment)
```

### File Naming Conventions

HAM uses file extensions to distinguish between component types:

| Component | Extension | Example |
|---|---|---|
| Layout | `.lhtml` | `default.lhtml`, `blog.lhtml` |
| Page | `.html` | `index.html`, `about.html` |
| Partial | `.phtml` | `header.phtml`, `footer.phtml`, `card.phtml` |

This is important — HAM uses these extensions to identify what each file is. Using the wrong extension will cause compilation errors.

### Layouts

A layout defines the outer shell of a web page — the `<html>`, `<head>`, and `<body>` structure. Use special `<embed>` and `<link>` tags to mark where dynamic content should be injected.

Layout files use the `.lhtml` extension.

```html
<!-- src/default.lhtml -->
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>My Site</title>
    <link type="ham/layout-css"/>
</head>
<body>
    <embed type="ham/page"/>
    <embed type="ham/layout-js"/>
</body>
</html>
```

| Placeholder | What it does |
|---|---|
| `<link type="ham/layout-css"/>` | Replaced with `<link>` tags for all CSS files listed in the page config |
| `<embed type="ham/page"/>` | Replaced with the compiled page content |
| `<embed type="ham/layout-js"/>` | Replaced with `<script>` tags for all JS files listed in the page config |

### Pages

A page provides the content for a specific route. Every page must reference a layout via the `data-ham-page-config` attribute. This is where you declare which layout, CSS, and JavaScript files the page needs.

Page files use the `.html` extension.

```html
<!-- src/index.html -->
<div class="page"
     data-ham-page-config='{
       "layout": "default.lhtml",
       "css": ["index.css"],
       "js": ["app.js"]
     }'
>
    <embed type="ham/partial" src="header.phtml"/>
    <h1>Welcome to my site</h1>
    <p>This is the home page.</p>
</div>
```

**Page config options:**

| Key | Type | Description |
|---|---|---|
| `layout` | string | Path to the layout file (relative to the page) |
| `css` | string[] | CSS files to include |
| `js` | string[] | JavaScript files to include |
| `js-mod` | string[] | TypeScript/ES module files (rendered as `<script type="module">`) |
| `id` | string | Sets the `id` attribute on the `<body>` tag |

### Partials

Partials are reusable HTML fragments — things like headers, footers, cards, or any component you want to share across pages. Include them with `<embed type="ham/partial">`.

Partial files use the `.phtml` extension.

```html
<!-- src/header.phtml -->
<header>
    <nav>
        <a href="/">Home</a>
        <a href="/about.html">About</a>
    </nav>
</header>
```

Include it in a page or another partial:

```html
<embed type="ham/partial" src="header.phtml"/>
```

Partials can be nested — a partial can include other partials, and HAM will resolve them all recursively.

## How Compilation Works

When you run `ham build`, HAM takes your source files and assembles them into complete HTML pages:

1. **Parse** — Reads each `.html` page file and extracts the `data-ham-page-config`
2. **Embed partials** — Recursively replaces all `<embed type="ham/partial">` tags with the contents of the referenced `.phtml` files
3. **Inject into layout** — Inserts the compiled page content into the layout at `<embed type="ham/page"/>`
4. **Attach resources** — Replaces `<link type="ham/layout-css"/>` and `<embed type="ham/layout-js"/>` with the actual `<link>` and `<script>` tags
5. **Write output** — Saves the final HTML files to the output directory, preserving the folder structure

**Before (source files):**

```
src/default.lhtml     ← layout shell
src/header.phtml      ← reusable nav partial
src/index.html        ← home page
src/index.css         ← home page styles
```

**After (`ham build`):**

```html
<!-- public/index.html -->
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>My Site</title>
    <link rel="stylesheet" href="/assets/css/index.css"/>
</head>
<body>
    <div class="page">
        <header>
            <nav>
                <a href="/">Home</a>
                <a href="/about.html">About</a>
            </nav>
        </header>
        <h1>Welcome to my site</h1>
        <p>This is the home page.</p>
    </div>
    <script src="/assets/js/app.js"></script>
</body>
</html>
```

## Template Variables

You can pass variables into partials using `data-ham-replace`. This lets you reuse the same partial with different content.

**Define a partial with placeholders** (use double underscores):

```html
<!-- src/card.phtml -->
<div class="card">
    <h2>__title__</h2>
    <p>__description__</p>
</div>
```

**Use it with different values:**

```html
<embed type="ham/partial" src="card.phtml"
       data-ham-replace="title:Getting Started,description:Learn the basics of HAM"/>

<embed type="ham/partial" src="card.phtml"
       data-ham-replace="title:Advanced Usage,description:Take your site to the next level"/>
```

## TypeScript Support

HAM projects come pre-configured with [Rollup](https://rollupjs.org/) for TypeScript bundling. Use the `js-mod` config key instead of `js` to include TypeScript files:

```html
<div class="page"
     data-ham-page-config='{
       "layout": "default.lhtml",
       "css": ["index.css"],
       "js-mod": ["index.ts"]
     }'
>
```

These are rendered as `<script type="module">` tags in the final output.

## Commands

| Command | Description |
|---|---|
| `ham init <sitename>` | Create a new project with boilerplate files |
| `ham build -w <dir> -o <dir>` | Compile the site for production |
| `ham serve -w <dir> -p <port>` | Start a dev server with hot reload (default port: 4120) |
| `ham proxy` | Run a reverse proxy for API + static files |
| `ham version` | Print the current HAM version |
| `ham help` | Show usage information |

### `ham build`

| Flag | Default | Description |
|---|---|---|
| `-w` | `./` | Working directory (where your `src/` folder is) |
| `-o` | `./public` | Output directory for compiled files |

### `ham serve`

| Flag | Default | Description |
|---|---|---|
| `-w` | `./` | Working directory |
| `-p` | `4120` | Port number |

The dev server uses Server-Sent Events to trigger browser reloads when source files change. No browser extension required.

## Project Structure

After running `ham init mysite`, you get:

```
mysite/
├── ham.json              # Project configuration
├── package.json          # npm dependencies (Rollup, TypeScript)
├── tsconfig.json         # TypeScript configuration
├── rollup.config.js      # JS/TS bundler configuration
├── .gitignore
└── src/
    ├── default.lhtml     # Default layout
    ├── index.html        # Home page
    ├── index.css         # Home page styles
    └── index.ts          # Home page script
```

You're free to organize `src/` however you like. A typical multi-page site might look like:

```
src/
├── default.lhtml         # Shared layout
├── header.phtml          # Shared nav partial
├── footer.phtml          # Shared footer partial
├── index.html            # Home page
├── index.css
├── about/
│   ├── index.html        # About page
│   └── index.css
└── blog/
    ├── index.html        # Blog listing page
    └── post.html         # Individual blog post page
```

## Reverse Proxy

`ham proxy` runs a production-ready server that serves your compiled site and proxies API requests to a backend. Configure it with environment variables:

| Variable | Default | Description |
|---|---|---|
| `WEB_ROOT` | `./public` | Directory containing compiled HTML |
| `PROXY_PORT` | `8082` | Port to listen on |
| `API_ENDPOINT` | `http://localhost:8080` | Backend API URL |
| `API_PROXY_PREFIX` | `/api/` | URL prefix for API routes |

Any request matching the API prefix is forwarded to your backend; everything else is served as a static file.

Pages can require authentication by adding `data-ham-proxy="requires-authentication"` to the page element. The proxy checks for a valid session token (set via the `X-HAM-PROXY-TOKEN` header) before serving the page.
