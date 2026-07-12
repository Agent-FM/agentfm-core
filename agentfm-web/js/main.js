// AgentFM landing — reveals, tabs, copy buttons, live GitHub stars.
(function () {
  // Scroll reveals via IntersectionObserver (no scroll listeners).
  const io = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (e.isIntersecting) {
          e.target.classList.add('in');
          io.unobserve(e.target);
        }
      }
    },
    { threshold: 0.18, rootMargin: '0px 0px -40px 0px' }
  );
  document.querySelectorAll('.reveal').forEach((el) => io.observe(el));

  // Ledger rows type in one by one when visible.
  const ledger = document.getElementById('ledger');
  if (ledger) {
    const rows = ledger.querySelectorAll('.row');
    const lio = new IntersectionObserver(
      (entries) => {
        if (!entries[0].isIntersecting) return;
        rows.forEach((r, i) => setTimeout(() => r.classList.add('on'), 220 * i));
        lio.disconnect();
      },
      { threshold: 0.4 }
    );
    lio.observe(ledger);
  }

  // Quickstart tabs — full ARIA tabs pattern with roving tabindex + arrows.
  const tabs = Array.from(document.querySelectorAll('.tab'));
  const panes = document.querySelectorAll('[data-pane-content]');
  function selectTab(tab, focus) {
    tabs.forEach((t) => {
      const sel = t === tab;
      t.setAttribute('aria-selected', String(sel));
      t.tabIndex = sel ? 0 : -1;
    });
    panes.forEach((p) => {
      p.hidden = p.getAttribute('data-pane-content') !== tab.dataset.pane;
    });
    if (focus) tab.focus();
  }
  tabs.forEach((tab, i) => {
    tab.addEventListener('click', () => selectTab(tab, false));
    tab.addEventListener('keydown', (e) => {
      let to = -1;
      if (e.key === 'ArrowRight') to = (i + 1) % tabs.length;
      else if (e.key === 'ArrowLeft') to = (i - 1 + tabs.length) % tabs.length;
      else if (e.key === 'Home') to = 0;
      else if (e.key === 'End') to = tabs.length - 1;
      if (to >= 0) {
        e.preventDefault();
        selectTab(tabs[to], true);
      }
    });
  });

  // Copy button copies the visible pane.
  const copyBtn = document.querySelector('[data-copy]');
  if (copyBtn) {
    copyBtn.addEventListener('click', () => {
      const visible = Array.from(panes).find((p) => !p.hidden);
      if (!visible) return;
      navigator.clipboard
        .writeText(visible.textContent.trim())
        .then(() => {
          copyBtn.textContent = 'copied';
          setTimeout(() => (copyBtn.textContent = 'copy'), 1400);
        })
        .catch(() => {
          // Clipboard unavailable (permissions / non-secure context):
          // select the code so the visitor can copy manually.
          const range = document.createRange();
          range.selectNodeContents(visible);
          const sel = window.getSelection();
          sel.removeAllRanges();
          sel.addRange(range);
          copyBtn.textContent = 'press ⌘C';
          setTimeout(() => (copyBtn.textContent = 'copy'), 2200);
        });
    });
  }

  // Live GitHub stars with graceful fallback.
  fetch('https://api.github.com/repos/Agent-FM/agentfm-core')
    .then((r) => (r.ok ? r.json() : null))
    .then((d) => {
      if (!d || typeof d.stargazers_count !== 'number') return;
      const n = d.stargazers_count;
      const label = n >= 1000 ? (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k' : String(n);
      const el = document.getElementById('gh-stars');
      if (el) el.textContent = label;
    })
    .catch(() => {});
})();
