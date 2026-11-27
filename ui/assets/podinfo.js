const POLL_MS = 2000;
const TIMEOUT_MS = 1500;
const WINDOW = 30;
const PING_MIN_MS = 350;
const PING_FAILED_MS = 1600;
const MASKED = "[REDACTED]";
const LONG = 40;

const prefix = document.body.dataset.prefix;
const $ = (selector) => document.querySelector(selector);
const root = document.documentElement;
const themeColor = $('meta[name="theme-color"]');
const message = $(".message");
const revisionShort = $(".revision");
const stage = $(".stage");
const version = $(".version");
const hostname = $(".hostname");
const up = $(".up");
const pulse = $(".up .pulse");
const lostLine = $(".lost");
const logo = $(".logo");
const tank = $(".bubbles");
const share = $(".share");
const bar = $(".bar");
const labels = $(".labels");
const details = $(".details");
const sheet = $(".sheet");
const host = $(".host");
const facts = $(".facts");
const offline = $(".offline");
const stamp = $(".stamp");
const stampVersion = $(".stamp b");
const announcer = $(".announcer");
const ping = $(".ping");
const pings = $(".pings");
const flip = $(".flip");
const pixel = document
  .createElement("canvas")
  .getContext("2d", { willReadFrequently: true });
pixel.canvas.width = pixel.canvas.height = 1;

function request(path, options) {
  const abort = new AbortController();
  setTimeout(() => abort.abort(), TIMEOUT_MS);
  return fetch(
    prefix + path,
    Object.assign({ signal: abort.signal }, options),
  ).then((res) => {
    if (!res.ok)
      throw Object.assign(new Error(`HTTP ${res.status}`), {
        status: res.status,
      });
    return res;
  });
}

// Pods of a version older than /api/pod answer it with 404; they still count as up.
function getInfo(sheetOpen) {
  const fetched = sheetOpen
    ? request("/api/pod").catch((err) => {
        if (err.status === 404) return request("/api/info");
        throw err;
      })
    : request("/api/info");
  return fetched
    .then((res) => res.json())
    .then((info) => {
      if (
        typeof info.version !== "string" ||
        typeof info.hostname !== "string"
      ) {
        throw new Error("unexpected response");
      }
      return info;
    });
}

function sendPing() {
  return request("/api/echo", {
    method: "POST",
    headers: {
      "Content-Type": "application/json; charset=UTF-8",
      "X-APP": String(Math.random()),
    },
    body: JSON.stringify({ random: Math.random() }),
  });
}

function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function setText(node, text) {
  if (node.textContent !== text) node.textContent = text;
}

// A canvas keeps its previous fill when it is given something that is not a color.
function isColor(value) {
  pixel.fillStyle = "#000001";
  pixel.fillStyle = value;
  const first = pixel.fillStyle;
  pixel.fillStyle = "#000002";
  pixel.fillStyle = value;
  return pixel.fillStyle === first;
}

// Text turns dark on colors with a luminance above 0.21, where dark ink has the higher contrast.
let painted = "";
function paint(color) {
  if (color === painted || !isColor(color)) return;
  painted = color;
  pixel.fillStyle = "#fff";
  pixel.fillRect(0, 0, 1, 1);
  pixel.fillStyle = color;
  pixel.fillRect(0, 0, 1, 1);
  const rgb = Array.from(
    pixel.getImageData(0, 0, 1, 1).data.slice(0, 3),
    (x) => {
      x /= 255;
      return x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
    },
  );
  const luminance = 0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2];
  root.style.setProperty("--color", color);
  themeColor.content = color;
  root.dataset.ink = luminance > 0.21 ? "dark" : "light";
}

function fitVersion() {
  version.style.fontSize = "";
  const room = version.parentNode.clientWidth;
  if (version.offsetWidth > room) {
    version.style.fontSize =
      (parseFloat(getComputedStyle(version).fontSize) * room) /
        version.offsetWidth +
      "px";
  }
}

