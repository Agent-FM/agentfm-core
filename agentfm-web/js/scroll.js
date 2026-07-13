// AgentFM scroll layer. Progressive enhancement: only runs if GSAP +
// ScrollTrigger loaded and the visitor hasn't asked for reduced motion.
// If GSAP is absent or reduced motion is on, main.js's IntersectionObserver
// still reveals every section, and the dispatch storyboard reads as a plain
// list, so nothing here is load-bearing.
(function () {
  var reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reduce || !window.gsap || !window.ScrollTrigger) return;

  gsap.registerPlugin(ScrollTrigger);

  // --- hero: pinned statement act on desktop, plain parallax on mobile ----
  var hero = document.querySelector('.hero');
  var heroCopy = document.querySelector('.hero-copy');
  var mesh = document.getElementById('mesh');
  var heroMM = gsap.matchMedia();

  heroMM.add('(min-width: 901px)', function () {
    if (!hero || !heroCopy) return;
    hero.classList.add('hero-act');
    var phases = gsap.utils.toArray('.hero-phase');
    var hint = hero.querySelector('.hero-hint');
    var tl = gsap.timeline({
      scrollTrigger: {
        trigger: hero,
        start: 'top top',
        end: '+=1800',
        pin: true,
        scrub: 0.5,
        onUpdate: function (self) {
          if (window.agentfmMesh) window.agentfmMesh.setEnergy(self.progress);
        },
      },
    });
    tl.to(heroCopy, { y: -60, autoAlpha: 0, ease: 'power1.in', duration: 2.2 }, 0);
    if (hint) tl.to(hint, { autoAlpha: 0, duration: 0.6 }, 0);
    if (mesh) tl.fromTo(mesh, { filter: 'blur(0px) brightness(1)' }, { filter: 'blur(7px) brightness(0.55)', ease: 'power1.inOut', duration: 1.8 }, 1.4);
    var veil = hero.querySelector('.hero-veil');
    if (veil) tl.to(veil, { opacity: 1, duration: 1.8 }, 1.4);
    tl.fromTo(phases[0], { opacity: 0, y: 46 }, { opacity: 1, y: 0, ease: 'power2.out', duration: 2 }, 2.6)
      .to(phases[0], { opacity: 0, y: -46, ease: 'power2.in', duration: 1.6 }, 5.8)
      .fromTo(phases[1], { opacity: 0, y: 46 }, { opacity: 1, y: 0, ease: 'power2.out', duration: 2 }, 7.2)
      .to({}, { duration: 1.4 });
    return function () {
      hero.classList.remove('hero-act');
      gsap.set([heroCopy].concat(phases), { clearProps: 'all' });
      if (hint) gsap.set(hint, { clearProps: 'all' });
      if (mesh) gsap.set(mesh, { clearProps: 'filter' });
      if (window.agentfmMesh) window.agentfmMesh.setEnergy(0);
    };
  });

  heroMM.add('(max-width: 900.98px)', function () {
    var tweens = [];
    if (heroCopy) {
      tweens.push(gsap.to(heroCopy, {
        y: 60, opacity: 0.15, ease: 'none',
        scrollTrigger: { trigger: '.hero', start: 'top top', end: 'bottom top', scrub: 0.4 },
      }));
    }
    if (mesh) {
      tweens.push(gsap.to(mesh, {
        y: -80, ease: 'none',
        scrollTrigger: { trigger: '.hero', start: 'top top', end: 'bottom top', scrub: 0.6 },
      }));
    }
    return function () {
      tweens.forEach(function (t) { t.scrollTrigger && t.scrollTrigger.kill(); t.kill(); });
      if (heroCopy) gsap.set(heroCopy, { clearProps: 'all' });
      if (mesh) gsap.set(mesh, { clearProps: 'all' });
    };
  });

  // --- section reveals: hand off from CSS/IO to scrubbed GSAP -------------
  // Take over the .reveal elements so entrances feel scroll-linked.
  // Lanes are excluded here: they get their own staggered rise below, and
  // two tweens on one element's opacity would fight.
  gsap.utils.toArray('.reveal:not(.lane)').forEach(function (el, i) {
    // clear the IO-added class so GSAP owns the animation
    el.classList.remove('in');
    gsap.set(el, { opacity: 0, y: 30 });
    gsap.to(el, {
      opacity: 1, y: 0, duration: 0.8, ease: 'power3.out',
      scrollTrigger: { trigger: el, start: 'top 88%', toggleActions: 'play none none none' },
    });
  });

  // --- role cards: staggered rise ----------------------------------------
  var lanes = gsap.utils.toArray('.lane');
  if (lanes.length) {
    gsap.from(lanes, {
      opacity: 0, y: 40, duration: 0.7, stagger: 0.12, ease: 'power3.out',
      scrollTrigger: { trigger: '.flow', start: 'top 82%' },
    });
  }

  // --- ledger rows: scrub in as the card enters --------------------------
  var rows = gsap.utils.toArray('#ledger .row');
  if (rows.length) {
    gsap.set(rows, { opacity: 0, y: 8 });
    gsap.to(rows, {
      opacity: 1, y: 0, stagger: 0.12, ease: 'none',
      scrollTrigger: { trigger: '#ledger', start: 'top 78%', end: 'top 40%', scrub: 0.5 },
    });
  }

  // --- the pinned dispatch storyboard ------------------------------------
  var scrolly = document.querySelector('.scrolly');
  if (scrolly && scrolly.querySelector('.stage')) {
    var inner = scrolly.querySelector('.scrolly-inner');
    var steps = gsap.utils.toArray('.step');

    var mm = gsap.matchMedia();
    mm.add('(min-width: 901px)', function () {
      scrolly.classList.add('story-on');

      var titles = gsap.utils.toArray('.stage-title');
      var rail = gsap.utils.toArray('.rail-item');
      var packet = scrolly.querySelector('.sc-packet');
      var dots = gsap.utils.toArray('.stream-dot');
      var routeGate = scrolly.querySelector('#route-gate');
      var routeW1 = scrolly.querySelector('#route-w1');
      var routeBack = scrolly.querySelector('#route-back');
      var lenGate = routeGate.getTotalLength();
      var lenW1 = routeW1.getTotalLength();
      var lenBack = routeBack.getTotalLength();
      var setPkX = gsap.quickSetter(packet, 'x', 'px');
      var setPkY = gsap.quickSetter(packet, 'y', 'px');
      var zipEl = scrolly.querySelector('.zip-chip');
      var setZipX = gsap.quickSetter(zipEl, 'x', 'px');
      var setZipY = gsap.quickSetter(zipEl, 'y', 'px');
      var dotSetters = dots.map(function (d) { return gsap.quickSetter(d, 'attr'); });
      var lastIdx = -1;

      function setStage(idx) {
        if (idx === lastIdx) return;
        lastIdx = idx;
        titles.forEach(function (t, i) { t.classList.toggle('on', i === idx); });
        rail.forEach(function (r, i) { r.classList.toggle('on', i <= idx); });
      }

      gsap.set('.gate-chip', { opacity: 0, scale: 0.8, transformOrigin: '50% 50%' });
      gsap.set('.sc-check', { strokeDasharray: 40, strokeDashoffset: 40 });
      gsap.set('.q-chip', { opacity: 0 });
      gsap.set('.sandbox', { opacity: 0 });
      gsap.set('.sandbox-rect', { strokeDasharray: 600, strokeDashoffset: 600 });
      gsap.set(['.sandbox-tag', '.sandbox-line'], { opacity: 0 });
      gsap.set('.term', { opacity: 0, y: 14 });
      gsap.set('.term-line', { opacity: 0 });
      gsap.set('.file-chip', { opacity: 0, scale: 0.6, transformOrigin: '50% 50%' });
      gsap.set('.zip-chip', { opacity: 0 });
      gsap.set('.ledger-mini', { opacity: 0, y: 16 });
      gsap.set('.ledger-row', { opacity: 0, x: -8 });
      gsap.set('.stream-dots', { opacity: 0 });
      gsap.set(['.sc-return', '.sc-packet'], { opacity: 0 });

      var pk1 = { p: 0 }, pk2 = { p: 0 }, stream = { p: 0 }, zip = { p: 0 };
      function placePacket(path, len, p) {
        var pt = path.getPointAtLength(len * p);
        setPkX(pt.x);
        setPkY(pt.y);
      }
      function placeZip() {
        var pt = routeBack.getPointAtLength(lenBack * zip.p);
        setZipX(pt.x);
        setZipY(pt.y);
      }
      function placeDots() {
        dotSetters.forEach(function (set, i) {
          var t = (stream.p * 1.8 + i / 6) % 1;
          var pt = routeBack.getPointAtLength(lenBack * t);
          set({ cx: pt.x, cy: pt.y });
        });
      }

      var tl = gsap.timeline({
        scrollTrigger: {
          trigger: scrolly,
          start: 'top top',
          end: '+=3600',
          pin: inner,
          scrub: 0.6,
          onUpdate: function (self) {
            var p = self.progress;
            setStage(p < 0.30 ? 0 : p < 0.44 ? 1 : p < 0.60 ? 2 : p < 0.82 ? 3 : 4);
          },
        },
      });

      tl.to('.sc-packet', { opacity: 1, duration: 0.15 }, 0)
        .to(pk1, { p: 1, duration: 1.3, ease: 'none', onUpdate: function () { placePacket(routeGate, lenGate, pk1.p); } }, 0)
        .to('.gate-chip', { opacity: 1, scale: 1, duration: 0.5, ease: 'back.out(2)' }, 1.35)
        .to('.sc-check', { strokeDashoffset: 0, duration: 0.4 }, 1.4)
        .to('.q-chip', { opacity: 1, duration: 0.4, stagger: 0.12 }, 1.8)
        .to(pk2, { p: 1, duration: 1.0, ease: 'none', onUpdate: function () { placePacket(routeW1, lenW1, pk2.p); } }, 2.3)
        .to(['.sc-worker[data-worker="1"]', '.sc-worker[data-worker="2"]'], { opacity: 0.3, duration: 0.5 }, 2.6)
        .to('.sc-packet', { opacity: 0, duration: 0.2 }, 3.3)
        .to('.sandbox', { opacity: 1, duration: 0.2 }, 3.15)
        .to('.sandbox-rect', { strokeDashoffset: 0, duration: 1.0, ease: 'none' }, 3.2)
        .to('.sandbox-tag', { opacity: 1, duration: 0.3 }, 3.5)
        .to('.sandbox-line', { opacity: 1, duration: 0.35, stagger: 0.25 }, 4.0)
        .to('.sc-return', { opacity: 1, duration: 0.4 }, 4.45)
        .to('.term', { opacity: 1, y: 0, duration: 0.5 }, 4.5)
        .to('.stream-dots', { opacity: 1, duration: 0.3 }, 4.55)
        .to(stream, { p: 1, duration: 1.7, ease: 'none', onUpdate: placeDots }, 4.55)
        .to('.term-line', { opacity: 1, duration: 0.3, stagger: 0.4 }, 4.75)
        .to('.stream-dots', { opacity: 0, duration: 0.3 }, 6.0)
        .to('.file-chip', { opacity: 1, scale: 1, duration: 0.4, stagger: 0.2, ease: 'back.out(1.8)' }, 6.1)
        .to('.file-chip', { opacity: 0, scale: 0.5, duration: 0.4 }, 6.85)
        .to('.zip-chip', { opacity: 1, duration: 0.35, onStart: placeZip }, 6.95)
        .to(zip, { p: 1, duration: 1.15, ease: 'power1.inOut', onUpdate: placeZip }, 7.3)
        .to('.zip-chip', { opacity: 0, duration: 0.3 }, 8.5)
        .to('.ledger-mini', { opacity: 1, y: 0, duration: 0.5 }, 8.35)
        .to('.ledger-row', { opacity: 1, x: 0, duration: 0.4, stagger: 0.35 }, 8.6)
        .set('.sc-pulse', { opacity: 0.7, scale: 1, transformOrigin: '50% 50%' }, 9.2)
        .to('.sc-pulse', { opacity: 0, scale: 1.9, duration: 1.0 }, 9.2)
        .to({}, { duration: 0.4 });

      setStage(0);

      return function () {
        scrolly.classList.remove('story-on');
        lastIdx = -1;
        titles.forEach(function (t) { t.classList.remove('on'); });
        rail.forEach(function (r) { r.classList.remove('on'); });
      };
    });

    mm.add('(max-width: 900.98px)', function () {
      steps.forEach(function (s) { s.classList.add('active'); });
      return function () {
        steps.forEach(function (s) { s.classList.remove('active'); });
      };
    });
  }

  // Recompute after fonts/images settle so pin distances are correct.
  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(function () { ScrollTrigger.refresh(); });
  }
  window.addEventListener('load', function () { ScrollTrigger.refresh(); });
})();
