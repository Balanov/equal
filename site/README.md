# EQUAL landing page

Static, bilingual (EN/RU), and runtime-dependency-free. Vite is used only for development and the production build. No backend, analytics, hosted badge service, or UI framework is required.

## Develop

```sh
npm install
npm run dev
```

Vite accepts normal CLI arguments, for example `npm run dev -- --host 127.0.0.1 --port 7100`.

## Build

First update the public evidence if benchmark results changed:

```sh
cd ..
python bench/render_site.py
cd site
npm run build
```

The static deliverable is `dist/`. It can be served from any ordinary web server at `/equal/`; asset URLs are relative.

- `src/main.js` — language, interactive local-sandbox demo, benchmark values, clipboard, OS-specific download, and native GitHub API counters.
- `src/i18n.js` — English and Russian interface text.
- `src/results.js` — generated from `bench/results`; do not edit manually.
- `public/results/` — downloadable raw JSON evidence.
- `public/examples/` — a real generated cassette and independently validated OpenAPI document.
- `assets/` — EQUAL logo plus Manrope and JetBrains Mono license texts.

See `../CREDITS.md` for asset and reference credits.