// The share of each version in the responses seen, lowest version first.
function shares(seen) {
  const counts = new Map();
  for (const v of seen) counts.set(v, (counts.get(v) || 0) + 1);
  return Array.from(counts, ([name, count]) => ({
    key: name,
    name,
    count,
    percent: Math.round((100 * count) / seen.length),
  })).sort((a, b) =>
    a.name.localeCompare(b.name, undefined, { numeric: true }),
  );
}

// Past three versions, names the current one and the two largest others, and sums the rest.
function legend(mix, current, total) {
  if (mix.length <= 3) return mix;
  const others = mix
    .filter((s) => s.name !== current)
    .sort((a, b) => b.count - a.count);
  const named = others
    .slice(0, 2)
    .map((s) => s.name)
    .concat(current);
  const rest = others.slice(2).reduce((n, s) => n + s.count, 0);
  return mix
    .filter((s) => named.includes(s.name))
    .concat({
      key: "",
      name: "",
      label: `${mix.length - 3} more`,
      count: rest,
      percent: Math.round((100 * rest) / total),
    });
}

// Keeps one child per key and moves it only when the order changes, so share changes
// animate and a sheet row is left alone unless its value changed.
function sync(parent, items, create, update) {
  const nodes = new Map(
    Array.from(parent.children, (node) => [node.dataset.key, node]),
  );
  items.forEach((item, i) => {
    let node = nodes.get(item.key);
    if (!node) {
      node = create(item);
      node.dataset.key = item.key;
    }
    nodes.delete(item.key);
    update(node, item);
    if (parent.children[i] !== node)
      parent.insertBefore(node, parent.children[i] || null);
  });
  for (const node of nodes.values()) node.remove();
}

const still = matchMedia("(prefers-reduced-motion: reduce)");
still.addEventListener("change", () => tank.replaceChildren());
function bubble(count) {
  const fish = logo.getBoundingClientRect();
  const unseen =
    !fish.height ||
    fish.bottom < 0 ||
    fish.top > innerHeight ||
    root.dataset.view === "details";
  if (
    still.matches ||
    document.hidden ||
    unseen ||
    tank.children.length + count > 40
  )
    return;
  for (let n = 0; n < count; n++) {
    const b = el("i");
    const size = 5 + Math.random() * 8;
    const top = fish.top + fish.height * (0.5 + Math.random() * 0.06);
    b.style.left = `${fish.left + fish.width * (0.08 + Math.random() * 0.08)}px`;
    b.style.top = `${top}px`;
    b.style.setProperty("--size", `${size}px`);
    b.style.setProperty("--rise", `${top + size}px`);
    b.style.setProperty("--drift", `${3 + Math.random() * 6}px`);
    b.style.setProperty("--wobble", `${0.9 + Math.random() * 0.8}s`);
    b.style.setProperty("--rise-time", `${5 + Math.random() * 3}s`);
    b.style.animationDelay = `${n * 0.22 + Math.random() * 0.3}s`;
    b.addEventListener("animationend", (event) => {
      if (event.animationName === "surface") b.remove();
    });
    tank.appendChild(b);
  }
}

let lostAt = 0;
function beat() {
  lostAt = 0;
  root.dataset.state = "up";
  lostLine.hidden = true;
  offline.hidden = true;
  up.hidden = false;
  setText(announcer, "");
  pulse.classList.remove("beat");
  void pulse.offsetWidth;
  pulse.classList.add("beat");
  bubble(1);
}

function lost() {
  lostAt = lostAt || Date.now();
  root.dataset.state = "down";
  up.hidden = true;
  lostLine.hidden = false;
  offline.hidden = false;
  const since = new Date(lostAt).toLocaleTimeString([], { hour12: false });
  for (const node of document.querySelectorAll(".since")) setText(node, since);
  setText(announcer, `No response since ${since}, retrying`);
}

// Paths, URLs, addresses, hashes, IDs and key=value lists read better in monospace.
function technical(value) {
  return /:\/\/|^\/|^[0-9a-f]{12,}$|^[0-9a-f-]{20,}$|^\d+\.\d+\.\d+\.\d+|=/.test(
    value,
  );
}

