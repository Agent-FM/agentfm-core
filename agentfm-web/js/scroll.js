// AgentFM scroll layer. Progressive enhancement: only runs if GSAP +
// ScrollTrigger loaded and the visitor hasn't asked for reduced motion.
// If GSAP is absent or reduced motion is on, main.js's IntersectionObserver
// still reveals every section, and the dispatch storyboard reads as a plain
// list, so nothing here is load-bearing.
(function () {
  var reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reduce || !window.gsap || !window.ScrollTrigger) return;

  gsap.registerPlugin(ScrollTrigger);

  // --- hero: gentle parallax + fade as you leave it -----------------------
  var heroCopy = document.querySelector('.hero-copy');
  var mesh = document.getElementById('mesh');
  if (heroCopy) {
    gsap.to(heroCopy, {
      y: 60, opacity: 0.15, ease: 'none',
      scrollTrigger: { trigger: '.hero', start: 'top top', end: 'bottom top', scrub: 0.4 },
    });
  }
  if (mesh) {
    gsap.to(mesh, {
      y: -80, ease: 'none',
      scrollTrigger: { trigger: '.hero', start: 'top top', end: 'bottom top', scrub: 0.6 },
    });
  }

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
  if (scrolly) {
    var inner = scrolly.querySelector('.scrolly-inner');
    var steps = gsap.utils.toArray('.step');
    var stops = gsap.utils.toArray('.stop');
    var packet = scrolly.querySelector('.packet');
    var fill = scrolly.querySelector('.pipe-fill');
    var yTop = 40, yBot = 480, lastIdx = -1;
    // Move the packet with a transform (composited) rather than rewriting the
    // cy attribute (which re-rasterizes its drop-shadow every frame).
    var movePacket = packet ? gsap.quickSetter(packet, 'y', 'px') : function () {};

    function setActive(idx) {
      if (idx === lastIdx) return;            // class churn only when it changes
      lastIdx = idx;
      steps.forEach(function (s, i) { s.classList.toggle('active', i === idx); });
      stops.forEach(function (s, i) { s.classList.toggle('active', i <= idx); });
    }

    // matchMedia creates the pin above 760px and tears it down below, so a
    // resize across the breakpoint never leaves a stale pin or dead scroll.
    var mm = gsap.matchMedia();
    mm.add('(min-width: 761px)', function () {
      lastIdx = -1;
      setActive(0);
      var tl = gsap.timeline({
        scrollTrigger: {
          trigger: scrolly,
          start: 'top top',
          end: '+=' + (steps.length * 320),
          pin: inner,
          scrub: 0.6,
          onUpdate: function (self) {
            var p = self.progress;             // 0..1 across the pin
            var y = yTop + (yBot - yTop) * p;
            movePacket(y - yTop);
            if (fill) fill.setAttribute('y2', y);
            setActive(Math.min(steps.length - 1, Math.floor(p * steps.length + 0.15)));
          },
        },
      });
      tl.to({}, { duration: 1 });             // duration so scrub can interpolate
      return function () {                     // cleanup when leaving desktop
        lastIdx = -1;
        movePacket(0);
        if (fill) fill.setAttribute('y2', yTop);
      };
    });
    mm.add('(max-width: 760px)', function () {
      steps.forEach(function (s) { s.classList.add('active'); });
      stops.forEach(function (s) { s.classList.add('active'); });
      return function () {
        steps.forEach(function (s) { s.classList.remove('active'); });
        stops.forEach(function (s) { s.classList.remove('active'); });
      };
    });
  }

  // Recompute after fonts/images settle so pin distances are correct.
  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(function () { ScrollTrigger.refresh(); });
  }
  window.addEventListener('load', function () { ScrollTrigger.refresh(); });
})();
