// MikVoc main.js — shared utilities

(function () {
  function readCookie(name) {
    const parts = (document.cookie || '').split(';');
    for (let i = 0; i < parts.length; i++) {
      const p = parts[i].trim();
      if (p.indexOf(name + '=') === 0) {
        const raw = p.slice(name.length + 1);
        try {
          return decodeURIComponent(raw);
        } catch (e) {
          return raw;
        }
      }
    }
    return '';
  }

  function csrfToken() {
    if (typeof window.__MIKVOC_CSRF__ === 'string' && window.__MIKVOC_CSRF__) {
      return window.__MIKVOC_CSRF__;
    }
    const m = document.querySelector('meta[name="csrf-token"]');
    const fromMeta = m ? (m.getAttribute('content') || '').trim() : '';
    if (fromMeta) return fromMeta;
    return readCookie('mikvoc_csrf') || '';
  }

  function injectCSRFIntoForm(f) {
    if (!f || !f.tagName || f.tagName.toLowerCase() !== 'form') return;
    const method = (f.getAttribute('method') || 'get').toLowerCase();
    if (method !== 'post' && method !== 'put' && method !== 'patch' && method !== 'delete') return;
    const token = csrfToken();
    if (!token) return;
    let input = f.querySelector('input[name="csrf_token"]');
    if (!input) {
      input = document.createElement('input');
      input.type = 'hidden';
      input.name = 'csrf_token';
      f.appendChild(input);
    }
    input.value = token;
  }

  function injectCSRFForms(root) {
    const scope = root && root.querySelectorAll ? root : document;
    if (scope.tagName && scope.tagName.toLowerCase() === 'form') {
      injectCSRFIntoForm(scope);
    }
    const forms = scope.querySelectorAll ? scope.querySelectorAll('form') : [];
    forms.forEach(injectCSRFIntoForm);
  }

  document.addEventListener(
    'submit',
    function (e) {
      if (e.target && e.target.tagName === 'FORM') {
        injectCSRFIntoForm(e.target);
      }
    },
    true
  );

  if (typeof HTMLFormElement !== 'undefined' && HTMLFormElement.prototype) {
    const nativeSubmit = HTMLFormElement.prototype.submit;
    HTMLFormElement.prototype.submit = function () {
      injectCSRFIntoForm(this);
      return nativeSubmit.call(this);
    };
  }

  const origFetch = window.fetch;
  window.fetch = function (input, init) {
    const req = input instanceof Request ? input : null;
    init = Object.assign({}, init || {});
    let method = (init.method || (req && req.method) || 'GET').toString().toUpperCase();
    if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS' && method !== 'TRACE') {
      const headers = new Headers(req ? req.headers : undefined);
      new Headers(init.headers || undefined).forEach(function (v, k) {
        headers.set(k, v);
      });
      if (
        !headers.has('X-CSRF-Token') &&
        !headers.has('X-CSRF-TOKEN') &&
        !headers.has('X-XSRF-TOKEN')
      ) {
        const token = csrfToken();
        if (token) headers.set('X-CSRF-Token', token);
      }
      if (!headers.has('X-Requested-With')) {
        headers.set('X-Requested-With', 'fetch');
      }
      init.headers = headers;
      if (init.credentials === undefined) {
        init.credentials = 'same-origin';
      }
    }
    return origFetch.call(this, input, init);
  };

  function bootCSRF() {
    injectCSRFForms(document);
    if (window.MutationObserver) {
      const mo = new MutationObserver(function (mutations) {
        for (let i = 0; i < mutations.length; i++) {
          const nodes = mutations[i].addedNodes;
          for (let j = 0; j < nodes.length; j++) {
            const n = nodes[j];
            if (n.nodeType !== 1) continue;
            injectCSRFForms(n);
          }
        }
      });
      mo.observe(document.documentElement, { childList: true, subtree: true });
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', bootCSRF);
  } else {
    bootCSRF();
  }

  window.mikvocCSRF = { token: csrfToken, inject: injectCSRFForms };

  let busyDepth = 0;
  let busyEl = null;

  function ensureBusyEl() {
    if (busyEl && document.body.contains(busyEl)) return busyEl;
    busyEl = document.getElementById('mikvoc-busy');
    if (busyEl) return busyEl;
    busyEl = document.createElement('div');
    busyEl.id = 'mikvoc-busy';
    busyEl.setAttribute('aria-live', 'polite');
    busyEl.setAttribute('aria-busy', 'true');
    busyEl.hidden = true;
    busyEl.innerHTML =
      '<div class="mikvoc-busy-card">' +
      '<span class="material-symbols-outlined mikvoc-busy-spin">progress_activity</span>' +
      '<div class="mikvoc-busy-title">Memproses...</div>' +
      '<div class="mikvoc-busy-msg"></div>' +
      '<div class="mikvoc-busy-hint">Jangan refresh halaman</div>' +
      '</div>';
    const style = document.createElement('style');
    style.textContent =
      '#mikvoc-busy{position:fixed;inset:0;z-index:9999;display:flex;align-items:center;justify-content:center;' +
      'background:rgba(15,23,42,.55);backdrop-filter:blur(2px)}' +
      '#mikvoc-busy[hidden]{display:none!important}' +
      '.mikvoc-busy-card{min-width:240px;max-width:90vw;background:var(--card,#fff);color:var(--foreground,#0f172a);' +
      'border:1px solid var(--border,#e2e8f0);border-radius:12px;padding:1.25rem 1.5rem;text-align:center;' +
      'box-shadow:0 20px 40px rgba(0,0,0,.2)}' +
      '.dark .mikvoc-busy-card,.dark #mikvoc-busy .mikvoc-busy-card{background:#0f172a;color:#f8fafc;border-color:#1e293b}' +
      '.mikvoc-busy-spin{font-size:36px;display:inline-block;animation:mikvoc-spin 0.9s linear infinite;color:#6366f1}' +
      '@keyframes mikvoc-spin{to{transform:rotate(360deg)}}' +
      '.mikvoc-busy-title{margin-top:.75rem;font-weight:600;font-size:.95rem}' +
      '.mikvoc-busy-msg{margin-top:.35rem;font-size:.8rem;opacity:.8;min-height:1.1em}' +
      '.mikvoc-busy-hint{margin-top:.5rem;font-size:.7rem;opacity:.55}';
    document.head.appendChild(style);
    document.body.appendChild(busyEl);
    return busyEl;
  }

  function showBusy(message) {
    busyDepth++;
    const el = ensureBusyEl();
    const msg = el.querySelector('.mikvoc-busy-msg');
    if (msg) msg.textContent = message || 'Mohon tunggu...';
    el.hidden = false;
    document.documentElement.style.overflow = 'hidden';
  }

  function hideBusy() {
    busyDepth = Math.max(0, busyDepth - 1);
    if (busyDepth > 0) return;
    if (busyEl) busyEl.hidden = true;
    document.documentElement.style.overflow = '';
  }

  function withBusy(message, promiseOrFn) {
    showBusy(message);
    let p;
    try {
      p = typeof promiseOrFn === 'function' ? promiseOrFn() : promiseOrFn;
    } catch (err) {
      hideBusy();
      return Promise.reject(err);
    }
    return Promise.resolve(p).finally(hideBusy);
  }

  window.mikvocBusy = { show: showBusy, hide: hideBusy, with: withBusy };

  document.addEventListener('click', (e) => {
    const t = e.target.closest('[data-confirm]');
    if (!t) return;
    const msg = t.getAttribute('data-confirm');
    if (!window.confirm(msg)) {
      e.preventDefault();
      e.stopImmediatePropagation();
    }
  });

  document.addEventListener('submit', (e) => {
    const form = e.target;
    if (!form || !form.matches || !form.matches('form[data-loading]')) return;
    if (form.dataset.busyShown === '1') return;
    form.dataset.busyShown = '1';
    const label = form.getAttribute('data-loading-text') || 'Memproses...';
    showBusy(label);
    // Defer disable so submitter name/value still enter the form body.
    setTimeout(function () {
      const buttons = form.querySelectorAll(
        'button[type="submit"], button:not([type]), input[type="submit"]'
      );
      buttons.forEach(function (btn) {
        if (btn.disabled) return;
        btn.disabled = true;
        if (btn.tagName === 'BUTTON') {
          btn.dataset.originalHtml = btn.innerHTML;
          btn.innerHTML =
            '<span class="material-symbols-outlined text-[18px] animate-spin">progress_activity</span> Memproses...';
        }
      });
      form.querySelectorAll('input, select, textarea, button').forEach(function (el) {
        if (el.type === 'hidden') return;
        el.disabled = true;
      });
    }, 0);
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      if (busyDepth > 0) return;
      document.querySelectorAll('.modal-backdrop').forEach((el) => el.remove());
    }
  });
})();
