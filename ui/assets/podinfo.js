(function () {
  'use strict';

  const POLL_MS = 2000;
  const TIMEOUT_MS = 1500;
  const WINDOW = 30;
  const PING_MIN_MS = 350;
  const PING_FAILED_MS = 1600;

  const prefix = document.body.dataset.prefix;
  const $ = (selector) => document.querySelector(selector);
  const root = document.documentElement;
  const version = $('.version');
  const ping = $('.ping');
  const pixel = document.createElement('canvas').getContext('2d', { willReadFrequently: true });
  pixel.canvas.width = pixel.canvas.height = 1;

  function request(path, options) {
    const abort = new AbortController();
    setTimeout(() => abort.abort(), TIMEOUT_MS);
    return fetch(prefix + path, Object.assign({ signal: abort.signal }, options)).then((res) => {
      if (!res.ok) throw new Error('HTTP ' + res.status);
      return res;
    });
  }

  function getInfo() {
    return request('/api/info').then((res) => res.json()).then((info) => {
      if (typeof info.version !== 'string' || typeof info.hostname !== 'string') {
        throw new Error('unexpected /api/info response');
      }
      return info;
    });
  }

  function sendPing() {
    return request('/api/echo', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=UTF-8', 'X-APP': String(Math.random()) },
      body: JSON.stringify({ random: Math.random() })
    });
  }

  // A canvas keeps its previous fill when it is given something that is not a color.
  function isColor(value) {
    pixel.fillStyle = '#000001';
    pixel.fillStyle = value;
    const first = pixel.fillStyle;
    pixel.fillStyle = '#000002';
    pixel.fillStyle = value;
    return pixel.fillStyle === first;
  }

  // Applies a valid color as the background and picks the text color with the higher contrast on it.
  let painted = '';
  function paint(color) {
    if (color === painted || !isColor(color)) return;
    painted = color;
    pixel.fillStyle = '#fff';
    pixel.fillRect(0, 0, 1, 1);
    pixel.fillStyle = color;
    pixel.fillRect(0, 0, 1, 1);
    const rgb = Array.from(pixel.getImageData(0, 0, 1, 1).data.slice(0, 3), (x) => {
      x /= 255;
      return x <= 0.03928 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4);
    });
    const luminance = 0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2];
    root.style.setProperty('--color', color);
    $('meta[name="theme-color"]').content = color;
    root.dataset.ink = luminance > 0.21 ? 'dark' : 'light';
  }

  function fitVersion() {
    version.style.fontSize = '';
    const room = version.parentNode.clientWidth;
    if (version.offsetWidth > room) {
      version.style.fontSize = parseFloat(getComputedStyle(version).fontSize) * room / version.offsetWidth + 'px';
    }
  }

  // The share of each version in the responses seen, oldest version first.
  function shares(seen) {
    const counts = new Map();
    seen.forEach((v) => counts.set(v, (counts.get(v) || 0) + 1));
    return Array.from(counts, ([name, count]) => ({ name, count, percent: Math.round(100 * count / seen.length) }))
      .sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true }));
  }

  // Past three versions, names the current one and the two largest others, and sums the rest.
  function legend(mix, current, total) {
    if (mix.length <= 3) return mix;
    const others = mix.filter((s) => s.name !== current).sort((a, b) => b.count - a.count);
    const named = others.slice(0, 2).map((s) => s.name).concat(current);
    const rest = others.slice(2).reduce((n, s) => n + s.count, 0);
    return mix.filter((s) => named.includes(s.name)).concat({
      name: '', label: (mix.length - 3) + ' more', count: rest, percent: Math.round(100 * rest / total)
    });
  }

  // Keeps one child per version and moves it only when the order changes, so a share change animates.
  function sync(parent, tag, mix, current, update) {
    const nodes = new Map(Array.from(parent.children, (el) => [el.dataset.name, el]));
    mix.forEach((s, i) => {
      let el = nodes.get(s.name);
      if (!el) {
        el = document.createElement(tag);
        el.dataset.name = s.name;
        if (tag === 'li') el.append(document.createElement('b'), document.createTextNode(''));
      }
      nodes.delete(s.name);
      el.className = s.name === current ? 'now' : '';
      update(el, s);
      if (parent.children[i] !== el) parent.insertBefore(el, parent.children[i] || null);
    });
    nodes.forEach((el) => el.remove());
  }

  // Releases bubbles from the cuddlefish; they rise to the surface and are removed there.
  const still = matchMedia('(prefers-reduced-motion: reduce)');
  still.addEventListener('change', () => $('.bubbles').replaceChildren());
  function bubble(count) {
    const tank = $('.bubbles');
    const fish = $('.logo').getBoundingClientRect();
    const offscreen = !fish.height || fish.bottom < 0 || fish.top > innerHeight;
    if (still.matches || document.hidden || offscreen || tank.children.length + count > 40) return;
    for (let n = 0; n < count; n++) {
      const el = document.createElement('i');
      const size = 5 + Math.random() * 8;
      const top = fish.top + fish.height * (0.5 + Math.random() * 0.06);
      el.style.left = fish.left + fish.width * (0.08 + Math.random() * 0.08) + 'px';
      el.style.top = top + 'px';
      el.style.setProperty('--size', size + 'px');
      el.style.setProperty('--rise', top + size + 'px');
      el.style.setProperty('--drift', 3 + Math.random() * 6 + 'px');
      el.style.setProperty('--sway', 0.9 + Math.random() * 0.8 + 's');
      el.style.setProperty('--time', 5 + Math.random() * 3 + 's');
      el.style.animationDelay = n * 0.22 + Math.random() * 0.3 + 's';
      el.addEventListener('animationend', (event) => {
        if (event.animationName === 'surface') el.remove();
      });
      tank.appendChild(el);
    }
  }

  // Restarts the pulse next to the pod name, the sign that a response just arrived.
  let lostAt = 0;
  function beat() {
    lostAt = 0;
    root.dataset.state = 'up';
    $('.lost').hidden = true;
    $('.up').hidden = false;
    const pulse = $('.pulse');
    pulse.classList.remove('beat');
    void pulse.offsetWidth;
    pulse.classList.add('beat');
    bubble(1);
  }

  function lost() {
    lostAt = lostAt || Date.now();
    root.dataset.state = 'down';
    $('.up').hidden = true;
    $('.lost').hidden = false;
    $('.since').textContent = new Date(lostAt).toLocaleTimeString([], { hour12: false });
  }

  const seen = [];
  function render(info) {
    beat();
    paint(info.color);
    document.title = info.hostname;
    $('.message').textContent = info.message || '';
    $('.hostname').textContent = info.hostname;
    $('.revision').textContent = String(info.revision || 'unknown').slice(0, 7);
    const logo = $('.logo');
    if (info.logo && logo.getAttribute('src') !== info.logo) logo.src = info.logo;
    if (version.textContent !== info.version) {
      version.textContent = info.version;
      fitVersion();
      version.classList.remove('swap');
      void version.offsetWidth;
      version.classList.add('swap');
    }

    seen.push(info.version);
    if (seen.length > WINDOW) seen.shift();
    const mix = shares(seen);
    sync($('.bar'), 'i', mix, info.version, (seg, s) => {
      seg.style.flexGrow = s.count;
    });
    sync($('.labels'), 'li', legend(mix, info.version, seen.length), info.version, (li, s) => {
      li.style.flexGrow = s.count;
      li.firstChild.textContent = s.label || s.name;
      li.lastChild.textContent = ' ' + s.percent + '%';
    });
  }

  // The next poll starts after this one settles, so responses cannot arrive out of order.
  function poll() {
    getInfo().then(render).catch((err) => {
      console.log('Request failed', err);
      lost();
    }).then(() => setTimeout(poll, POLL_MS));
  }

  // Moves the right edge of the button fill: 100% is empty, 0% is full.
  function pingFill(gap, ms) {
    ping.style.setProperty('--ms', ms + 'ms');
    ping.style.setProperty('--gap', gap);
  }

  // Every click sends a ping and restarts the fill; only the latest click settles the button.
  let pingSeq = 0, pingTimer = 0, answered = 0, failed = 0;
  function pinged(ok, seq, started) {
    if (ok) answered++; else failed++;
    if (ok) bubble(4);
    $('.pings').textContent = answered + ' answered' + (failed ? ', ' + failed + ' failed' : '');
    if (seq !== pingSeq) return;
    pingTimer = setTimeout(() => {
      ping.dataset.state = ok ? '' : 'failed';
      pingFill('0%', 180);
      if (!ok) pingTimer = setTimeout(() => { ping.dataset.state = ''; }, PING_FAILED_MS);
    }, Math.max(0, PING_MIN_MS - (performance.now() - started)));
  }

  ping.addEventListener('click', () => {
    const seq = ++pingSeq, started = performance.now();
    clearTimeout(pingTimer);
    ping.dataset.state = '';
    pingFill('100%', 0);
    void ping.offsetWidth;
    pingFill('12%', PING_MIN_MS);
    sendPing().then(() => pinged(true, seq, started), (err) => {
      console.log('Request failed', err);
      pinged(false, seq, started);
    });
  });

  window.addEventListener('resize', fitVersion);
  poll();
})();
