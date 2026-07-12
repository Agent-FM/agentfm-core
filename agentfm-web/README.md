# agentfm-web

The static landing page for AgentFM. Zero dependencies, no build step: plain HTML, CSS, and vanilla JS with self-hosted fonts (OFL: Space Grotesk, JetBrains Mono).

## Structure

```
index.html        the page
css/style.css     theme (dark blueprint, single amber accent)
js/mesh.js        hero canvas: animated agent-mesh visualization
js/main.js        scroll reveals, quickstart tabs, copy buttons, live GitHub stars
assets/fonts/     self-hosted woff2 (no external font requests)
assets/img/       logo + desktop app screenshots
```

## Develop

```bash
cd agentfm-web
python3 -m http.server 8899
# open http://127.0.0.1:8899
```

## Deploy

It is a plain static folder. Point any static host at it:

- **GitHub Pages** - serve the `agentfm-web/` folder from a branch or move it to a `gh-pages` root.
- **Netlify / Cloudflare Pages** - publish directory `agentfm-web`, no build command.
- **Any nginx / caddy** - `root /path/to/agentfm-web;`

Notes:

- Screenshots are copied from `../assets/screenshots/` when they are refreshed; re-copy after UI changes.
- The GitHub star count is fetched client-side from the GitHub API and falls back to a static label offline.
- Animations honor `prefers-reduced-motion` and pause when the hero is off-screen or the tab is hidden.
