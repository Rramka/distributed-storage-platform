# Landing page

Static waitlist page. It is **not** embedded in the gateway binary (the fleet visualizer is). No Go handler, no `waitlist` table, no migration.

## Files

- `index.html` — copy leads with the M4 demo claim
- `styles.css` — same dark tokens as `web/visualizer/`

## Waitlist

The form posts to [Formspree](https://formspree.io/). Replace `YOUR_FORM_ID` in `index.html` with a form ID from a Formspree account before going public. No secrets belong in this repo.

Until that ID is set, submit will 404 at Formspree. That is expected.

## Deploy

Point Cloudflare Pages or GitHub Pages at `web/landing/` (this directory as the site root). Build command: none. Publish directory: `.`

Do not add a `GET /` route on the control-plane gateway. Solo-track web on the gateway remains `GET /demo/` behind `DEMO_MODE=1`.
