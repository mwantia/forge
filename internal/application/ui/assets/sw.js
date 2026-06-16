const CACHE = 'forge-ui-__ASSET_VERSION__';

self.addEventListener('install', e => {
  self.skipWaiting();
});

self.addEventListener('activate', e => {
  e.waitUntil(
    caches.keys()
      .then(keys => Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k))))
      .then(() => self.clients.claim())
      .then(() => self.clients.matchAll({ type: 'window' }))
      .then(clients => clients.forEach(c => c.navigate(c.url)))
  );
});

self.addEventListener('fetch', e => {
  const url = new URL(e.request.url);
  if (e.request.method !== 'GET') return;

  // Versioned static assets already carry Cache-Control: immutable from the
  // server — cache-first here so they survive offline and load instantly.
  if (url.pathname.startsWith('/ui/assets/')) {
    e.respondWith(
      caches.match(e.request).then(cached =>
        cached ||
        fetch(e.request).then(res => {
          const clone = res.clone();
          caches.open(CACHE).then(c => c.put(e.request, clone));
          return res;
        })
      )
    );
    return;
  }

  // All other requests (HTML pages, HTMX partials, API calls) must always hit
  // the network — never serve stale HTML from the cache.
});
