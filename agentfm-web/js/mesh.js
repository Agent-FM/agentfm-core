// AgentFM hero mesh — original canvas visualization.
// Labeled agent nodes drift gently; dashed links connect them to the boss;
// packets travel boss->worker (task) and worker->boss (artifact).
(function () {
  const canvas = document.getElementById('mesh');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  const AMBER = '#f0a12e';
  const DIM = 'rgba(255,255,255,0.55)';
  const FAINT = 'rgba(255,255,255,0.16)';

  // Layout in unit space (x,y in 0..1), sized/typed per node.
  const NODES = [
    { id: 'boss',    x: 0.63, y: 0.30, r: 26, label: 'you',           sub: 'desktop · boss', kind: 'boss' },
    { id: 'w1',      x: 0.30, y: 0.22, r: 20, label: 'research bot',  sub: '6 slots · cpu',  kind: 'worker' },
    { id: 'w2',      x: 0.84, y: 0.16, r: 20, label: 'hr bot',        sub: '9 slots · cpu',  kind: 'worker' },
    { id: 'w3',      x: 0.90, y: 0.52, r: 20, label: 'render bot',    sub: '2 slots · gpu',  kind: 'worker' },
    { id: 'w4',      x: 0.44, y: 0.55, r: 20, label: 'code bot',      sub: '4 slots · cpu',  kind: 'worker' },
    { id: 'relay',   x: 0.66, y: 0.68, r: 17, label: 'relay',         sub: 'lighthouse',     kind: 'relay' },
  ];
  const LINKS = [
    ['boss', 'w1'], ['boss', 'w2'], ['boss', 'w3'], ['boss', 'w4'],
    ['boss', 'relay'], ['w3', 'relay'], ['w1', 'relay'],
  ];

  let W = 0, H = 0, dpr = 1, t0 = performance.now();
  let packets = [];
  let raf = 0, running = false;

  function resize() {
    const rect = canvas.getBoundingClientRect();
    dpr = Math.min(window.devicePixelRatio || 1, 2);
    W = rect.width; H = rect.height;
    canvas.width = Math.round(W * dpr);
    canvas.height = Math.round(H * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  function nodePos(n, t) {
    // gentle organic drift, deterministic per node
    const seed = n.id.charCodeAt(0) * 13.7;
    const dx = reduce ? 0 : Math.sin(t / 4200 + seed) * 9;
    const dy = reduce ? 0 : Math.cos(t / 5200 + seed * 1.7) * 7;
    // keep the graph clear of the hero copy: upper 72% on desktop,
    // upper 42% on narrow screens where the copy stacks taller
    const band = W < 640 ? 0.42 : 0.72;
    return { x: n.x * W + dx, y: n.y * H * band + dy };
  }

  function curve(a, b) {
    // control point offset perpendicular to the line for a soft arc
    const mx = (a.x + b.x) / 2, my = (a.y + b.y) / 2;
    const dx = b.x - a.x, dy = b.y - a.y;
    const len = Math.hypot(dx, dy) || 1;
    const off = Math.min(46, len * 0.18);
    return { cx: mx - (dy / len) * off, cy: my + (dx / len) * off };
  }

  function pointOn(a, c, b, p) {
    const q = 1 - p;
    return {
      x: q * q * a.x + 2 * q * p * c.cx + p * p * b.x,
      y: q * q * a.y + 2 * q * p * c.cy + p * p * b.y,
    };
  }

  function spawnPacket() {
    const link = LINKS[(Math.random() * LINKS.length) | 0];
    const fromBoss = Math.random() > 0.45;
    packets.push({
      a: fromBoss ? link[0] : link[1],
      b: fromBoss ? link[1] : link[0],
      p: 0,
      v: 0.0035 + Math.random() * 0.004,
      amber: fromBoss,
    });
    if (packets.length > 7) packets.shift();
  }

  function byId(id) { return NODES.find((n) => n.id === id); }

  function draw(now) {
    const t = now - t0;
    ctx.clearRect(0, 0, W, H);
    const pos = {};
    for (const n of NODES) pos[n.id] = nodePos(n, t);

    // links
    ctx.lineWidth = 1;
    for (const [ai, bi] of LINKS) {
      const a = pos[ai], b = pos[bi];
      const c = curve(a, b);
      ctx.strokeStyle = FAINT;
      ctx.setLineDash([4, 7]);
      ctx.lineDashOffset = reduce ? 0 : -(t / 60) % 11;
      ctx.beginPath();
      ctx.moveTo(a.x, a.y);
      ctx.quadraticCurveTo(c.cx, c.cy, b.x, b.y);
      ctx.stroke();
    }
    ctx.setLineDash([]);

    // packets
    if (!reduce) {
      for (const pk of packets) {
        pk.p += pk.v;
        if (pk.p >= 1) { pk.p = 1; }
        const a = pos[pk.a], b = pos[pk.b];
        const c = curve(a, b);
        const pt = pointOn(a, c, b, pk.p);
        ctx.beginPath();
        ctx.arc(pt.x, pt.y, 2.4, 0, Math.PI * 2);
        ctx.fillStyle = pk.amber ? AMBER : 'rgba(255,255,255,0.75)';
        ctx.shadowColor = pk.amber ? AMBER : 'transparent';
        ctx.shadowBlur = pk.amber ? 10 : 0;
        ctx.fill();
        ctx.shadowBlur = 0;
      }
      packets = packets.filter((pk) => pk.p < 1);
    }

    // nodes
    for (const n of NODES) {
      const p = pos[n.id];
      const isBoss = n.kind === 'boss';
      const pulse = reduce ? 0 : (Math.sin(t / 900 + n.x * 8) + 1) / 2;

      // halo
      const halo = ctx.createRadialGradient(p.x, p.y, 2, p.x, p.y, n.r * (2.1 + pulse * 0.35));
      halo.addColorStop(0, isBoss ? 'rgba(240,161,46,0.28)' : 'rgba(255,255,255,0.10)');
      halo.addColorStop(1, 'transparent');
      ctx.fillStyle = halo;
      ctx.beginPath();
      ctx.arc(p.x, p.y, n.r * 2.6, 0, Math.PI * 2);
      ctx.fill();

      // ring
      ctx.beginPath();
      ctx.arc(p.x, p.y, n.r, 0, Math.PI * 2);
      ctx.fillStyle = '#0e1014';
      ctx.fill();
      ctx.lineWidth = 1.4;
      ctx.strokeStyle = isBoss ? AMBER : (n.kind === 'relay' ? 'rgba(240,161,46,0.5)' : 'rgba(255,255,255,0.38)');
      ctx.stroke();

      // glyph
      ctx.strokeStyle = isBoss ? AMBER : DIM;
      ctx.lineWidth = 1.4;
      ctx.lineCap = 'round';
      const s = n.r * 0.42;
      ctx.beginPath();
      if (n.kind === 'relay') {
        ctx.arc(p.x, p.y, s * 0.45, 0, Math.PI * 2);
        ctx.moveTo(p.x, p.y - s); ctx.lineTo(p.x, p.y - s * 0.55);
        ctx.moveTo(p.x, p.y + s); ctx.lineTo(p.x, p.y + s * 0.55);
        ctx.moveTo(p.x - s, p.y); ctx.lineTo(p.x - s * 0.55, p.y);
        ctx.moveTo(p.x + s, p.y); ctx.lineTo(p.x + s * 0.55, p.y);
      } else if (isBoss) {
        // send glyph
        ctx.moveTo(p.x - s, p.y + s * 0.7);
        ctx.lineTo(p.x + s, p.y - s * 0.7);
        ctx.moveTo(p.x + s, p.y - s * 0.7);
        ctx.lineTo(p.x + s * 0.15, p.y - s * 0.75);
        ctx.moveTo(p.x + s, p.y - s * 0.7);
        ctx.lineTo(p.x + s * 0.9, p.y + s * 0.1);
      } else {
        // box glyph
        ctx.rect(p.x - s, p.y - s * 0.75, s * 2, s * 1.5);
        ctx.moveTo(p.x - s, p.y - s * 0.2);
        ctx.lineTo(p.x + s, p.y - s * 0.2);
      }
      ctx.stroke();

      // labels
      ctx.font = '600 11.5px "JetBrains Mono", ui-monospace, monospace';
      ctx.textAlign = 'center';
      ctx.fillStyle = 'rgba(242,243,245,0.88)';
      ctx.fillText(n.label, p.x, p.y + n.r + 18);
      ctx.font = '400 10px "JetBrains Mono", ui-monospace, monospace';
      ctx.fillStyle = 'rgba(125,133,144,0.85)';
      ctx.fillText(n.sub, p.x, p.y + n.r + 33);
    }
  }

  let spawnTimer = 0;
  function loop(now) {
    if (!running) return;
    if (!reduce && now - spawnTimer > 900) { spawnPacket(); spawnTimer = now; }
    draw(now);
    if (reduce) { running = false; return; } // single static frame
    raf = requestAnimationFrame(loop);
  }

  function start() {
    if (running) return;
    running = true;
    raf = requestAnimationFrame(loop);
  }
  function stop() {
    running = false;
    cancelAnimationFrame(raf);
  }

  resize();
  window.addEventListener('resize', () => { resize(); if (reduce) { running = true; raf = requestAnimationFrame(loop); } });

  // Reduced-motion users get exactly one frame; redraw it once the web
  // fonts land so labels don't stay in the fallback font forever.
  if (reduce && document.fonts && document.fonts.ready) {
    document.fonts.ready.then(() => { running = true; raf = requestAnimationFrame(loop); });
  }

  // Only animate while the hero is on screen; pause in background tabs.
  const io = new IntersectionObserver((entries) => {
    entries.forEach((e) => (e.isIntersecting ? start() : stop()));
  }, { threshold: 0.05 });
  io.observe(canvas);
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) stop();
    else if (canvas.getBoundingClientRect().bottom > 0) start();
  });
})();