function syncRows(list, rows, fill) {
  sync(
    list,
    rows,
    (row) => {
      const node = el("div");
      node.append(el("dt", "", row.key), el("dd"));
      return node;
    },
    (node, row) => {
      if (node.dataset.value === row.value) return;
      node.dataset.value = row.value;
      fill(node, row);
    },
  );
}

function renderSheet(info, revision) {
  setText(host, info.hostname);
  setText(stampVersion, info.version);
  stamp.setAttribute("aria-label", `Version ${info.version}`);
  const rows = [
    { key: "Revision", value: revision.slice(0, 12), full: revision },
    { key: "Runtime", value: info.runtime },
    {
      key: "Platform",
      value: info.goos && info.goarch ? `${info.goos}/${info.goarch}` : "",
    },
    { key: "CPUs", value: info.num_cpu },
    { key: "Goroutines", value: info.num_goroutine },
  ];
  syncRows(
    facts,
    rows.filter((row) => row.value),
    (node, row) => {
      const dd = node.lastChild;
      dd.textContent = row.value;
      dd.className = row.full ? "mono" : "";
      dd.title = row.full || "";
    },
  );
  renderLines("env", info.env && Object.entries(info.env));
  renderLines(
    "headers",
    info.headers &&
      Object.entries(info.headers).map(([name, values]) => [
        name,
        [].concat(values).join(", "),
      ]),
  );
  tabStops();
}

// A pod without /api/pod has no environment or headers to show: its lists stay empty.
function renderLines(name, pairs) {
  const rows = (pairs || [])
    .map(([key, value]) => ({ key, value: String(value) }))
    .sort((a, b) => a.key.localeCompare(b.key));
  setText($(`#${name}-title span`), pairs ? String(rows.length) : "n/a");
  syncRows($(`#${name}`), rows, (node, row) => {
    const dd = node.lastChild;
    const masked = row.value.split(", ").every((v) => v === MASKED);
    node.className = !masked && row.value.length > LONG ? "long" : "";
    dd.className = !masked && technical(row.value) ? "mono" : "";
    dd.title = masked ? "Masked by podinfo" : "";
    if (masked) {
      const redacted = el("span", "redacted");
      redacted.append(el("span", "sr", "redacted"));
      dd.replaceChildren(redacted);
    } else {
      dd.textContent = row.value;
    }
  });
}

// The sheet and its columns are tab stops only while they scroll.
function tabStops() {
  for (const node of document.querySelectorAll(".sheet, .col")) {
    node.tabIndex = node.scrollHeight > node.clientHeight ? 0 : -1;
  }
}

function clearSheet() {
  setText(host, "");
  setText(stampVersion, "");
  stamp.removeAttribute("aria-label");
  for (const list of [facts, $("#env"), $("#headers")]) list.replaceChildren();
  for (const count of document.querySelectorAll(".col h2 span"))
    setText(count, "");
}

// Clips the closed sheet to the rollout bar, without a transition, so it opens out of the bar and closes into it.
function anchorSheet() {
  const box = sheet.getBoundingClientRect();
  const slot = bar.getBoundingClientRect();
  const running = sheet.getAnimations().length > 0;
  if (!running) sheet.style.transition = "none";
  sheet.style.setProperty("--bar-top", `${Math.max(0, slot.top - box.top)}px`);
  sheet.style.setProperty(
    "--bar-bottom",
    `${Math.max(0, box.bottom - slot.bottom)}px`,
  );
  if (!running) {
    void getComputedStyle(sheet).clipPath;
    sheet.style.transition = "";
  }
}

// Opening empties the sheet and polls /api/pod right away, so it never shows one pod's
// name over another pod's environment.
function showSheet(open) {
  anchorSheet();
  if (open) clearSheet();
  root.dataset.view = open ? "details" : "";
  flip.setAttribute("aria-expanded", String(open));
  stage.inert = open;
  share.inert = open;
  details.inert = !open;
  if (open) {
    sheet.focus({ preventScroll: true });
    poll();
  } else if (details.contains(document.activeElement)) {
    flip.focus();
  }
}

