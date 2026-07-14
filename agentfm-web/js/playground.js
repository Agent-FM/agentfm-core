(function () {
  const API = location.hostname.endsWith('agentfm.net') ? '/papi' : 'https://agentfm.net/papi';
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  const views = {
    radar: document.getElementById('view-radar'),
    agents: document.getElementById('view-agents'),
    chat: document.getElementById('view-chat'),
  };
  const radarLog = document.getElementById('radar-log');
  const grid = document.getElementById('agent-grid');
  const emptyBox = document.getElementById('agents-empty');
  const countEl = document.getElementById('agents-count');
  const meshBadge = document.querySelector('[data-mesh-count]');

  let agents = [];
  let pollStop = false;
  let pollFails = 0;
  let activeAgent = null;
  let streaming = false;
  let aborter = null;
  let pendingBack = false;
  const transcripts = new Map();

  const num = (x) => (Number.isFinite(Number(x)) ? Number(x) : 0);
  const pct = (x) => Math.max(0, Math.min(100, Math.round(num(x))));

  function esc(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function show(name) {
    Object.entries(views).forEach(([k, el]) => { el.hidden = k !== name; });
    if (name === 'agents' && window.gsap && !reduce) {
      gsap.fromTo('.agent-card', { opacity: 0, y: 26 }, { opacity: 1, y: 0, duration: 0.6, stagger: 0.09, ease: 'power3.out', clearProps: 'all' });
    }
  }

  async function fetchAgents() {
    const res = await fetch(API + '/agents', { headers: { Accept: 'application/json' } });
    if (!res.ok) throw new Error('gateway replied ' + res.status);
    const data = await res.json();
    if (!data.success) throw new Error(data.error || 'gateway error');
    return data.agents || [];
  }

  function typeLine(text, cls) {
    return new Promise((done) => {
      const ln = document.createElement('span');
      ln.className = 'ln' + (cls ? ' ' + cls : '');
      radarLog.appendChild(ln);
      if (reduce) { ln.textContent = text; done(); return; }
      const cursor = document.createElement('span');
      cursor.className = 'cursor';
      ln.appendChild(cursor);
      let i = 0;
      const tick = () => {
        i += 1 + (Math.random() * 2 | 0);
        ln.textContent = text.slice(0, i);
        ln.appendChild(cursor);
        if (i < text.length) setTimeout(tick, 22);
        else { cursor.remove(); done(); }
      };
      tick();
    });
  }

  function markOk(suffix) {
    const last = radarLog.lastElementChild;
    if (!last) return;
    const ok = document.createElement('span');
    ok.className = 'ok';
    ok.textContent = '  ' + (suffix || 'ok');
    last.appendChild(ok);
  }

  async function radarSequence() {
    radarLog.innerHTML = '';
    const fetching = fetchAgents();
    fetching.catch(() => {});
    const t0 = performance.now();
    await typeLine('> dialing lighthouse 12D3KooWQHw…Bs5zL');
    markOk();
    await typeLine('> subscribed agentfm-telemetry-v1');
    markOk();
    await typeLine('> listening for agent heartbeats…');
    let list;
    try {
      list = await fetching;
    } catch (e) {
      await typeLine('> lighthouse unreachable — ' + e.message, 'err');
      const retry = document.createElement('button');
      retry.className = 'btn btn-ghost radar-retry';
      retry.textContent = 'Retry scan';
      retry.addEventListener('click', () => { retry.remove(); radarSequence(); });
      radarLog.appendChild(retry);
      return;
    }
    const online = list.filter((a) => a.online);
    if (!reduce) await new Promise((r) => setTimeout(r, 700));
    for (const a of online.slice(0, 4)) {
      await typeLine('> found ' + (a.name || 'agent').toLowerCase() + ' · ' + String(a.peer_id).slice(0, 10) + '… · ' + (a.agent_capability || 'general'));
      markOk('');
      if (!reduce) await new Promise((r) => setTimeout(r, 240));
    }
    const wait = Math.max(0, (reduce ? 300 : 4600) - (performance.now() - t0));
    await new Promise((r) => setTimeout(r, wait));
    await typeLine('> ' + online.length + ' agent' + (online.length === 1 ? '' : 's') + ' answering');
    markOk('signal locked');
    await new Promise((r) => setTimeout(r, reduce ? 150 : 700));
    agents = list;
    renderAgents();
    if (window.gsap && !reduce) {
      await new Promise((done) => {
        gsap.timeline({ onComplete: done })
          .to('.radar', { scale: 0.86, autoAlpha: 0, duration: 0.65, ease: 'power2.in' }, 0)
          .to('.radar-log', { autoAlpha: 0, y: 14, duration: 0.5, ease: 'power2.in' }, 0.1);
      });
      gsap.set(['.radar', '.radar-log'], { clearProps: 'all' });
    }
    show('agents');
    schedulePoll();
    setBadge(online.length + ' on mesh', false);
  }

  const repCache = new Map();
  function loadRep(peer) {
    if (repCache.has(peer)) return repCache.get(peer);
    const p = (async () => {
      const rep = await (await fetch(API + '/rep/' + encodeURIComponent(peer))).json();
      let entries = [];
      try {
        const log = await (await fetch(API + '/replog/' + encodeURIComponent(peer))).json();
        entries = (log.entries || []).slice(0, 12);
        const comments = entries.filter((e) => e.kind === 'Comment' && e.text_cid).slice(0, 3);
        await Promise.all(comments.map(async (c) => {
          try {
            const body = await (await fetch(API + '/comment/' + encodeURIComponent(peer) + '/' + encodeURIComponent(c.text_cid))).json();
            c.body = body.body;
          } catch (e) {}
        }));
      } catch (e) {}
      return { count: num(rep.rating_count), weighted: num(rep.scores && rep.scores.honesty), entries };
    })();
    repCache.set(peer, p);
    p.catch(() => repCache.delete(peer));
    return p;
  }

  function hydrateRepChip(peer) {
    loadRep(peer).then((r) => {
      const chip = grid.querySelector('[data-peer="' + CSS.escape(peer) + '"] .chip-rep');
      if (chip) chip.textContent = '★ ' + r.count + ' rating' + (r.count === 1 ? '' : 's');
    }).catch(() => {});
  }

  function timeAgo(iso) {
    const s = (Date.now() - new Date(iso).getTime()) / 1000;
    if (!Number.isFinite(s) || s < 0) return '';
    if (s < 3600) return Math.max(1, Math.round(s / 60)) + 'm ago';
    if (s < 86400) return Math.round(s / 3600) + 'h ago';
    return Math.round(s / 86400) + 'd ago';
  }

  function setBadge(text, stale) {
    if (meshBadge) {
      meshBadge.hidden = false;
      meshBadge.innerHTML = '&#9679; ' + esc(text);
      meshBadge.classList.toggle('stale', !!stale);
    }
  }

  function statusOf(a) {
    if (!a.online) return { key: 'full', label: '● offline', disabled: true };
    if (num(a.current_tasks) >= num(a.max_tasks)) return { key: 'full', label: '● at capacity', disabled: true };
    if (a.dispatch_allowed === false) return { key: 'full', label: '● gated', disabled: true };
    if (num(a.current_tasks) > 0) return { key: 'busy', label: '● busy · slots free', disabled: false };
    return { key: 'ok', label: '● available', disabled: false };
  }

  function hardwareChip(a) {
    const m = /\(([^)]+)\)/.exec(a.hardware || '');
    return m ? m[1].toLowerCase() : 'cpu';
  }

  function modelOf(a) {
    const m = (a.hardware || '').split('(')[0].trim();
    return m || null;
  }

  function cardHTML(a) {
    const st = statusOf(a);
    const gpu = a.has_gpu
      ? '<div class="meter m-gpu gpu"><span>gpu</span><div class="bar"><i style="width:' + pct(a.gpu_usage_pct) + '%"></i></div><b>' + pct(a.gpu_usage_pct) + '%</b></div>'
      : '<div class="meter m-gpu gpu"><span>gpu</span><span class="none">—</span><b class="none">none</b></div>';
    const model = modelOf(a);
    return (
      '<div class="ac-head">' +
        '<span class="ac-avatar">' + esc((a.name || '?')[0].toUpperCase()) + '</span>' +
        '<div class="ac-id"><h3>' + esc(a.name || 'Agent') + '</h3></div>' +
        '<span class="ac-cap">' + esc(a.agent_capability || 'general') + '</span>' +
        '<span class="ac-status ' + st.key + '">' + st.label + '</span>' +
      '</div>' +
      '<p class="ac-desc">' + esc(a.description || 'No description advertised.') + '</p>' +
      '<div class="ac-meters">' +
        '<div class="meter m-cpu"><span>cpu</span><div class="bar"><i style="width:' + pct(a.cpu_usage_pct) + '%"></i></div><b>' + pct(a.cpu_usage_pct) + '%</b></div>' +
        gpu +
        '<div class="meter m-ram"><span>ram</span><span class="none"></span><b>' + num(a.ram_free_gb).toFixed(1) + ' GB free</b></div>' +
      '</div>' +
      '<div class="ac-foot">' +
        '<span class="ac-chip chip-tasks' + (num(a.current_tasks) >= num(a.max_tasks) ? ' warn' : '') + '">tasks ' + num(a.current_tasks) + '/' + num(a.max_tasks) + '</span>' +
        '<span class="ac-chip chip-rep">★ …</span>' +
        (model ? '<span class="ac-chip">' + esc(model) + '</span>' : '') +
        '<span class="ac-chip">' + esc(hardwareChip(a)) + '</span>' +
        '<span class="ac-open">' + (st.disabled ? 'unavailable' : 'chat →') + '</span>' +
      '</div>'
    );
  }

  function renderAgents() {
    grid.innerHTML = '';
    const online = agents.filter((a) => a.online);
    countEl.textContent = online.length + ' online';
    emptyBox.hidden = online.length > 0;
    online.forEach((a) => {
      let card;
      try {
        const st = statusOf(a);
        card = document.createElement('button');
        card.type = 'button';
        card.className = 'agent-card';
        card.disabled = st.disabled;
        card.dataset.peer = a.peer_id;
        card.dataset.gpu = a.has_gpu ? '1' : '0';
        card.innerHTML = cardHTML(a);
        card.addEventListener('click', () => openChat(a.peer_id));
        grid.appendChild(card);
      } catch (e) {
        if (card) card.remove();
      }
    });
    online.forEach((a) => hydrateRepChip(a.peer_id));
  }

  function updateCards() {
    const online = agents.filter((a) => a.online);
    const stale = online.some((a) => {
      const card = grid.querySelector('[data-peer="' + CSS.escape(a.peer_id) + '"]');
      return !card || card.dataset.gpu !== (a.has_gpu ? '1' : '0');
    });
    if (stale || grid.children.length !== online.length) return renderAgents();
    online.forEach((a) => {
      const card = grid.querySelector('[data-peer="' + CSS.escape(a.peer_id) + '"]');
      const st = statusOf(a);
      card.disabled = st.disabled;
      const cpuBar = card.querySelector('.m-cpu .bar i');
      const cpuNum = card.querySelector('.m-cpu b');
      if (cpuBar) cpuBar.style.width = pct(a.cpu_usage_pct) + '%';
      if (cpuNum) cpuNum.textContent = pct(a.cpu_usage_pct) + '%';
      if (a.has_gpu) {
        const gpuBar = card.querySelector('.m-gpu .bar i');
        const gpuNum = card.querySelector('.m-gpu b');
        if (gpuBar) gpuBar.style.width = pct(a.gpu_usage_pct) + '%';
        if (gpuNum) gpuNum.textContent = pct(a.gpu_usage_pct) + '%';
      }
      const ram = card.querySelector('.m-ram b');
      if (ram) ram.textContent = num(a.ram_free_gb).toFixed(1) + ' GB free';
      const status = card.querySelector('.ac-status');
      status.className = 'ac-status ' + st.key;
      status.textContent = st.label;
      const tasks = card.querySelector('.chip-tasks');
      tasks.textContent = 'tasks ' + num(a.current_tasks) + '/' + num(a.max_tasks);
      tasks.classList.toggle('warn', num(a.current_tasks) >= num(a.max_tasks));
      const open = card.querySelector('.ac-open');
      open.textContent = st.disabled ? 'unavailable' : 'chat →';
    });
  }

  async function pollOnce() {
    try {
      agents = await fetchAgents();
      pollFails = 0;
      if (!views.agents.hidden) updateCards();
      if (!views.chat.hidden && activeAgent) updateChatTele();
      const online = agents.filter((a) => a.online).length;
      countEl.textContent = online + ' online';
      setBadge(online + ' on mesh', false);
    } catch (e) {
      pollFails += 1;
      if (pollFails >= 3) {
        countEl.textContent = 'telemetry stale';
        setBadge('telemetry stale', true);
      }
    }
  }

  function schedulePoll() {
    if (pollStop) return;
    setTimeout(async () => {
      await pollOnce();
      schedulePoll();
    }, 3000);
  }

  const chatLog = document.getElementById('chat-log');
  const chatForm = document.getElementById('chat-form');
  const chatInput = document.getElementById('chat-input');
  const chatSend = document.getElementById('chat-send');

  function openChat(peerId) {
    const a = agents.find((x) => x.peer_id === peerId);
    if (!a || streaming) return;
    activeAgent = a;
    document.getElementById('chat-avatar').textContent = (a.name || '?')[0].toUpperCase();
    document.getElementById('chat-name').textContent = a.name || 'Agent';
    document.getElementById('chat-cap').textContent = a.agent_capability || 'general';
    chatLog.innerHTML = transcripts.get(peerId) || '';
    if (!transcripts.has(peerId)) {
      meta('connected to ' + (a.name || 'agent') + ' · ' + peerId.slice(0, 12) + '… · fresh sandbox per task');
    }
    renderRepStrip(peerId);
    updateChatTele();
    show('chat');
    scrollLog();
    chatInput.focus();
  }

  function renderRepStrip(peerId) {
    const strip = document.getElementById('chat-rep');
    strip.hidden = true;
    strip.innerHTML = '';
    loadRep(peerId).then((r) => {
      if (activeAgent && activeAgent.peer_id !== peerId) return;
      const head = document.createElement('span');
      head.className = 'rep-head';
      const w = r.weighted;
      head.textContent = 'signed ledger · ' + r.count + ' rating' + (r.count === 1 ? '' : 's') +
        ' · weighted by this node ' + (w >= 0 ? '+' : '') + w.toFixed(2);
      strip.appendChild(head);
      const comments = r.entries.filter((e) => e.kind === 'Comment' && e.body).slice(0, 2);
      const ratings = r.entries.filter((e) => e.kind === 'Rating').slice(0, 2);
      const shown = comments.concat(ratings)
        .sort((x, y) => new Date(y.received_at) - new Date(x.received_at))
        .slice(0, 4);
      shown.forEach((e) => {
        const row = document.createElement('span');
        row.className = 'rep-row';
        if (e.kind === 'Rating') {
          const s = num(e.score);
          const score = document.createElement('span');
          score.className = 'rep-score' + (s < 0 ? ' neg' : '');
          score.textContent = '★ ' + (s >= 0 ? '+' : '') + s.toFixed(2);
          row.appendChild(score);
          const ctx = String(e.context || '').startsWith('task:') ? 'automated' : (e.context || 'rating');
          row.appendChild(document.createTextNode(' · ' + ctx + ' · ' + timeAgo(e.received_at) + ' · sig ok'));
        } else {
          const q = document.createElement('span');
          q.className = 'rep-quote';
          q.textContent = '“' + e.body + '”';
          row.appendChild(q);
          row.appendChild(document.createTextNode(' · ' + timeAgo(e.received_at) + ' · sig ok'));
        }
        strip.appendChild(row);
      });
      const note = document.createElement('span');
      note.className = 'rep-row rep-note';
      note.innerHTML = 'scores are weighed by each node — to rate this agent or leave feedback, use the <a href="index.html#desktop">desktop app</a> →';
      strip.appendChild(note);
      strip.hidden = false;
    }).catch(() => {});
  }

  function setInputEnabled(on) {
    chatSend.disabled = !on;
    chatInput.disabled = !on;
  }

  function updateChatTele() {
    if (!activeAgent) return;
    const live = agents.find((x) => x.peer_id === activeAgent.peer_id);
    const tele = document.getElementById('chat-tele');
    if (!live || !live.online) {
      tele.innerHTML = '<span class="full">agent offline</span>';
      if (!streaming) setInputEnabled(false);
      return;
    }
    activeAgent = live;
    if (!streaming) setInputEnabled(true);
    const full = num(live.current_tasks) >= num(live.max_tasks);
    tele.innerHTML =
      'cpu ' + pct(live.cpu_usage_pct) + '% · ram ' + num(live.ram_free_gb).toFixed(1) + 'gb<br>' +
      '<span class="' + (full ? 'full' : 'ok') + '">tasks ' + num(live.current_tasks) + '/' + num(live.max_tasks) + (full ? ' · at capacity' : '') + '</span>';
  }

  function meta(text, isErr) {
    const el = document.createElement('span');
    el.className = 'msg-meta' + (isErr ? ' err' : '');
    el.textContent = '> ' + text;
    chatLog.appendChild(el);
    scrollLog();
  }

  function scrollLog() {
    chatLog.scrollTop = chatLog.scrollHeight;
  }

  function saveTranscript() {
    if (activeAgent) transcripts.set(activeAgent.peer_id, chatLog.innerHTML);
  }

  function goBack() {
    saveTranscript();
    show('agents');
    updateCards();
  }

  async function sendPrompt(prompt) {
    streaming = true;
    setInputEnabled(false);
    aborter = new AbortController();
    let idleTimer = 0;
    const armIdle = () => {
      clearTimeout(idleTimer);
      idleTimer = setTimeout(() => aborter.abort(new Error('no data for 3 minutes')), 180000);
    };

    const user = document.createElement('div');
    user.className = 'msg-user';
    user.textContent = prompt;
    chatLog.appendChild(user);

    const out = document.createElement('div');
    out.className = 'msg-agent streaming';
    chatLog.appendChild(out);
    scrollLog();

    let progressEl = null;

    try {
      armIdle();
      const res = await fetch(API + '/execute', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ worker_id: activeAgent.peer_id, prompt }),
        signal: aborter.signal,
      });
      if (!res.ok) {
        const limited = res.status === 429 || res.status === 503;
        throw new Error(limited ? 'rate limited — give the mesh a minute and try again' : 'gateway replied ' + res.status);
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        armIdle();
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split('\n');
        buf = lines.pop();
        for (const line of lines) handleLine(line);
      }
      buf += decoder.decode();
      if (buf.trim()) handleLine(buf);
    } catch (e) {
      const msg = aborter.signal.aborted
        ? 'stream cancelled' + (aborter.signal.reason && aborter.signal.reason.message ? ' — ' + aborter.signal.reason.message : '')
        : 'stream failed — ' + e.message;
      meta(msg, true);
    }

    clearTimeout(idleTimer);
    out.classList.remove('streaming');
    if (!out.textContent.trim()) out.remove();
    streaming = false;
    aborter = null;
    saveTranscript();
    updateChatTele();
    if (pendingBack) {
      pendingBack = false;
      goBack();
    } else if (!chatInput.disabled) {
      chatInput.focus();
    }

    function handleLine(line) {
      if (line.startsWith('[PROGRESS]:')) {
        const mb = line.slice(11).trim();
        if (!progressEl) {
          progressEl = document.createElement('span');
          progressEl.className = 'artifact-progress';
          chatLog.appendChild(progressEl);
        }
        progressEl.textContent = '⇣ receiving artifact… ' + mb + ' MB';
        scrollLog();
        return;
      }
      if (line.startsWith('[PROGRESS_COMPLETE]')) {
        if (progressEl) progressEl.remove();
        progressEl = null;
        return;
      }
      if (line.startsWith('[DOWNLOAD_READY]:')) {
        if (progressEl) { progressEl.remove(); progressEl = null; }
        const file = line.slice(17).trim();
        if (!/^[A-Za-z0-9._-]+\.zip$/.test(file)) {
          meta('artifact ready but its filename was rejected: ' + file, true);
          return;
        }
        const chip = document.createElement('a');
        chip.className = 'artifact-chip';
        chip.href = API + '/download/' + file;
        chip.setAttribute('download', file);
        chip.innerHTML =
          '<svg viewBox="0 0 24 24" fill="none" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/></svg>' +
          '<span>' + esc(file) + ' · <b>download</b></span>';
        chatLog.appendChild(chip);
        scrollLog();
        return;
      }
      if (line.startsWith('> Task ID:') || line.startsWith('> [SYSTEM')) {
        meta(line.replace(/^> /, ''));
        return;
      }
      if (line.trim() === '') {
        if (out.textContent) out.append('\n');
        return;
      }
      out.append((out.textContent ? '\n' : '') + line);
      scrollLog();
    }
  }

  chatForm.addEventListener('submit', (e) => {
    e.preventDefault();
    const prompt = chatInput.value.trim();
    if (!prompt || streaming || !activeAgent) return;
    chatInput.value = '';
    chatInput.style.height = 'auto';
    sendPrompt(prompt);
  });

  chatInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      chatForm.requestSubmit();
    }
  });

  chatInput.addEventListener('input', () => {
    chatInput.style.height = 'auto';
    chatInput.style.height = Math.min(chatInput.scrollHeight, 130) + 'px';
  });

  document.getElementById('chat-back').addEventListener('click', () => {
    if (streaming && aborter) {
      pendingBack = true;
      aborter.abort(new Error('left the chat'));
      return;
    }
    goBack();
  });

  radarSequence();
})();
