const grid = document.getElementById("grid");
const chunksEl = document.getElementById("chunks");
const repairEl = document.getElementById("repair");

function statusClass(s) {
  if (s === "online") return "online";
  if (s === "suspect") return "suspect";
  return "offline";
}

function label(n) {
  return n.hostname_label || (n.endpoint || n.id).slice(0, 18);
}

async function tick() {
  try {
    const r = await fetch("/v1/demo/fleet");
    if (!r.ok) throw new Error(r.statusText);
    const data = await r.json();
    const nodes = data.nodes || [];
    grid.innerHTML = nodes
      .map((n) => {
        const st = n.status || "unknown";
        return `<div class="node ${statusClass(st)}">
          <div class="name">${label(n)}</div>
          <div class="meta">${st}<br>${n.region || "—"} · ASN ${n.asn ?? "—"}<br>rep ${(n.reputation ?? 0).toFixed(2)}</div>
        </div>`;
      })
      .join("");
    const chunks = data.chunks || [];
    chunksEl.innerHTML = chunks
      .map((c) => {
        const h = c.healthy | 0;
        const t = c.target || 16;
        let cls = "ok";
        if (h < 13) cls = "bad";
        else if (h < t) cls = "warn";
        return `<span class="chunk ${cls}">${h}/${t}</span>`;
      })
      .join("");
    repairEl.textContent = "repair in progress: " + (data.repair_in_progress || 0);
  } catch (e) {
    repairEl.textContent = "fleet unreachable";
  }
}

tick();
setInterval(tick, 2000);
