import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './index.css';

// Global API Fetch Interceptor: injects X-Sirius-Token and ensures application/json on state-changing requests
const originalFetch = window.fetch;
let cachedToken: string | null = null;

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
  return match ? match[2] : null;
}

window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

  if (url.startsWith('/api/') || url.includes('/api/')) {
    const options: RequestInit = init ? { ...init } : {};
    const headers = new Headers(options.headers || {});

    // 1. Resolve token if not yet cached
    if (!cachedToken) {
      cachedToken = getCookie('sirius_token');
    }
    if (!cachedToken && !url.includes('/api/auth/token')) {
      try {
        const res = await originalFetch('/api/auth/token');
        if (res.ok) {
          const data = await res.json();
          if (data && data.token) {
            cachedToken = data.token;
          }
        }
      } catch {
        // ignore network error during token warmup
      }
    }

    if (cachedToken && !headers.has('X-Sirius-Token')) {
      headers.set('X-Sirius-Token', cachedToken);
    }

    // 2. Ensure application/json Content-Type on state-changing requests
    const method = (options.method || 'GET').toUpperCase();
    if (['POST', 'PUT', 'DELETE', 'PATCH'].includes(method)) {
      if (!headers.has('Content-Type')) {
        headers.set('Content-Type', 'application/json');
      }
    }

    options.headers = headers;
    return originalFetch(input, options);
  }

  return originalFetch(input, init);
};

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
