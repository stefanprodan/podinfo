const prefix = document.body.dataset.prefix;
const $ = (selector) => document.querySelector(selector);
const items = document.querySelectorAll('.timeline li');

let calls = 0;
let pings = 0;

async function getInfo() {
  try {
    const res = await fetch(`${prefix}/api/info`);
    const info = await res.json();

    document.title = info.hostname;
    $('.hero').style.backgroundColor = info.color;
    if ($('.logo').getAttribute('src') !== info.logo) {
      $('.logo').src = info.logo;
    }
    $('.message').textContent = info.message;
    $('.hostname').textContent = info.hostname;
    $('.version').textContent = info.version;
    $('.revision').textContent = info.revision;

    const patch = parseInt(info.version.split('.').pop(), 10);
    const current = items[calls % 2];
    items[(calls + 1) % 2].classList.add('stale');
    current.className = patch % 2 === 0 ? 'blue' : 'green';
    current.querySelector('h3').textContent = info.version;
    calls++;
  } catch (err) {
    console.log('Request failed', err);
  }
}

async function ping() {
  try {
    const res = await fetch(`${prefix}/api/echo`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json; charset=UTF-8',
        'X-APP': Math.random(),
      },
      body: JSON.stringify({ random: Math.random() }),
    });
    $('.badge').textContent = ++pings;
    console.log('Request successful', await res.json());
  } catch (err) {
    console.log('Request failed', err);
  }
}

$('.ping').addEventListener('click', ping);
getInfo();
setInterval(getInfo, 3000);
