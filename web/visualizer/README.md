# Fleet visualizer

Solo-track demo UI: a grid of storage nodes that go red when killed and green when repaired, plus per-chunk `n/16` health.

Served by the gateway at `GET /demo/` when `DEMO_MODE=1`. Polls `GET /v1/demo/fleet` every 2s.

Do not build the customer/provider/admin dashboards on the solo track. The marketing waitlist is a separate static tree (`web/landing/`), not served by this gateway.