flip.addEventListener("click", () =>
  showSheet(root.dataset.view !== "details"),
);
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && root.dataset.view === "details")
    showSheet(false);
});

const seen = [];
function render(info, sheetOpen) {
  const revision = String(info.revision || "unknown");
  beat();
  paint(info.color);
  document.title = info.hostname;
  if (info.message !== undefined) setText(message, info.message);
  setText(hostname, info.hostname);
  setText(revisionShort, revision.slice(0, 7));
  if (info.logo && logo.getAttribute("src") !== info.logo) logo.src = info.logo;
  if (version.textContent !== info.version) {
    version.textContent = info.version;
    fitVersion();
    version.classList.remove("swap");
    void version.offsetWidth;
    version.classList.add("swap");
  }

  if (sheetOpen) renderSheet(info, revision);

  seen.push(info.version);
  if (seen.length > WINDOW) seen.shift();
  const mix = shares(seen);
  sync(
    bar,
    mix,
    () => el("i"),
    (seg, s) => {
      seg.className = s.name === info.version ? "now" : "";
      seg.style.flexGrow = s.count;
    },
  );
  sync(
    labels,
    legend(mix, info.version, seen.length),
    () => {
      const li = el("li");
      li.append(el("b"), document.createTextNode(""));
      return li;
    },
    (li, s) => {
      li.className = s.name === info.version ? "now" : "";
      li.style.flexGrow = s.count;
      li.firstChild.textContent = s.label || s.name;
      li.lastChild.textContent = ` ${s.percent}%`;
    },
  );
}

// One poll at a time: a new poll supersedes the one in flight, so responses
// never apply out of order and each response counts once in the rollout bar.
let pollSeq = 0,
  pollTimer = 0;
function poll() {
  clearTimeout(pollTimer);
  const seq = ++pollSeq;
  const sheetOpen = root.dataset.view === "details";
  getInfo(sheetOpen)
    .then(
      (info) => {
        if (seq === pollSeq) render(info, sheetOpen);
      },
      (err) => {
        if (seq !== pollSeq) return;
        console.log("Request failed", err);
        lost();
      },
    )
    .finally(() => {
      if (seq === pollSeq) pollTimer = setTimeout(poll, POLL_MS);
    });
}

// Moves the right edge of the button fill: 100% is empty, 0% is full.
function pingFill(gap, ms) {
  ping.style.setProperty("--ms", `${ms}ms`);
  ping.style.setProperty("--gap", gap);
}

// Every click sends a ping and restarts the fill; only the latest click settles the button.
let pingSeq = 0,
  pingTimer = 0,
  answered = 0,
  failed = 0;
function pinged(ok, seq, started) {
  if (ok) answered++;
  else failed++;
  if (ok) bubble(4);
  pings.textContent = `${answered} answered${failed ? `, ${failed} failed` : ""}`;
  if (seq !== pingSeq) return;
  pingTimer = setTimeout(
    () => {
      ping.dataset.state = ok ? "" : "failed";
      pingFill("0%", 180);
      if (!ok)
        pingTimer = setTimeout(() => {
          ping.dataset.state = "";
        }, PING_FAILED_MS);
    },
    Math.max(0, PING_MIN_MS - (performance.now() - started)),
  );
}

ping.addEventListener("click", () => {
  const seq = ++pingSeq,
    started = performance.now();
  clearTimeout(pingTimer);
  ping.dataset.state = "";
  pingFill("100%", 0);
  void ping.offsetWidth;
  pingFill("12%", PING_MIN_MS);
  sendPing().then(
    () => pinged(true, seq, started),
    (err) => {
      console.log("Request failed", err);
      pinged(false, seq, started);
    },
  );
});

window.addEventListener("resize", () => {
  fitVersion();
  anchorSheet();
  tabStops();
});
anchorSheet();
poll();
