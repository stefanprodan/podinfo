// Fake API for reviewing the UI with `make ui-demo`; not embedded in the binary.
// Plays a Flagger canary release: the canary pods take a growing share of traffic
// up to half, the primary is promoted to the new version, then the API goes away
// for a few seconds. Every fourth ping fails.
(function () {
  'use strict';

  let step = 0, pings = 0;

  function info() {
    step++;
    if (step > 46 && step <= 50) throw new TypeError('demo outage');
    const promoted = step > 36;
    const canary = !promoted && Math.random() < Math.max(0, Math.min(0.5, (step - 4) / 60));
    const pods = promoted
      ? ['podinfo-primary-5d8c7f6b94-m2xvd', 'podinfo-primary-5d8c7f6b94-t9bqk', 'podinfo-primary-5d8c7f6b94-w6rhn']
      : canary
        ? ['podinfo-6f8d7c9b54-h7xkq', 'podinfo-6f8d7c9b54-r2m9d']
        : ['podinfo-primary-7b5c6d8f9-4tzwn', 'podinfo-primary-7b5c6d8f9-kq8sv', 'podinfo-primary-7b5c6d8f9-zl2pj'];
    const next = promoted || canary;
    return {
      hostname: pods[Math.floor(Math.random() * pods.length)],
      version: next ? '7.0.1' : '7.0.0',
      revision: next ? '59437a333ca331d7f50947a03d389dc26e7a5123' : '1c192f7a8e6990b4d2f0c3e5a7b9d1f3e5a7c9b1',
      color: next ? '#17694f' : '#1f4e8c',
      message: 'greetings from podinfo'
    };
  }

  function json(body, status) {
    return new Response(JSON.stringify(body), { status: status, headers: { 'Content-Type': 'application/json' } });
  }

  window.fetch = function (url) {
    if (String(url).endsWith('/api/info')) {
      return new Promise((resolve) => resolve(json(info(), 200)));
    }
    return new Promise((resolve, reject) => {
      const fail = ++pings % 4 === 0;
      setTimeout(() => {
        if (fail) reject(new TypeError('demo failure')); else resolve(json({}, 202));
      }, 200 + Math.random() * 900);
    });
  };
})();
