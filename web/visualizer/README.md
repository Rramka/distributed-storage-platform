# Fleet visualizer

Solo-track demo UI: a grid of storage nodes that go red when killed and green when repaired, plus per-chunk `n/16` health.

Served by the gateway at `GET /demo/` when `DEMO_MODE=1`. Polls `GET /v1/demo/fleet` every 2s.

Do not build the customer/provider/admin dashboards on the solo track. This page is the only web UI until M4 is recorded.
