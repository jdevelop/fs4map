# Web SPA KML Viewer

Static single-page map viewer using:
- Leaflet (CDN)
- OpenStreetMap tiles
- KML parsing in browser via `@tmcw/togeojson` (CDN)
- Custom togglable overlay layers from KML folder/type names with legend

## File Layout
- `web/index.html`
- `web/styles.css`
- `web/app.js`
- `web/data/export.kml` (your KML file, not committed)

## Run Locally
Serve the repository root with any static file server, then open:
- `http://localhost:8080/web/`

Examples:
- Python: `python3 -m http.server 8080`
- Node: `npx serve .`

## KML Input
- Fixed file path: `./data/export.kml`
- Place your export file there before opening `web/`

## Layers and Legend
- Right/left panel legend (depending on viewport) lists KML type layers with feature counts
- Toggle each layer on/off with checkbox
- Base map selector (top-right Leaflet control): `OSM Standard`, `OSM HOT`, `Carto Light`
- Type extraction uses KML `Folder` -> `Placemark` membership (for your exported venue-type folders)
- Popups include venue name, type, visit count, and most recent visit timestamp (UTC)
- Header status shows total loaded type count and venue count

## Self-Hosting
Copy the `web/` folder and host it with any static web server. Ensure the KML file is reachable via HTTP(S) from the page location.
