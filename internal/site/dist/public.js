var Zn = Array.isArray, Qr = Array.prototype.indexOf, Wt = Array.prototype.includes, en = Array.from, $r = Object.defineProperty, Mt = Object.getOwnPropertyDescriptor, ei = Object.getOwnPropertyDescriptors, ti = Object.prototype, ni = Array.prototype, Jn = Object.getPrototypeOf, zn = Object.isExtensible;
const ri = () => {
};
function ii(e) {
  for (var t = 0; t < e.length; t++)
    e[t]();
}
function Qn() {
  var e, t, n = new Promise((r, i) => {
    e = r, t = i;
  });
  return { promise: n, resolve: e, reject: t };
}
const ie = 2, Nt = 4, tn = 8, $n = 1 << 24, Ee = 16, we = 32, ze = 64, dn = 128, me = 512, ne = 1024, re = 2048, Se = 4096, oe = 8192, be = 16384, bt = 32768, Bn = 1 << 25, tt = 65536, Kt = 1 << 17, ai = 1 << 18, wt = 1 << 19, si = 1 << 20, Ae = 1 << 25, nt = 65536, Xt = 1 << 21, vt = 1 << 22, je = 1 << 23, ln = Symbol("$state"), li = Symbol(""), er = Symbol("attributes"), vn = Symbol("class"), oi = Symbol("style"), hn = Symbol("text"), Ht = Symbol("form reset"), Ft = new class extends Error {
  name = "StaleReactionError";
  message = "The reaction that called `getAbortSignal()` was re-run or destroyed";
}();
function fi(e) {
  throw new Error("https://svelte.dev/e/lifecycle_outside_component");
}
function ui() {
  throw new Error("https://svelte.dev/e/async_derived_orphan");
}
function ci(e, t, n) {
  throw new Error("https://svelte.dev/e/each_key_duplicate");
}
function di(e) {
  throw new Error("https://svelte.dev/e/effect_in_teardown");
}
function vi() {
  throw new Error("https://svelte.dev/e/effect_in_unowned_derived");
}
function hi(e) {
  throw new Error("https://svelte.dev/e/effect_orphan");
}
function _i() {
  throw new Error("https://svelte.dev/e/effect_update_depth_exceeded");
}
function pi() {
  throw new Error("https://svelte.dev/e/state_descriptors_fixed");
}
function gi() {
  throw new Error("https://svelte.dev/e/state_prototype_fixed");
}
function mi() {
  throw new Error("https://svelte.dev/e/state_unsafe_mutation");
}
function bi() {
  throw new Error("https://svelte.dev/e/svelte_boundary_reset_onerror");
}
const wi = 1, yi = 2, tr = 4, Ei = 8, ki = 16, xi = 1, Si = 2, te = Symbol("uninitialized"), Ti = "http://www.w3.org/1999/xhtml";
function Ci() {
  console.warn("https://svelte.dev/e/derived_inert");
}
function Ai() {
  console.warn("https://svelte.dev/e/svelte_boundary_reset_noop");
}
function nr(e) {
  return e === this.v;
}
function Mi(e, t) {
  return e != e ? t == t : e !== t || e !== null && typeof e == "object" || typeof e == "function";
}
function rr(e) {
  return !Mi(e, this.v);
}
let fe = null;
function ht(e) {
  fe = e;
}
function at(e, t = !1, n) {
  fe = {
    p: fe,
    i: !1,
    c: null,
    e: null,
    s: e,
    x: null,
    r: (
      /** @type {Effect} */
      F
    ),
    l: null
  };
}
function st(e) {
  var t = (
    /** @type {ComponentContext} */
    fe
  ), n = t.e;
  if (n !== null) {
    t.e = null;
    for (var r of n)
      Er(r);
  }
  return t.i = !0, fe = t.p, /** @type {T} */
  {};
}
function ir() {
  return !0;
}
let Ze = [];
function ar() {
  var e = Ze;
  Ze = [], ii(e);
}
function Oe(e) {
  if (Ze.length === 0 && !Rt) {
    var t = Ze;
    queueMicrotask(() => {
      t === Ze && ar();
    });
  }
  Ze.push(e);
}
function Ri() {
  for (; Ze.length > 0; )
    ar();
}
function sr(e) {
  var t = F;
  if (t === null)
    return O.f |= je, e;
  if ((t.f & bt) === 0 && (t.f & Nt) === 0)
    throw e;
  He(e, t);
}
function He(e, t) {
  if (!(t !== null && (t.f & be) !== 0)) {
    for (; t !== null; ) {
      if ((t.f & dn) !== 0) {
        if ((t.f & bt) === 0)
          throw e;
        try {
          t.b.error(e);
          return;
        } catch (n) {
          e = n;
        }
      }
      t = t.parent;
    }
    throw e;
  }
}
const Ii = -7169;
function Z(e, t) {
  e.f = e.f & Ii | t;
}
function kn(e) {
  (e.f & me) !== 0 || e.deps === null ? Z(e, ne) : Z(e, Se);
}
function lr(e) {
  if (e !== null)
    for (const t of e)
      (t.f & ie) === 0 || (t.f & nt) === 0 || (t.f ^= nt, lr(
        /** @type {Derived} */
        t.deps
      ));
}
function or(e, t, n) {
  (e.f & re) !== 0 ? t.add(e) : (e.f & Se) !== 0 && n.add(e), lr(e.deps), Z(e, ne);
}
let Un = !1;
function Li() {
  Un || (Un = !0, document.addEventListener(
    "reset",
    (e) => {
      Promise.resolve().then(() => {
        if (!e.defaultPrevented)
          for (
            const t of
            /**@type {HTMLFormElement} */
            e.target.elements
          )
            t[Ht]?.();
      });
    },
    // In the capture phase to guarantee we get noticed of it (no possibility of stopPropagation)
    { capture: !0 }
  ));
}
function yt(e) {
  var t = O, n = F;
  ye(null), Ie(null);
  try {
    return e();
  } finally {
    ye(t), Ie(n);
  }
}
function Ni(e, t, n, r = n) {
  e.addEventListener(t, () => yt(n));
  const i = (
    /** @type {any} */
    e[Ht]
  );
  i ? e[Ht] = () => {
    i(), r(!0);
  } : e[Ht] = () => r(!0), Li();
}
function Pi(e) {
  let t = 0, n = rt(0), r;
  return () => {
    An() && (s(n), kr(() => (t === 0 && (r = In(() => e(() => It(n)))), t += 1, () => {
      Oe(() => {
        t -= 1, t === 0 && (r?.(), r = void 0, It(n));
      });
    })));
  };
}
var Oi = tt | wt;
function Fi(e, t, n, r) {
  new Di(e, t, n, r);
}
class Di {
  /** @type {Boundary | null} */
  parent;
  is_pending = !1;
  /**
   * API-level transformError transform function. Transforms errors before they reach the `failed` snippet.
   * Inherited from parent boundary, or defaults to identity.
   * @type {(error: unknown) => unknown}
   */
  transform_error;
  /** @type {TemplateNode} */
  #t;
  /** @type {TemplateNode | null} */
  #s = null;
  /** @type {BoundaryProps} */
  #e;
  /** @type {((anchor: Node) => void)} */
  #o;
  /** @type {Effect} */
  #r;
  /** @type {Effect | null} */
  #a = null;
  /** @type {Effect | null} */
  #n = null;
  /** @type {Effect | null} */
  #l = null;
  /** @type {DocumentFragment | null} */
  #i = null;
  #_ = 0;
  #f = 0;
  #u = !1;
  /** @type {Set<Effect>} */
  #d = /* @__PURE__ */ new Set();
  /** @type {Set<Effect>} */
  #p = /* @__PURE__ */ new Set();
  /**
   * A source containing the number of pending async deriveds/expressions.
   * Only created if `$effect.pending()` is used inside the boundary,
   * otherwise updating the source results in needless `Batch.ensure()`
   * calls followed by no-op flushes
   * @type {Source<number> | null}
   */
  #c = null;
  #m = Pi(() => (this.#c = rt(this.#_), () => {
    this.#c = null;
  }));
  /**
   * @param {TemplateNode} node
   * @param {BoundaryProps} props
   * @param {((anchor: Node) => void)} children
   * @param {((error: unknown) => unknown) | undefined} [transform_error]
   */
  constructor(t, n, r, i) {
    this.#t = t, this.#e = n, this.#o = (a) => {
      var l = (
        /** @type {Effect} */
        F
      );
      l.b = this, l.f |= dn, r(a);
    }, this.parent = /** @type {Effect} */
    F.b, this.transform_error = i ?? this.parent?.transform_error ?? ((a) => a), this.#r = nn(() => {
      this.#v();
    }, Oi);
  }
  #g() {
    try {
      this.#a = pe(() => this.#o(this.#t));
    } catch (t) {
      this.error(t);
    }
  }
  /**
   * @param {unknown} error The deserialized error from the server's hydration comment
   */
  #y(t) {
    const n = this.#e.failed, { reset: r, invoke_onerror: i } = this.#b(t);
    Oe(i), n && (this.#l = pe(() => {
      n(
        this.#t,
        () => t,
        () => r
      );
    }));
  }
  /**
   * Creates the `reset` function for a failed boundary, along with a function
   * that invokes `onerror` with it (if provided)
   * @param {unknown} error
   * @returns {{ reset: () => void, invoke_onerror: () => void }}
   */
  #b(t) {
    var n = !1, r = !1;
    const i = () => {
      if (n) {
        Ai();
        return;
      }
      n = !0, r && bi(), this.#l !== null && $e(this.#l, () => {
        this.#l = null;
      }), this.#h(() => {
        this.#v();
      });
    };
    return { reset: i, invoke_onerror: () => {
      try {
        r = !0, this.#e.onerror?.(t, i), r = !1;
      } catch (l) {
        He(l, this.#r && this.#r.parent);
      }
    } };
  }
  #E() {
    const t = this.#e.pending;
    t && (this.is_pending = !0, this.#n = pe(() => t(this.#t)), Oe(() => {
      var n = this.#i = document.createDocumentFragment(), r = De();
      n.append(r), this.#a = this.#h(() => pe(() => this.#o(r))), this.#f === 0 && (this.#t.before(n), this.#i = null, $e(
        /** @type {Effect} */
        this.#n,
        () => {
          this.#n = null;
        }
      ), this.#w(
        /** @type {Batch} */
        N
      ));
    }));
  }
  #v() {
    try {
      if (this.is_pending = this.has_pending_snippet(), this.#f = 0, this.#_ = 0, this.#a = pe(() => {
        this.#o(this.#t);
      }), this.#f > 0) {
        var t = this.#i = document.createDocumentFragment();
        Rn(this.#a, t);
        const n = (
          /** @type {(anchor: Node) => void} */
          this.#e.pending
        );
        this.#n = pe(() => n(this.#t));
      } else
        this.#w(
          /** @type {Batch} */
          N
        );
    } catch (n) {
      this.error(n);
    }
  }
  /**
   * @param {Batch} batch
   */
  #w(t) {
    this.is_pending = !1, t.transfer_effects(this.#d, this.#p);
  }
  /**
   * Defer an effect inside a pending boundary until the boundary resolves
   * @param {Effect} effect
   */
  defer_effect(t) {
    or(t, this.#d, this.#p);
  }
  /**
   * Returns `false` if the effect exists inside a boundary whose pending snippet is shown
   * @returns {boolean}
   */
  is_rendered() {
    return !this.is_pending && (!this.parent || this.parent.is_rendered());
  }
  has_pending_snippet() {
    return !!this.#e.pending;
  }
  /**
   * @template T
   * @param {() => T} fn
   */
  #h(t) {
    var n = F, r = O, i = fe;
    Ie(this.#r), ye(this.#r), ht(this.#r.ctx);
    try {
      return Ge.ensure(), t();
    } catch (a) {
      return sr(a), null;
    } finally {
      Ie(n), ye(r), ht(i);
    }
  }
  /**
   * Updates the pending count associated with the currently visible pending snippet,
   * if any, such that we can replace the snippet with content once work is done
   * @param {1 | -1} d
   * @param {Batch} batch
   */
  #k(t, n) {
    if (!this.has_pending_snippet()) {
      this.parent && this.parent.#k(t, n);
      return;
    }
    this.#f += t, this.#f === 0 && (this.#w(n), this.#n && $e(this.#n, () => {
      this.#n = null;
    }), this.#i && (this.#t.before(this.#i), this.#i = null));
  }
  /**
   * Update the source that powers `$effect.pending()` inside this boundary,
   * and controls when the current `pending` snippet (if any) is removed.
   * Do not call from inside the class
   * @param {1 | -1} d
   * @param {Batch} batch
   */
  update_pending_count(t, n) {
    this.#k(t, n), this.#_ += t, !(!this.#c || this.#u) && (this.#u = !0, Oe(() => {
      this.#u = !1, this.#c && _t(this.#c, this.#_);
    }));
  }
  get_effect_pending() {
    return this.#m(), s(
      /** @type {Source<number>} */
      this.#c
    );
  }
  /** @param {unknown} error */
  error(t) {
    if (!this.#e.onerror && !this.#e.failed)
      throw t;
    N?.is_fork ? (this.#a && N.skip_effect(this.#a), this.#n && N.skip_effect(this.#n), this.#l && N.skip_effect(this.#l), N.oncommit(() => {
      this.#x(t);
    })) : this.#x(t);
  }
  /**
   * @param {unknown} error
   */
  #x(t) {
    this.#a && (ce(this.#a), this.#a = null), this.#n && (ce(this.#n), this.#n = null), this.#l && (ce(this.#l), this.#l = null);
    let n = this.#e.failed;
    const r = (i) => {
      const { reset: a, invoke_onerror: l } = this.#b(i);
      l(), n && (this.#l = this.#h(() => {
        try {
          return pe(() => {
            var f = (
              /** @type {Effect} */
              F
            );
            f.b = this, f.f |= dn, n(
              this.#t,
              () => i,
              () => a
            );
          });
        } catch (f) {
          return He(
            f,
            /** @type {Effect} */
            this.#r.parent
          ), null;
        }
      }));
    };
    Oe(() => {
      var i;
      try {
        i = this.transform_error(t);
      } catch (a) {
        He(a, this.#r && this.#r.parent);
        return;
      }
      i !== null && typeof i == "object" && typeof /** @type {any} */
      i.then == "function" ? i.then(
        r,
        /** @param {unknown} e */
        (a) => He(a, this.#r && this.#r.parent)
      ) : r(i);
    });
  }
}
function zi(e, t, n, r) {
  const i = xn;
  var a = e.filter((c) => !c.settled), l = t.map(i);
  if (n.length === 0 && a.length === 0) {
    r(l);
    return;
  }
  var f = (
    /** @type {Effect} */
    F
  ), o = Bi(), u = a.length === 1 ? a[0].promise : a.length > 1 ? Promise.all(a.map((c) => c.promise)) : null;
  function d(c) {
    if ((f.f & be) === 0) {
      o();
      try {
        r([...l, ...c]);
      } catch (v) {
        He(v, f);
      }
      Zt();
    }
  }
  var _ = fr();
  if (n.length === 0) {
    u.then(() => d([])).finally(_);
    return;
  }
  function h() {
    Promise.all(n.map((c) => /* @__PURE__ */ Ui(c))).then(d).catch((c) => He(c, f)).finally(_);
  }
  u ? u.then(() => {
    o(), h(), Zt();
  }) : h();
}
function Bi() {
  var e = (
    /** @type {Effect} */
    F
  ), t = O, n = fe, r = (
    /** @type {Batch} */
    N
  );
  return function(a = !0) {
    Ie(e), ye(t), ht(n), a && (e.f & be) === 0 && (r?.activate(), r?.apply());
  };
}
function Zt(e = !0) {
  Ie(null), ye(null), ht(null), e && N?.deactivate();
}
function fr() {
  var e = (
    /** @type {Effect} */
    F
  ), t = e.b, n = (
    /** @type {Batch} */
    N
  ), r = !!t?.is_rendered();
  return t?.update_pending_count(1, n), n.increment(r, e), () => {
    t?.update_pending_count(-1, n), n.decrement(r, e);
  };
}
// @__NO_SIDE_EFFECTS__
function xn(e) {
  var t = ie | re;
  return F !== null && (F.f |= wt), {
    ctx: fe,
    deps: null,
    effects: null,
    equals: nr,
    f: t,
    fn: e,
    reactions: null,
    rv: 0,
    v: (
      /** @type {V} */
      te
    ),
    wv: 0,
    parent: F,
    ac: null
  };
}
const Tt = Symbol("obsolete");
// @__NO_SIDE_EFFECTS__
function Ui(e, t, n) {
  let r = (
    /** @type {Effect | null} */
    F
  );
  r === null && ui();
  var i = (
    /** @type {Promise<V>} */
    /** @type {unknown} */
    void 0
  ), a = rt(
    /** @type {V} */
    te
  ), l = !O, f = /* @__PURE__ */ new Set();
  return ta(() => {
    var o = (
      /** @type {Effect} */
      F
    ), u = Qn();
    i = u.promise;
    try {
      Promise.resolve(e()).then(u.resolve, (c) => {
        c !== Ft && u.reject(c);
      }).finally(Zt);
    } catch (c) {
      u.reject(c), Zt();
    }
    var d = (
      /** @type {Batch} */
      N
    );
    if (l) {
      if ((o.f & bt) !== 0)
        var _ = fr();
      if (
        // boundary can be null if the async derived is inside an $effect.root not connected to the component render tree
        r.b?.is_rendered()
      )
        d.async_deriveds.get(o)?.reject(Tt);
      else
        for (const c of f.values())
          c.reject(Tt);
      f.add(u), d.async_deriveds.set(o, u);
    }
    const h = (c, v = void 0) => {
      _?.(), f.delete(u), v !== Tt && (d.activate(), v ? (a.f |= je, _t(a, v)) : ((a.f & je) !== 0 && (a.f ^= je), _t(a, c)), d.deactivate());
    };
    u.promise.then(h, (c) => h(null, c || "unknown"));
  }), yr(() => {
    for (const o of f)
      o.reject(Tt);
  }), new Promise((o) => {
    function u(d) {
      function _() {
        d === i ? o(a) : u(i);
      }
      d.then(_, _);
    }
    u(i);
  });
}
// @__NO_SIDE_EFFECTS__
function X(e) {
  const t = /* @__PURE__ */ xn(e);
  return Ar(t), t;
}
// @__NO_SIDE_EFFECTS__
function Hi(e) {
  const t = /* @__PURE__ */ xn(e);
  return t.equals = rr, t;
}
function ji(e) {
  var t = e.effects;
  if (t !== null) {
    e.effects = null;
    for (var n = 0; n < t.length; n += 1)
      ce(
        /** @type {Effect} */
        t[n]
      );
  }
}
function Sn(e) {
  var t, n = F, r = e.parent;
  if (!Ye && r !== null && e.v !== te && // if it was never evaluated before, it's guaranteed to fail downstream, so we try to execute instead
  (r.f & (be | oe)) !== 0)
    return Ci(), e.v;
  Ie(r);
  try {
    e.f &= ~nt, ji(e), t = Lr(e);
  } finally {
    Ie(n);
  }
  return t;
}
function ur(e) {
  var t = Sn(e);
  if (!e.equals(t) && (e.wv = Rr(), (!N?.is_fork || e.deps === null) && (N !== null ? (N.capture(e, t, !0), _n?.capture(e, t, !0)) : e.v = t, e.deps === null))) {
    Z(e, ne);
    return;
  }
  Ye || (ke !== null ? (An() || N?.is_fork) && ke.set(e, t) : kn(e));
}
function qi(e) {
  if (e.effects !== null)
    for (const t of e.effects)
      (t.teardown || t.ac) && (t.teardown?.(), t.ac !== null && yt(() => {
        t.ac.abort(Ft), t.ac = null;
      }), t.fn !== null && (t.teardown = ri), Pt(t, 0), Mn(t));
}
function cr(e) {
  if (e.effects !== null)
    for (const t of e.effects)
      t.teardown && t.fn !== null && gt(t);
}
let on = null, ut = null, N = null, _n = null, ke = null, pn = null, Rt = !1, fn = !1, dt = null, jt = null;
var Hn = 0;
let Gi = 1;
class Ge {
  id = Gi++;
  /** True as soon as `#process` was called */
  #t = !1;
  linked = !0;
  /** @type {Batch | null} */
  #s = null;
  /** @type {Batch | null} */
  #e = null;
  /** @type {Map<Effect, ReturnType<typeof deferred<any>>>} */
  async_deriveds = /* @__PURE__ */ new Map();
  /**
   * The current values of any signals that are updated in this batch.
   * Tuple format: [value, is_derived] (note: is_derived is false for deriveds, too, if they were overridden via assignment)
   * They keys of this map are identical to `this.#previous`
   * @type {Map<Value, [any, boolean]>}
   */
  current = /* @__PURE__ */ new Map();
  /**
   * The values of any signals (sources and deriveds) that are updated in this batch _before_ those updates took place.
   * They keys of this map are identical to `this.#current`
   * @type {Map<Value, any>}
   */
  previous = /* @__PURE__ */ new Map();
  /**
   * When the batch is committed (and the DOM is updated), we need to remove old branches
   * and append new ones by calling the functions added inside (if/each/key/etc) blocks
   * @type {Set<(batch: Batch) => void>}
   */
  #o = /* @__PURE__ */ new Set();
  /**
   * If a fork is discarded, we need to destroy any effects that are no longer needed
   * @type {Set<(batch: Batch) => void>}
   */
  #r = /* @__PURE__ */ new Set();
  /**
   * The number of async effects that are currently in flight
   */
  #a = 0;
  /**
   * Async effects that are currently in flight, _not_ inside a pending boundary
   * @type {Map<Effect, number>}
   */
  #n = /* @__PURE__ */ new Map();
  /**
   * A deferred that resolves when the batch is committed, used with `settled()`
   * TODO replace with Promise.withResolvers once supported widely enough
   * @type {{ promise: Promise<void>, resolve: (value?: any) => void, reject: (reason: unknown) => void } | null}
   */
  #l = null;
  /**
   * The root effects that need to be flushed
   * @type {Effect[]}
   */
  #i = [];
  /**
   * Effects created while this batch was active.
   * @type {Effect[]}
   */
  #_ = [];
  /**
   * Deferred effects (which run after async work has completed) that are DIRTY
   * @type {Set<Effect>}
   */
  #f = /* @__PURE__ */ new Set();
  /**
   * Deferred effects that are MAYBE_DIRTY
   * @type {Set<Effect>}
   */
  #u = /* @__PURE__ */ new Set();
  /**
   * A map of branches that still exist, but will be destroyed when this batch
   * is committed — we skip over these during `process`.
   * The value contains child effects that were dirty/maybe_dirty before being reset,
   * so they can be rescheduled if the branch survives.
   * @type {Map<Effect, { d: Effect[], m: Effect[] }>}
   */
  #d = /* @__PURE__ */ new Map();
  /**
   * Inverse of #skipped_branches which we need to tell prior batches to unskip them when committing
   * @type {Set<Effect>}
   */
  #p = /* @__PURE__ */ new Set();
  is_fork = !1;
  #c = !1;
  constructor() {
    ut === null ? on = ut = this : (ut.#e = this, this.#s = ut), ut = this;
  }
  #m() {
    if (this.is_fork) return !0;
    for (const r of this.#n.keys()) {
      for (var t = r, n = !1; t.parent !== null; ) {
        if (this.#d.has(t)) {
          n = !0;
          break;
        }
        t = t.parent;
      }
      if (!n)
        return !0;
    }
    return !1;
  }
  /**
   * Add an effect to the #skipped_branches map and reset its children
   * @param {Effect} effect
   */
  skip_effect(t) {
    this.#d.has(t) || this.#d.set(t, { d: [], m: [] }), this.#p.delete(t);
  }
  /**
   * Remove an effect from the #skipped_branches map and reschedule
   * any tracked dirty/maybe_dirty child effects
   * @param {Effect} effect
   * @param {(e: Effect) => void} callback
   */
  unskip_effect(t, n = (r) => this.schedule(r)) {
    var r = this.#d.get(t);
    if (r) {
      this.#d.delete(t);
      for (var i of r.d)
        Z(i, re), n(i);
      for (i of r.m)
        Z(i, Se), n(i);
    }
    this.#p.add(t);
  }
  #g() {
    this.#t = !0, Hn++ > 1e3 && (this.#h(), Vi());
    for (const o of this.#f)
      this.#u.delete(o), Z(o, re), this.schedule(o);
    for (const o of this.#u)
      Z(o, Se), this.schedule(o);
    const t = this.#i;
    this.#i = [], this.apply();
    var n = dt = [], r = [], i = jt = [];
    for (const o of t)
      try {
        this.#y(o, n, r);
      } catch (u) {
        throw hr(o), this.#m() || this.discard(), u;
      }
    if (N = null, i.length > 0) {
      var a = Ge.ensure();
      for (const o of i)
        a.schedule(o);
    }
    if (dt = null, jt = null, this.#m()) {
      this.#v(r), this.#v(n);
      for (const [o, u] of this.#d)
        vr(o, u);
      i.length > 0 && /** @type {unknown} */
      N.#g();
      return;
    }
    const l = this.#b();
    if (l) {
      this.#v(r), this.#v(n), l.#E(this);
      return;
    }
    this.#f.clear(), this.#u.clear();
    for (const o of this.#o) o(this);
    this.#o.clear(), _n = this, jn(r), jn(n), _n = null, this.#l?.resolve();
    var f = (
      /** @type {Batch | null} */
      /** @type {unknown} */
      N
    );
    if (this.#a === 0 && (this.#i.length === 0 || f !== null) && this.#h(), this.#i.length > 0)
      if (f !== null) {
        const o = f;
        o.#i.push(...this.#i.filter((u) => !o.#i.includes(u)));
      } else
        f = this;
    f !== null && f.#g();
  }
  /**
   * Traverse the effect tree, executing effects or stashing
   * them for later execution as appropriate
   * @param {Effect} root
   * @param {Effect[]} effects
   * @param {Effect[]} render_effects
   */
  #y(t, n, r) {
    t.f ^= ne;
    for (var i = t.first; i !== null; ) {
      var a = i.f, l = (a & (we | ze)) !== 0, f = l && (a & ne) !== 0, o = f || (a & oe) !== 0 || this.#d.has(i);
      if (!o && i.fn !== null) {
        l ? i.f ^= ne : (a & Nt) !== 0 ? n.push(i) : zt(i) && ((a & Ee) !== 0 && this.#u.add(i), gt(i));
        var u = i.first;
        if (u !== null) {
          i = u;
          continue;
        }
      }
      for (; i !== null; ) {
        var d = i.next;
        if (d !== null) {
          i = d;
          break;
        }
        i = i.parent;
      }
    }
  }
  #b() {
    for (var t = this.#s; t !== null; ) {
      if (!t.is_fork) {
        for (const [n, [, r]] of this.current)
          if (t.current.has(n) && !r)
            return t;
      }
      t = t.#s;
    }
    return null;
  }
  /**
   * @param {Batch} batch
   */
  #E(t) {
    for (const [r, i] of t.current)
      !this.previous.has(r) && t.previous.has(r) && this.previous.set(r, t.previous.get(r)), this.current.set(r, i);
    for (const [r, i] of t.async_deriveds) {
      const a = this.async_deriveds.get(r);
      a && i.promise.then(a.resolve).catch(a.reject);
    }
    t.async_deriveds.clear(), this.transfer_effects(t.#f, t.#u);
    const n = (r) => {
      var i = r.reactions;
      if (i !== null && !((r.f & ie) !== 0 && (r.f & (re | Se)) === 0))
        for (const f of i) {
          var a = f.f;
          if ((a & ie) !== 0)
            n(
              /** @type {Derived} */
              f
            );
          else {
            var l = (
              /** @type {Effect} */
              f
            );
            a & (vt | Ee) && !this.async_deriveds.has(l) && (this.#u.delete(l), Z(l, re), this.schedule(l));
          }
        }
    };
    for (const r of this.current.keys())
      n(r);
    this.oncommit(() => t.discard()), t.#h(), N = this, this.#g();
  }
  /**
   * @param {Effect[]} effects
   */
  #v(t) {
    for (var n = 0; n < t.length; n += 1)
      or(t[n], this.#f, this.#u);
  }
  /**
   * Associate a change to a given source with the current
   * batch, noting its previous and current values
   * @param {Value} source
   * @param {any} value
   * @param {boolean} [is_derived]
   */
  capture(t, n, r = !1) {
    t.v !== te && !this.previous.has(t) && this.previous.set(t, t.v), (t.f & je) === 0 && (this.current.set(t, [n, r]), ke?.set(t, n)), this.is_fork || (t.v = n);
  }
  activate() {
    N = this;
  }
  deactivate() {
    N = null, ke = null;
  }
  flush() {
    try {
      fn = !0, N = this, this.#g();
    } finally {
      Hn = 0, pn = null, dt = null, jt = null, fn = !1, N = null, ke = null, Qe.clear();
    }
  }
  discard() {
    for (const t of this.#r) t(this);
    this.#r.clear();
    for (const t of this.async_deriveds.values())
      t.reject(Tt);
    this.#h(), this.#l?.resolve();
  }
  /**
   * @param {Effect} effect
   */
  register_created_effect(t) {
    this.#_.push(t);
  }
  #w() {
    for (let _ = on; _ !== null; _ = _.#e) {
      var t = _.id < this.id, n = [];
      for (const [h, [c, v]] of this.current) {
        if (_.current.has(h)) {
          var r = (
            /** @type {[any, boolean]} */
            _.current.get(h)[0]
          );
          if (t && c !== r)
            _.current.set(h, [c, v]);
          else
            continue;
        }
        n.push(h);
      }
      if (t)
        for (const [h, c] of this.async_deriveds) {
          const v = _.async_deriveds.get(h);
          v && c.promise.then(v.resolve).catch(v.reject);
        }
      var i = [..._.current.keys()].filter(
        (h) => !/** @type {[any, boolean]} */
        _.current.get(h)[1]
      );
      if (!(!_.#t || i.length === 0)) {
        var a = i.filter((h) => !this.current.has(h));
        if (a.length === 0)
          t && _.discard();
        else if (n.length > 0) {
          if (t)
            for (const h of this.#p)
              _.unskip_effect(h, (c) => {
                (c.f & (Ee | vt)) !== 0 ? _.schedule(c) : _.#v([c]);
              });
          _.activate();
          var l = /* @__PURE__ */ new Set(), f = /* @__PURE__ */ new Map();
          for (var o of n)
            dr(o, a, l, f);
          f = /* @__PURE__ */ new Map();
          var u = [..._.current].filter(([h, c]) => {
            const v = this.current.get(h);
            return v ? v[0] !== c[0] || v[1] !== c[1] : !0;
          }).map(([h]) => h);
          if (u.length > 0)
            for (const h of this.#_)
              (h.f & (be | oe | Kt)) === 0 && Tn(h, u, f) && ((h.f & (vt | Ee)) !== 0 ? (Z(h, re), _.schedule(h)) : _.#f.add(h));
          if (_.#i.length > 0 && !_.#c) {
            _.apply();
            for (var d of _.#i)
              _.#y(d, [], []);
            _.#i = [];
          }
          _.deactivate();
        }
      }
    }
  }
  /**
   * @param {boolean} blocking
   * @param {Effect} effect
   */
  increment(t, n) {
    if (this.#a += 1, t) {
      let r = this.#n.get(n) ?? 0;
      this.#n.set(n, r + 1);
    }
  }
  /**
   * @param {boolean} blocking
   * @param {Effect} effect
   */
  decrement(t, n) {
    if (this.#a -= 1, t) {
      let r = this.#n.get(n) ?? 0;
      r === 1 ? this.#n.delete(n) : this.#n.set(n, r - 1);
    }
    this.#c || (this.#c = !0, Oe(() => {
      this.#c = !1, this.linked && this.flush();
    }));
  }
  /**
   * @param {Set<Effect>} dirty_effects
   * @param {Set<Effect>} maybe_dirty_effects
   */
  transfer_effects(t, n) {
    for (const r of t)
      this.#f.add(r);
    for (const r of n)
      this.#u.add(r);
    t.clear(), n.clear();
  }
  /** @param {(batch: Batch) => void} fn */
  oncommit(t) {
    this.#o.add(t);
  }
  /** @param {(batch: Batch) => void} fn */
  ondiscard(t) {
    this.#r.add(t);
  }
  settled() {
    return (this.#l ??= Qn()).promise;
  }
  static ensure() {
    if (N === null) {
      const t = N = new Ge();
      !fn && !Rt && Oe(() => {
        t.#t || t.flush();
      });
    }
    return N;
  }
  apply() {
    {
      ke = null;
      return;
    }
  }
  /**
   *
   * @param {Effect} effect
   */
  schedule(t) {
    if (pn = t, t.b?.is_pending && (t.f & (Nt | tn | $n)) !== 0 && (t.f & bt) === 0) {
      t.b.defer_effect(t);
      return;
    }
    for (var n = t; n.parent !== null; ) {
      n = n.parent;
      var r = n.f;
      if (dt !== null && n === F && (O === null || (O.f & ie) === 0))
        return;
      if ((r & (ze | we)) !== 0) {
        if ((r & ne) === 0)
          return;
        n.f ^= ne;
      }
    }
    this.#i.push(n);
  }
  #h() {
    if (this.linked) {
      var t = this.#s, n = this.#e;
      t === null ? on = n : t.#e = n, n === null ? ut = t : n.#s = t, this.linked = !1;
    }
  }
}
function Yi(e) {
  var t = Rt;
  Rt = !0;
  try {
    for (var n; ; ) {
      if (Ri(), N === null)
        return (
          /** @type {T} */
          n
        );
      N.flush();
    }
  } finally {
    Rt = t;
  }
}
function Vi() {
  try {
    _i();
  } catch (e) {
    He(e, pn);
  }
}
let Pe = null;
function jn(e) {
  var t = e.length;
  if (t !== 0) {
    for (var n = 0; n < t; ) {
      var r = e[n++];
      if ((r.f & (be | oe)) === 0 && zt(r) && (Pe = /* @__PURE__ */ new Set(), gt(r), r.deps === null && r.first === null && r.nodes === null && r.teardown === null && r.ac === null && Sr(r), Pe?.size > 0)) {
        Qe.clear();
        for (const i of Pe) {
          if ((i.f & (be | oe)) !== 0) continue;
          const a = [i];
          let l = i.parent;
          for (; l !== null; )
            Pe.has(l) && (Pe.delete(l), a.push(l)), l = l.parent;
          for (let f = a.length - 1; f >= 0; f--) {
            const o = a[f];
            (o.f & (be | oe)) === 0 && gt(o);
          }
        }
        Pe.clear();
      }
    }
    Pe = null;
  }
}
function dr(e, t, n, r) {
  if (!n.has(e) && (n.add(e), e.reactions !== null))
    for (const i of e.reactions) {
      const a = i.f;
      (a & ie) !== 0 ? dr(
        /** @type {Derived} */
        i,
        t,
        n,
        r
      ) : (a & (vt | Ee)) !== 0 && (a & re) === 0 && Tn(i, t, r) && (Z(i, re), Cn(
        /** @type {Effect} */
        i
      ));
    }
}
function Tn(e, t, n) {
  const r = n.get(e);
  if (r !== void 0) return r;
  if (e.deps !== null)
    for (const i of e.deps) {
      if (Wt.call(t, i))
        return !0;
      if ((i.f & ie) !== 0 && Tn(
        /** @type {Derived} */
        i,
        t,
        n
      ))
        return n.set(
          /** @type {Derived} */
          i,
          !0
        ), !0;
    }
  return n.set(e, !1), !1;
}
function Cn(e) {
  N.schedule(e);
}
function vr(e, t) {
  if (!((e.f & we) !== 0 && (e.f & ne) !== 0)) {
    (e.f & re) !== 0 ? t.d.push(e) : (e.f & Se) !== 0 && t.m.push(e), Z(e, ne);
    for (var n = e.first; n !== null; )
      vr(n, t), n = n.next;
  }
}
function hr(e) {
  Z(e, ne);
  for (var t = e.first; t !== null; )
    hr(t), t = t.next;
}
let Jt = /* @__PURE__ */ new Set();
const Qe = /* @__PURE__ */ new Map();
let _r = !1;
function rt(e, t) {
  var n = {
    f: 0,
    // TODO ideally we could skip this altogether, but it causes type errors
    v: e,
    reactions: null,
    equals: nr,
    rv: 0,
    wv: 0
  };
  return n;
}
// @__NO_SIDE_EFFECTS__
function V(e, t) {
  const n = rt(e);
  return Ar(n), n;
}
// @__NO_SIDE_EFFECTS__
function Wi(e, t = !1, n = !0) {
  const r = rt(e);
  return t || (r.equals = rr), r;
}
function x(e, t, n = !1) {
  O !== null && // since we are untracking the function inside `$inspect.with` we need to add this check
  // to ensure we error if state is set inside an inspect effect
  (!xe || (O.f & Kt) !== 0) && ir() && (O.f & (ie | Ee | vt | Kt)) !== 0 && (Me === null || !Me.has(e)) && mi();
  let r = n ? Fe(t) : t;
  return _t(e, r, jt);
}
function _t(e, t, n = null) {
  if (!e.equals(t)) {
    Qe.set(e, Ye ? t : e.v);
    var r = Ge.ensure();
    if (r.capture(e, t), (e.f & ie) !== 0) {
      const i = (
        /** @type {Derived} */
        e
      );
      (e.f & re) !== 0 && Sn(i), ke === null && kn(i);
    }
    e.wv = Rr(), pr(e, re, n), F !== null && (F.f & ne) !== 0 && (F.f & (we | ze)) === 0 && (_e === null ? ia([e]) : _e.push(e)), !r.is_fork && Jt.size > 0 && !_r && Ki();
  }
  return t;
}
function Ki() {
  _r = !1;
  for (const e of Jt) {
    (e.f & ne) !== 0 && Z(e, Se);
    let t;
    try {
      t = zt(e);
    } catch {
      t = !0;
    }
    t && gt(e);
  }
  Jt.clear();
}
function It(e) {
  x(e, e.v + 1);
}
function pr(e, t, n) {
  var r = e.reactions;
  if (r !== null)
    for (var i = r.length, a = 0; a < i; a++) {
      var l = r[a], f = l.f, o = (f & re) === 0;
      if (o && Z(l, t), (f & Kt) !== 0)
        Jt.add(
          /** @type {Effect} */
          l
        );
      else if ((f & ie) !== 0) {
        var u = (
          /** @type {Derived} */
          l
        );
        ke?.delete(u), (f & nt) === 0 && (f & me && (F === null || (F.f & Xt) === 0) && (l.f |= nt), pr(u, Se, n));
      } else if (o) {
        var d = (
          /** @type {Effect} */
          l
        );
        (f & Ee) !== 0 && Pe !== null && Pe.add(d), n !== null ? n.push(d) : Cn(d);
      }
    }
}
function Fe(e) {
  if (typeof e != "object" || e === null || ln in e)
    return e;
  const t = Jn(e);
  if (t !== ti && t !== ni)
    return e;
  var n = /* @__PURE__ */ new Map(), r = Zn(e), i = /* @__PURE__ */ V(0), a = et, l = (f) => {
    if (et === a)
      return f();
    var o = O, u = et;
    ye(null), Gn(a);
    var d = f();
    return ye(o), Gn(u), d;
  };
  return r && n.set("length", /* @__PURE__ */ V(
    /** @type {any[]} */
    e.length
  )), new Proxy(
    /** @type {any} */
    e,
    {
      defineProperty(f, o, u) {
        (!("value" in u) || u.configurable === !1 || u.enumerable === !1 || u.writable === !1) && pi();
        var d = n.get(o);
        return d === void 0 ? l(() => {
          var _ = /* @__PURE__ */ V(u.value);
          return n.set(o, _), _;
        }) : x(d, u.value, !0), !0;
      },
      deleteProperty(f, o) {
        var u = n.get(o);
        if (u === void 0) {
          if (o in f) {
            const d = l(() => /* @__PURE__ */ V(te));
            n.set(o, d), It(i);
          }
        } else
          x(u, te), It(i);
        return !0;
      },
      get(f, o, u) {
        if (o === ln)
          return e;
        var d = n.get(o), _ = o in f;
        if (d === void 0 && (!_ || Mt(f, o)?.writable) && (d = l(() => {
          var c = Fe(_ ? f[o] : te), v = /* @__PURE__ */ V(c);
          return v;
        }), n.set(o, d)), d !== void 0) {
          var h = s(d);
          return h === te ? void 0 : h;
        }
        return Reflect.get(f, o, u);
      },
      getOwnPropertyDescriptor(f, o) {
        var u = Reflect.getOwnPropertyDescriptor(f, o);
        if (u && "value" in u) {
          var d = n.get(o);
          d && (u.value = s(d));
        } else if (u === void 0) {
          var _ = n.get(o), h = _?.v;
          if (_ !== void 0 && h !== te)
            return {
              enumerable: !0,
              configurable: !0,
              value: h,
              writable: !0
            };
        }
        return u;
      },
      has(f, o) {
        if (o === ln)
          return !0;
        var u = n.get(o), d = u !== void 0 && u.v !== te || Reflect.has(f, o);
        if (u !== void 0 || F !== null && (!d || Mt(f, o)?.writable)) {
          u === void 0 && (u = l(() => {
            var h = d ? Fe(f[o]) : te, c = /* @__PURE__ */ V(h);
            return c;
          }), n.set(o, u));
          var _ = s(u);
          if (_ === te)
            return !1;
        }
        return d;
      },
      set(f, o, u, d) {
        var _ = n.get(o), h = o in f;
        if (r && o === "length")
          for (var c = u; c < /** @type {Source<number>} */
          _.v; c += 1) {
            var v = n.get(c + "");
            v !== void 0 ? x(v, te) : c in f && (v = l(() => /* @__PURE__ */ V(te)), n.set(c + "", v));
          }
        if (_ === void 0)
          (!h || Mt(f, o)?.writable) && (_ = l(() => /* @__PURE__ */ V(void 0)), x(_, Fe(u)), n.set(o, _));
        else {
          h = _.v !== te;
          var y = l(() => Fe(u));
          x(_, y);
        }
        var p = Reflect.getOwnPropertyDescriptor(f, o);
        if (p?.set && p.set.call(d, u), !h) {
          if (r && typeof o == "string") {
            var E = (
              /** @type {Source<number>} */
              n.get("length")
            ), k = Number(o);
            Number.isInteger(k) && k >= E.v && x(E, k + 1);
          }
          It(i);
        }
        return !0;
      },
      ownKeys(f) {
        s(i);
        var o = Reflect.ownKeys(f).filter((_) => {
          var h = n.get(_);
          return h === void 0 || h.v !== te;
        });
        for (var [u, d] of n)
          d.v !== te && !(u in f) && o.push(u);
        return o;
      },
      setPrototypeOf() {
        gi();
      }
    }
  );
}
var gn, gr, mr, br;
function Xi() {
  if (gn === void 0) {
    gn = window, gr = /Firefox/.test(navigator.userAgent);
    var e = Element.prototype, t = Node.prototype, n = Text.prototype;
    mr = Mt(t, "firstChild").get, br = Mt(t, "nextSibling").get, zn(e) && (e[vn] = void 0, e[er] = null, e[oi] = void 0, e.__e = void 0), zn(n) && (n[hn] = void 0);
  }
}
function De(e = "") {
  return document.createTextNode(e);
}
// @__NO_SIDE_EFFECTS__
function pt(e) {
  return (
    /** @type {TemplateNode | null} */
    mr.call(e)
  );
}
// @__NO_SIDE_EFFECTS__
function Dt(e) {
  return (
    /** @type {TemplateNode | null} */
    br.call(e)
  );
}
function g(e, t) {
  return /* @__PURE__ */ pt(e);
}
function qe(e, t = !1) {
  {
    var n = /* @__PURE__ */ pt(e);
    return n instanceof Comment && n.data === "" ? /* @__PURE__ */ Dt(n) : n;
  }
}
function b(e, t = 1, n = !1) {
  let r = e;
  for (; t--; )
    r = /** @type {TemplateNode} */
    /* @__PURE__ */ Dt(r);
  return r;
}
function Zi(e) {
  e.textContent = "";
}
function wr() {
  return !1;
}
function Ji(e, t, n) {
  return (
    /** @type {T extends keyof HTMLElementTagNameMap ? HTMLElementTagNameMap[T] : Element} */
    n ? document.createElement(e, { is: n }) : document.createElement(e)
  );
}
function Qi(e) {
  F === null && (O === null && hi(), vi()), Ye && di();
}
function $i(e, t) {
  var n = t.last;
  n === null ? t.last = t.first = e : (n.next = e, e.prev = n, t.last = e);
}
function Ve(e, t) {
  var n = F;
  n !== null && (n.f & oe) !== 0 && (e |= oe);
  var r = {
    ctx: fe,
    deps: null,
    nodes: null,
    f: e | re | me,
    first: null,
    fn: t,
    last: null,
    next: null,
    parent: n,
    b: n && n.b,
    prev: null,
    teardown: null,
    wv: 0,
    ac: null
  };
  N?.register_created_effect(r);
  var i = r;
  if ((e & Nt) !== 0)
    dt !== null ? dt.push(r) : Ge.ensure().schedule(r);
  else if (t !== null) {
    try {
      gt(r);
    } catch (l) {
      throw ce(r), l;
    }
    i.deps === null && i.teardown === null && i.nodes === null && i.first === i.last && // either `null`, or a singular child
    (i.f & wt) === 0 && (i = i.first, (e & Ee) !== 0 && (e & tt) !== 0 && i !== null && (i.f |= tt));
  }
  if (i !== null && (i.parent = n, n !== null && $i(i, n), O !== null && (O.f & ie) !== 0 && (e & ze) === 0)) {
    var a = (
      /** @type {Derived} */
      O
    );
    (a.effects ??= []).push(i);
  }
  return r;
}
function An() {
  return O !== null && !xe;
}
function yr(e) {
  const t = Ve(tn, null);
  return Z(t, ne), t.teardown = e, t;
}
function mn(e) {
  Qi();
  var t = (
    /** @type {Effect} */
    F.f
  ), n = !O && (t & we) !== 0 && fe !== null && !fe.i;
  if (n) {
    var r = (
      /** @type {ComponentContext} */
      fe
    );
    (r.e ??= []).push(e);
  } else
    return Er(e);
}
function Er(e) {
  return Ve(Nt | si, e);
}
function ea(e) {
  Ge.ensure();
  const t = Ve(ze | wt, e);
  return (n = {}) => new Promise((r) => {
    n.outro ? $e(t, () => {
      ce(t), r(void 0);
    }) : (ce(t), r(void 0));
  });
}
function ta(e) {
  return Ve(vt | wt, e);
}
function kr(e, t = 0) {
  return Ve(tn | t, e);
}
function U(e, t = [], n = [], r = []) {
  zi(r, t, n, (i) => {
    Ve(tn, () => {
      e(...i.map(s));
    });
  });
}
function nn(e, t = 0) {
  var n = Ve(Ee | t, e);
  return n;
}
function pe(e) {
  return Ve(we | wt, e);
}
function xr(e) {
  var t = e.teardown;
  if (t !== null) {
    const n = Ye, r = O;
    qn(!0), ye(null);
    try {
      t.call(null);
    } finally {
      qn(n), ye(r);
    }
  }
}
function Mn(e, t = !1) {
  var n = e.first;
  for (e.first = e.last = null; n !== null; ) {
    const i = n.ac;
    i !== null && yt(() => {
      i.abort(Ft);
    });
    var r = n.next;
    (n.f & ze) !== 0 ? n.parent = null : ce(n, t), n = r;
  }
}
function na(e) {
  for (var t = e.first; t !== null; ) {
    var n = t.next;
    (t.f & we) === 0 && ce(t), t = n;
  }
}
function ce(e, t = !0) {
  var n = !1;
  (t || (e.f & ai) !== 0) && e.nodes !== null && e.nodes.end !== null && (ra(
    e.nodes.start,
    /** @type {TemplateNode} */
    e.nodes.end
  ), n = !0), e.f |= Bn, Mn(e, t && !n), Pt(e, 0);
  var r = e.nodes && e.nodes.t;
  if (r !== null)
    for (const a of r)
      a.stop();
  xr(e), e.f ^= Bn, e.f |= be;
  var i = e.parent;
  i !== null && i.first !== null && Sr(e), e.next = e.prev = e.teardown = e.ctx = e.deps = e.fn = e.nodes = e.ac = e.b = null;
}
function ra(e, t) {
  for (; e !== null; ) {
    var n = e === t ? null : /* @__PURE__ */ Dt(e);
    e.remove(), e = n;
  }
}
function Sr(e) {
  var t = e.parent, n = e.prev, r = e.next;
  n !== null && (n.next = r), r !== null && (r.prev = n), t !== null && (t.first === e && (t.first = r), t.last === e && (t.last = n));
}
function $e(e, t, n = !0) {
  var r = [];
  Tr(e, r, !0);
  var i = () => {
    n && ce(e), t && t();
  }, a = r.length;
  if (a > 0) {
    var l = () => --a || i();
    for (var f of r)
      f.out(l);
  } else
    i();
}
function Tr(e, t, n) {
  if ((e.f & oe) === 0) {
    e.f ^= oe;
    var r = e.nodes && e.nodes.t;
    if (r !== null)
      for (const f of r)
        (f.is_global || n) && t.push(f);
    for (var i = e.first; i !== null; ) {
      var a = i.next;
      if ((i.f & ze) === 0) {
        var l = (i.f & tt) !== 0 || // If this is a branch effect without a block effect parent,
        // it means the parent block effect was pruned. In that case,
        // transparency information was transferred to the branch effect.
        (i.f & we) !== 0 && (e.f & Ee) !== 0;
        Tr(i, t, l ? n : !1);
      }
      i = a;
    }
  }
}
function Qt(e) {
  Cr(e, !0);
}
function Cr(e, t) {
  if ((e.f & oe) !== 0) {
    e.f ^= oe, (e.f & ne) === 0 && (Z(e, re), Ge.ensure().schedule(e));
    for (var n = e.first; n !== null; ) {
      var r = n.next, i = (n.f & tt) !== 0 || (n.f & we) !== 0;
      Cr(n, i ? t : !1), n = r;
    }
    var a = e.nodes && e.nodes.t;
    if (a !== null)
      for (const l of a)
        (l.is_global || t) && l.in();
  }
}
function Rn(e, t) {
  if (e.nodes)
    for (var n = e.nodes.start, r = e.nodes.end; n !== null; ) {
      var i = n === r ? null : /* @__PURE__ */ Dt(n);
      t.append(n), n = i;
    }
}
let qt = !1, Ye = !1;
function qn(e) {
  Ye = e;
}
let O = null, xe = !1;
function ye(e) {
  O = e;
}
let F = null;
function Ie(e) {
  F = e;
}
let Me = null;
function Ar(e) {
  O !== null && (Me ??= /* @__PURE__ */ new Set()).add(e);
}
let ue = null, he = 0, _e = null;
function ia(e) {
  _e = e;
}
let Mr = 1, Je = 0, et = Je;
function Gn(e) {
  et = e;
}
function Rr() {
  return ++Mr;
}
function zt(e) {
  var t = e.f;
  if ((t & re) !== 0)
    return !0;
  if (t & ie && (e.f &= ~nt), (t & Se) !== 0) {
    for (var n = (
      /** @type {Value[]} */
      e.deps
    ), r = n.length, i = 0; i < r; i++) {
      var a = n[i];
      if (zt(
        /** @type {Derived} */
        a
      ) && ur(
        /** @type {Derived} */
        a
      ), a.wv > e.wv)
        return !0;
    }
    (t & me) !== 0 && // During time traveling we don't want to reset the status so that
    // traversal of the graph in the other batches still happens
    ke === null && Z(e, ne);
  }
  return !1;
}
function Ir(e, t, n = !0) {
  var r = e.reactions;
  if (r !== null && !(Me !== null && Me.has(e)))
    for (var i = 0; i < r.length; i++) {
      var a = r[i];
      (a.f & ie) !== 0 ? Ir(
        /** @type {Derived} */
        a,
        t,
        !1
      ) : t === a && (n ? Z(a, re) : (a.f & ne) !== 0 && Z(a, Se), Cn(
        /** @type {Effect} */
        a
      ));
    }
}
function Lr(e) {
  var t = ue, n = he, r = _e, i = O, a = Me, l = fe, f = xe, o = et, u = e.f;
  ue = /** @type {null | Value[]} */
  null, he = 0, _e = null, O = (u & (we | ze)) === 0 ? e : null, Me = null, ht(e.ctx), xe = !1, et = ++Je, e.ac !== null && (yt(() => {
    e.ac.abort(Ft);
  }), e.ac = null);
  try {
    e.f |= Xt;
    var d = (
      /** @type {Function} */
      e.fn
    ), _ = d();
    e.f |= bt;
    var h = e.deps, c = N?.is_fork;
    if (ue !== null) {
      var v;
      if (c || Pt(e, he), h !== null && he > 0)
        for (h.length = he + ue.length, v = 0; v < ue.length; v++)
          h[he + v] = ue[v];
      else
        e.deps = h = ue;
      if (An() && (e.f & me) !== 0)
        for (v = he; v < h.length; v++)
          (h[v].reactions ??= []).push(e);
    } else !c && h !== null && he < h.length && (Pt(e, he), h.length = he);
    if (ir() && _e !== null && !xe && h !== null && (e.f & (ie | Se | re)) === 0)
      for (v = 0; v < /** @type {Source[]} */
      _e.length; v++)
        Ir(
          _e[v],
          /** @type {Effect} */
          e
        );
    if (i !== null && i !== e) {
      if (Je++, i.deps !== null)
        for (let y = 0; y < n; y += 1)
          i.deps[y].rv = Je;
      if (t !== null)
        for (const y of t)
          y.rv = Je;
      _e !== null && (r === null ? r = _e : r.push(.../** @type {Source[]} */
      _e));
    }
    return (e.f & je) !== 0 && (e.f ^= je), _;
  } catch (y) {
    return sr(y);
  } finally {
    e.f ^= Xt, ue = t, he = n, _e = r, O = i, Me = a, ht(l), xe = f, et = o;
  }
}
function aa(e, t) {
  let n = t.reactions;
  if (n !== null) {
    var r = Qr.call(n, e);
    if (r !== -1) {
      var i = n.length - 1;
      i === 0 ? n = t.reactions = null : (n[r] = n[i], n.pop());
    }
  }
  if (n === null && (t.f & ie) !== 0 && // Destroying a child effect while updating a parent effect can cause a dependency to appear
  // to be unused, when in fact it is used by the currently-updating parent. Checking `new_deps`
  // allows us to skip the expensive work of disconnecting and immediately reconnecting it
  (ue === null || !Wt.call(ue, t))) {
    var a = (
      /** @type {Derived} */
      t
    );
    (a.f & me) !== 0 && (a.f ^= me, a.f &= ~nt), a.v !== te && kn(a), a.ac !== null && yt(() => {
      a.ac.abort(Ft), a.ac = null, Z(a, re);
    }), qi(a), Pt(a, 0);
  }
}
function Pt(e, t) {
  var n = e.deps;
  if (n !== null)
    for (var r = t; r < n.length; r++)
      aa(e, n[r]);
}
function gt(e) {
  var t = e.f;
  if ((t & be) === 0) {
    Z(e, ne);
    var n = F, r = qt;
    F = e, qt = (t & (we | ze)) === 0;
    try {
      (t & (Ee | $n)) !== 0 ? na(e) : Mn(e), xr(e);
      var i = Lr(e);
      e.teardown = typeof i == "function" ? i : null, e.wv = Mr;
      var a;
    } finally {
      qt = r, F = n;
    }
  }
}
async function sa() {
  await Promise.resolve(), Yi();
}
function s(e) {
  var t = e.f, n = (t & ie) !== 0;
  if (O !== null && !xe) {
    var r = F !== null && (F.f & be) !== 0;
    if (!r && (Me === null || !Me.has(e))) {
      var i = O.deps;
      if ((O.f & Xt) !== 0)
        e.rv < Je && (e.rv = Je, ue === null && i !== null && i[he] === e ? he++ : ue === null ? ue = [e] : ue.push(e));
      else {
        O.deps ??= [], Wt.call(O.deps, e) || O.deps.push(e);
        var a = e.reactions;
        a === null ? e.reactions = [O] : Wt.call(a, O) || a.push(O);
      }
    }
  }
  if (Ye && Qe.has(e))
    return Qe.get(e);
  if (n) {
    var l = (
      /** @type {Derived} */
      e
    );
    if (Ye) {
      var f = l.v;
      return ((l.f & ne) === 0 && l.reactions !== null || Pr(l)) && (f = Sn(l)), Qe.set(l, f), f;
    }
    var o = (l.f & me) === 0 && !xe && O !== null && (qt || (O.f & me) !== 0), u = (l.f & bt) === 0;
    zt(l) && (o && (l.f |= me), ur(l)), o && !u && (cr(l), Nr(l));
  }
  if (ke?.has(e))
    return ke.get(e);
  if ((e.f & je) !== 0)
    throw e.v;
  return e.v;
}
function Nr(e) {
  if (e.f |= me, e.deps !== null)
    for (const t of e.deps)
      (t.reactions ??= []).push(e), (t.f & ie) !== 0 && (t.f & me) === 0 && (cr(
        /** @type {Derived} */
        t
      ), Nr(
        /** @type {Derived} */
        t
      ));
}
function Pr(e) {
  if (e.v === te) return !0;
  if (e.deps === null) return !1;
  for (const t of e.deps)
    if (Qe.has(t) || (t.f & ie) !== 0 && Pr(
      /** @type {Derived} */
      t
    ))
      return !0;
  return !1;
}
function In(e) {
  var t = xe;
  try {
    return xe = !0, e();
  } finally {
    xe = t;
  }
}
const la = ["touchstart", "touchmove"];
function oa(e) {
  return la.includes(e);
}
const Ct = Symbol("events"), Or = /* @__PURE__ */ new Set(), bn = /* @__PURE__ */ new Set();
function fa(e, t, n, r = {}) {
  function i(a) {
    if (r.capture || wn.call(t, a), !a.cancelBubble)
      return yt(() => n?.call(this, a));
  }
  return e.startsWith("pointer") || e.startsWith("touch") || e === "wheel" ? Oe(() => {
    t.addEventListener(e, i, r);
  }) : t.addEventListener(e, i, r), i;
}
function ua(e, t, n, r, i) {
  var a = { capture: r, passive: i }, l = fa(e, t, n, a);
  (t === document.body || // @ts-ignore
  t === window || // @ts-ignore
  t === document || // Firefox has quirky behavior, it can happen that we still get "canplay" events when the element is already removed
  t instanceof HTMLMediaElement) && yr(() => {
    t.removeEventListener(e, l, a);
  });
}
function $(e, t, n) {
  (t[Ct] ??= {})[e] = n;
}
function lt(e) {
  for (var t = 0; t < e.length; t++)
    Or.add(e[t]);
  for (var n of bn)
    n(e);
}
let Yn = null;
function wn(e) {
  var t = this, n = (
    /** @type {Node} */
    t.ownerDocument
  ), r = e.type, i = e.composedPath?.() || [], a = (
    /** @type {null | Element} */
    i[0] || e.target
  );
  Yn = e;
  var l = 0, f = Yn === e && e[Ct];
  if (f) {
    var o = i.indexOf(f);
    if (o !== -1 && (t === document || t === /** @type {any} */
    window)) {
      e[Ct] = t;
      return;
    }
    var u = i.indexOf(t);
    if (u === -1)
      return;
    o <= u && (l = o);
  }
  if (a = /** @type {Element} */
  i[l] || e.target, a !== t) {
    $r(e, "currentTarget", {
      configurable: !0,
      get() {
        return a || n;
      }
    });
    var d = O, _ = F;
    ye(null), Ie(null);
    try {
      for (var h, c = []; a !== null && a !== t; ) {
        try {
          var v = a[Ct]?.[r];
          v != null && (!/** @type {any} */
          a.disabled || // DOM could've been updated already by the time this is reached, so we check this as well
          // -> the target could not have been disabled because it emits the event in the first place
          e.target === a) && v.call(a, e);
        } catch (y) {
          h ? c.push(y) : h = y;
        }
        if (e.cancelBubble) break;
        l++, a = l < i.length ? (
          /** @type {Element} */
          i[l]
        ) : null;
      }
      if (h) {
        for (let y of c)
          queueMicrotask(() => {
            throw y;
          });
        throw h;
      }
    } finally {
      e[Ct] = t, delete e.currentTarget, ye(d), Ie(_);
    }
  }
}
const ca = (
  // We gotta write it like this because after downleveling the pure comment may end up in the wrong location
  globalThis?.window?.trustedTypes && /* @__PURE__ */ globalThis.window.trustedTypes.createPolicy("svelte-trusted-html", {
    /** @param {string} html */
    createHTML: (e) => e
  })
);
function da(e) {
  return (
    /** @type {string} */
    ca?.createHTML(e) ?? e
  );
}
function Fr(e) {
  var t = Ji("template");
  return t.innerHTML = da(e.replaceAll("<!>", "<!---->")), t.content;
}
function Ot(e, t) {
  var n = (
    /** @type {Effect} */
    F
  );
  n.nodes === null && (n.nodes = { start: e, end: t, a: null, t: null });
}
// @__NO_SIDE_EFFECTS__
function D(e, t) {
  var n = (t & xi) !== 0, r = (t & Si) !== 0, i, a = !e.startsWith("<!>");
  return () => {
    i === void 0 && (i = Fr(a ? e : "<!>" + e), n || (i = /** @type {TemplateNode} */
    /* @__PURE__ */ pt(i)));
    var l = (
      /** @type {TemplateNode} */
      r || gr ? document.importNode(i, !0) : i.cloneNode(!0)
    );
    if (n) {
      var f = (
        /** @type {TemplateNode} */
        /* @__PURE__ */ pt(l)
      ), o = (
        /** @type {TemplateNode} */
        l.lastChild
      );
      Ot(f, o);
    } else
      Ot(l, l);
    return l;
  };
}
// @__NO_SIDE_EFFECTS__
function va(e, t, n = "svg") {
  var r = !e.startsWith("<!>"), i = `<${n}>${r ? e : "<!>" + e}</${n}>`, a;
  return () => {
    if (!a) {
      var l = (
        /** @type {DocumentFragment} */
        Fr(i)
      ), f = (
        /** @type {Element} */
        /* @__PURE__ */ pt(l)
      );
      a = /** @type {Element} */
      /* @__PURE__ */ pt(f);
    }
    var o = (
      /** @type {TemplateNode} */
      a.cloneNode(!0)
    );
    return Ot(o, o), o;
  };
}
// @__NO_SIDE_EFFECTS__
function Dr(e, t) {
  return /* @__PURE__ */ va(e, t, "svg");
}
function Gt(e = "") {
  {
    var t = De(e + "");
    return Ot(t, t), t;
  }
}
function Lt() {
  var e = document.createDocumentFragment(), t = document.createComment(""), n = De();
  return e.append(t, n), Ot(t, n), e;
}
function C(e, t) {
  e !== null && e.before(
    /** @type {Node} */
    t
  );
}
function M(e, t) {
  var n = t == null ? "" : typeof t == "object" ? `${t}` : t;
  n !== /** @type {any} */
  (e[hn] ??= e.nodeValue) && (e[hn] = n, e.nodeValue = `${n}`);
}
function ha(e, t) {
  return _a(e, t);
}
const Ut = /* @__PURE__ */ new Map();
function _a(e, { target: t, anchor: n, props: r = {}, events: i, context: a, intro: l = !0, transformError: f }) {
  Xi();
  var o = void 0, u = ea(() => {
    var d = n ?? t.appendChild(De());
    Fi(
      /** @type {TemplateNode} */
      d,
      {
        pending: () => {
        }
      },
      (c) => {
        at({});
        var v = (
          /** @type {ComponentContext} */
          fe
        );
        a && (v.c = a), i && (r.$$events = i), o = e(c, r) || {}, st();
      },
      f
    );
    var _ = /* @__PURE__ */ new Set(), h = (c) => {
      for (var v = 0; v < c.length; v++) {
        var y = c[v];
        if (!_.has(y)) {
          _.add(y);
          var p = oa(y);
          for (const w of [t, document]) {
            var E = Ut.get(w);
            E === void 0 && (E = /* @__PURE__ */ new Map(), Ut.set(w, E));
            var k = E.get(y);
            k === void 0 ? (w.addEventListener(y, wn, { passive: p }), E.set(y, 1)) : E.set(y, k + 1);
          }
        }
      }
    };
    return h(en(Or)), bn.add(h), () => {
      for (var c of _)
        for (const p of [t, document]) {
          var v = (
            /** @type {Map<string, number>} */
            Ut.get(p)
          ), y = (
            /** @type {number} */
            v.get(c)
          );
          --y == 0 ? (p.removeEventListener(c, wn), v.delete(c), v.size === 0 && Ut.delete(p)) : v.set(c, y);
        }
      bn.delete(h), d !== n && d.parentNode?.removeChild(d);
    };
  });
  return pa.set(o, u), o;
}
let pa = /* @__PURE__ */ new WeakMap();
class zr {
  /** @type {TemplateNode} */
  anchor;
  /** @type {Map<Batch, Key>} */
  #t = /* @__PURE__ */ new Map();
  /**
   * Map of keys to effects that are currently rendered in the DOM.
   * These effects are visible and actively part of the document tree.
   * Example:
   * ```
   * {#if condition}
   * 	foo
   * {:else}
   * 	bar
   * {/if}
   * ```
   * Can result in the entries `true->Effect` and `false->Effect`
   * @type {Map<Key, Effect>}
   */
  #s = /* @__PURE__ */ new Map();
  /**
   * Similar to #onscreen with respect to the keys, but contains branches that are not yet
   * in the DOM, because their insertion is deferred.
   * @type {Map<Key, Branch>}
   */
  #e = /* @__PURE__ */ new Map();
  /**
   * Keys of effects that are currently outroing
   * @type {Set<Key>}
   */
  #o = /* @__PURE__ */ new Set();
  /**
   * Whether to pause (i.e. outro) on change, or destroy immediately.
   * This is necessary for `<svelte:element>`
   */
  #r = !0;
  /**
   * @param {TemplateNode} anchor
   * @param {boolean} transition
   */
  constructor(t, n = !0) {
    this.anchor = t, this.#r = n;
  }
  /**
   * @param {Batch} batch
   */
  #a = (t) => {
    if (this.#t.has(t)) {
      var n = (
        /** @type {Key} */
        this.#t.get(t)
      ), r = this.#s.get(n);
      if (r)
        Qt(r), this.#o.delete(n);
      else {
        var i = this.#e.get(n);
        i && (Qt(i.effect), this.#s.set(n, i.effect), this.#e.delete(n), i.fragment.lastChild.remove(), this.anchor.before(i.fragment), r = i.effect);
      }
      for (const [a, l] of this.#t) {
        if (this.#t.delete(a), a === t)
          break;
        const f = this.#e.get(l);
        f && (ce(f.effect), this.#e.delete(l));
      }
      for (const [a, l] of this.#s) {
        if (a === n || this.#o.has(a)) continue;
        const f = () => {
          if (Array.from(this.#t.values()).includes(a)) {
            var u = document.createDocumentFragment();
            Rn(l, u), u.append(De()), this.#e.set(a, { effect: l, fragment: u });
          } else
            ce(l);
          this.#o.delete(a), this.#s.delete(a);
        };
        this.#r || !r ? (this.#o.add(a), $e(l, f, !1)) : f();
      }
    }
  };
  /**
   * @param {Batch} batch
   */
  #n = (t) => {
    this.#t.delete(t);
    const n = Array.from(this.#t.values());
    for (const [r, i] of this.#e)
      n.includes(r) || (ce(i.effect), this.#e.delete(r));
  };
  /**
   *
   * @param {any} key
   * @param {null | ((target: TemplateNode) => void)} fn
   */
  ensure(t, n) {
    var r = (
      /** @type {Batch} */
      N
    ), i = wr();
    if (n && !this.#s.has(t) && !this.#e.has(t))
      if (i) {
        var a = document.createDocumentFragment(), l = De();
        a.append(l), this.#e.set(t, {
          effect: pe(() => n(l)),
          fragment: a
        });
      } else
        this.#s.set(
          t,
          pe(() => n(this.anchor))
        );
    if (this.#t.set(r, t), i) {
      for (const [f, o] of this.#s)
        f === t ? r.unskip_effect(o) : r.skip_effect(o);
      for (const [f, o] of this.#e)
        f === t ? r.unskip_effect(o.effect) : r.skip_effect(o.effect);
      r.oncommit(this.#a), r.ondiscard(this.#n);
    } else
      this.#a(r);
  }
}
function se(e, t, n = !1) {
  var r = new zr(e), i = n ? tt : 0;
  function a(l, f) {
    r.ensure(l, f);
  }
  nn(() => {
    var l = !1;
    t((f, o = 0) => {
      l = !0, a(o, f);
    }), l || a(-1, null);
  }, i);
}
function it(e, t) {
  return t;
}
function ga(e, t, n) {
  for (var r = [], i = t.length, a, l = t.length, f = 0; f < i; f++) {
    let _ = t[f];
    $e(
      _,
      () => {
        if (a) {
          if (a.pending.delete(_), a.done.add(_), a.pending.size === 0) {
            var h = (
              /** @type {Set<EachOutroGroup>} */
              e.outrogroups
            );
            yn(e, en(a.done)), h.delete(a), h.size === 0 && (e.outrogroups = null);
          }
        } else
          l -= 1;
      },
      !1
    );
  }
  if (l === 0) {
    var o = r.length === 0 && n !== null && e.pending.size === 0;
    if (o) {
      var u = (
        /** @type {Element} */
        n
      ), d = (
        /** @type {Element} */
        u.parentNode
      );
      Zi(d), d.append(u), e.items.clear();
    }
    yn(e, t, !o);
  } else
    a = {
      pending: new Set(t),
      done: /* @__PURE__ */ new Set()
    }, (e.outrogroups ??= /* @__PURE__ */ new Set()).add(a);
}
function yn(e, t, n = !0) {
  var r;
  if (e.pending.size > 0) {
    r = /* @__PURE__ */ new Set();
    for (const l of e.pending.values())
      for (const f of l)
        r.add(
          /** @type {EachItem} */
          e.items.get(f).e
        );
  }
  for (var i = 0; i < t.length; i++) {
    var a = t[i];
    if (r?.has(a)) {
      a.f |= Ae;
      const l = document.createDocumentFragment();
      Rn(a, l);
    } else
      ce(t[i], n);
  }
}
var Vn;
function le(e, t, n, r, i, a = null) {
  var l = e, f = /* @__PURE__ */ new Map(), o = (t & tr) !== 0;
  if (o) {
    var u = (
      /** @type {Element} */
      e
    );
    l = u.appendChild(De());
  }
  var d = null, _ = /* @__PURE__ */ Hi(() => {
    var w = n();
    return (
      /** @type {V[]} */
      Zn(w) ? w : w == null ? [] : en(w)
    );
  }), h, c = /* @__PURE__ */ new Map(), v = !0;
  function y(w) {
    (k.effect.f & be) === 0 && (k.pending.delete(w), k.fallback = d, ma(k, h, l, t, r), d !== null && (h.length === 0 ? (d.f & Ae) === 0 ? Qt(d) : (d.f ^= Ae, At(d, null, l)) : $e(d, () => {
      d = null;
    })));
  }
  function p(w) {
    k.pending.delete(w);
  }
  var E = nn(() => {
    h = /** @type {V[]} */
    s(_);
    for (var w = h.length, S = /* @__PURE__ */ new Set(), T = (
      /** @type {Batch} */
      N
    ), R = wr(), A = 0; A < w; A += 1) {
      var B = h[A], j = r(B, A), I = v ? null : f.get(j);
      I ? (I.v && _t(I.v, B), I.i && _t(I.i, A), R && T.unskip_effect(I.e)) : (I = ba(
        f,
        v ? l : Vn ??= De(),
        B,
        j,
        A,
        i,
        t,
        n
      ), v || (I.e.f |= Ae), f.set(j, I)), S.add(j);
    }
    if (w === 0 && a && !d && (v ? d = pe(() => a(l)) : (d = pe(() => a(Vn ??= De())), d.f |= Ae)), w > S.size && ci(), !v)
      if (c.set(T, S), R) {
        for (const [J, ee] of f)
          S.has(J) || T.skip_effect(ee.e);
        T.oncommit(y), T.ondiscard(p);
      } else
        y(T);
    s(_);
  }), k = { effect: E, items: f, pending: c, outrogroups: null, fallback: d };
  v = !1;
}
function St(e) {
  for (; e !== null && (e.f & we) === 0; )
    e = e.next;
  return e;
}
function ma(e, t, n, r, i) {
  var a = (r & Ei) !== 0, l = t.length, f = e.items, o = St(e.effect.first), u, d = null, _, h = [], c = [], v, y, p, E;
  if (a)
    for (E = 0; E < l; E += 1)
      v = t[E], y = i(v, E), p = /** @type {EachItem} */
      f.get(y).e, (p.f & Ae) === 0 && (p.nodes?.a?.measure(), (_ ??= /* @__PURE__ */ new Set()).add(p));
  for (E = 0; E < l; E += 1) {
    if (v = t[E], y = i(v, E), p = /** @type {EachItem} */
    f.get(y).e, e.outrogroups !== null)
      for (const I of e.outrogroups)
        I.pending.delete(p), I.done.delete(p);
    if ((p.f & oe) !== 0 && (Qt(p), a && (p.nodes?.a?.unfix(), (_ ??= /* @__PURE__ */ new Set()).delete(p))), (p.f & Ae) !== 0)
      if (p.f ^= Ae, p === o)
        At(p, null, n);
      else {
        var k = d ? d.next : o;
        p === e.effect.last && (e.effect.last = p.prev), p.prev && (p.prev.next = p.next), p.next && (p.next.prev = p.prev), Be(e, d, p), Be(e, p, k), At(p, k, n), d = p, h = [], c = [], o = St(d.next);
        continue;
      }
    if (p !== o) {
      if (u !== void 0 && u.has(p)) {
        if (h.length < c.length) {
          var w = c[0], S;
          d = w.prev;
          var T = h[0], R = h[h.length - 1];
          for (S = 0; S < h.length; S += 1)
            At(h[S], w, n);
          for (S = 0; S < c.length; S += 1)
            u.delete(c[S]);
          Be(e, T.prev, R.next), Be(e, d, T), Be(e, R, w), o = w, d = R, E -= 1, h = [], c = [];
        } else
          u.delete(p), At(p, o, n), Be(e, p.prev, p.next), Be(e, p, d === null ? e.effect.first : d.next), Be(e, d, p), d = p;
        continue;
      }
      for (h = [], c = []; o !== null && o !== p; )
        (u ??= /* @__PURE__ */ new Set()).add(o), c.push(o), o = St(o.next);
      if (o === null)
        continue;
    }
    (p.f & Ae) === 0 && h.push(p), d = p, o = St(p.next);
  }
  if (e.outrogroups !== null) {
    for (const I of e.outrogroups)
      I.pending.size === 0 && (yn(e, en(I.done)), e.outrogroups?.delete(I));
    e.outrogroups.size === 0 && (e.outrogroups = null);
  }
  if (o !== null || u !== void 0) {
    var A = [];
    if (u !== void 0)
      for (p of u)
        (p.f & oe) === 0 && A.push(p);
    for (; o !== null; )
      (o.f & oe) === 0 && o !== e.fallback && A.push(o), o = St(o.next);
    var B = A.length;
    if (B > 0) {
      var j = (r & tr) !== 0 && l === 0 ? n : null;
      if (a) {
        for (E = 0; E < B; E += 1)
          A[E].nodes?.a?.measure();
        for (E = 0; E < B; E += 1)
          A[E].nodes?.a?.fix();
      }
      ga(e, A, j);
    }
  }
  a && Oe(() => {
    if (_ !== void 0)
      for (p of _)
        p.nodes?.a?.apply();
  });
}
function ba(e, t, n, r, i, a, l, f) {
  var o = (l & wi) !== 0 ? (l & ki) === 0 ? /* @__PURE__ */ Wi(n, !1, !1) : rt(n) : null, u = (l & yi) !== 0 ? rt(i) : null;
  return {
    v: o,
    i: u,
    e: pe(() => (a(t, o ?? n, u ?? i, f), () => {
      e.delete(r);
    }))
  };
}
function At(e, t, n) {
  if (e.nodes)
    for (var r = e.nodes.start, i = e.nodes.end, a = t && (t.f & Ae) === 0 ? (
      /** @type {EffectNodes} */
      t.nodes.start
    ) : n; r !== null; ) {
      var l = (
        /** @type {TemplateNode} */
        /* @__PURE__ */ Dt(r)
      );
      if (a.before(r), r === i)
        return;
      r = l;
    }
}
function Be(e, t, n) {
  t === null ? e.effect.first = n : t.next = n, n === null ? e.effect.last = t : n.prev = t;
}
function wa(e, t, ...n) {
  var r = new zr(e);
  nn(() => {
    const i = t() ?? null;
    r.ensure(i, i && ((a) => i(a, ...n)));
  }, tt);
}
const Wn = [...` 	
\r\f \v\uFEFF`];
function ya(e, t, n) {
  var r = e == null ? "" : "" + e;
  if (t && (r = r ? r + " " + t : t), n) {
    for (var i of Object.keys(n))
      if (n[i])
        r = r ? r + " " + i : i;
      else if (r.length)
        for (var a = i.length, l = 0; (l = r.indexOf(i, l)) >= 0; ) {
          var f = l + a;
          (l === 0 || Wn.includes(r[l - 1])) && (f === r.length || Wn.includes(r[f])) ? r = (l === 0 ? "" : r.substring(0, l)) + r.substring(f + 1) : l = f;
        }
  }
  return r === "" ? null : r;
}
function Re(e, t, n, r, i, a) {
  var l = (
    /** @type {any} */
    e[vn]
  );
  if (l !== n || l === void 0) {
    var f = ya(n, r, a);
    f == null ? e.removeAttribute("class") : t ? e.className = f : e.setAttribute("class", f), e[vn] = n;
  } else if (a && i !== a)
    for (var o in a) {
      var u = !!a[o];
      (i == null || u !== !!i[o]) && e.classList.toggle(o, u);
    }
  return a;
}
const Ea = Symbol("is custom element"), ka = Symbol("is html");
function H(e, t, n, r) {
  var i = xa(e);
  i[t] !== (i[t] = n) && (t === "loading" && (e[li] = n), n == null ? e.removeAttribute(t) : typeof n != "string" && Sa(e).includes(t) ? e[t] = n : e.setAttribute(t, n));
}
function xa(e) {
  return (
    /** @type {Record<string | symbol, unknown>} **/
    /** @type {any} */
    e[er] ??= {
      [Ea]: e.nodeName.includes("-"),
      [ka]: e.namespaceURI === Ti
    }
  );
}
var Kn = /* @__PURE__ */ new Map();
function Sa(e) {
  var t = e.getAttribute("is") || e.nodeName, n = Kn.get(t);
  if (n) return n;
  Kn.set(t, n = []);
  for (var r, i = e, a = Element.prototype; a !== i; ) {
    r = ei(i);
    for (var l in r)
      r[l].set && // better safe than sorry, we don't want spread attributes to mess with HTML content
      l !== "innerHTML" && l !== "textContent" && l !== "innerText" && n.push(l);
    i = Jn(i);
  }
  return n;
}
function Br(e, t, n = t) {
  var r = /* @__PURE__ */ new WeakSet();
  Ni(e, "input", async (i) => {
    var a = i ? e.defaultValue : e.value;
    if (a = un(e) ? cn(a) : a, n(a), N !== null && r.add(N), await sa(), a !== (a = t())) {
      var l = e.selectionStart, f = e.selectionEnd, o = e.value.length;
      if (e.value = a ?? "", f !== null) {
        var u = e.value.length;
        l === f && f === o && u > o ? (e.selectionStart = u, e.selectionEnd = u) : (e.selectionStart = l, e.selectionEnd = Math.min(f, u));
      }
    }
  }), // If we are hydrating and the value has since changed,
  // then use the updated value from the input instead.
  // If defaultValue is set, then value == defaultValue
  // TODO Svelte 6: remove input.value check and set to empty string?
  In(t) == null && e.value && (n(un(e) ? cn(e.value) : e.value), N !== null && r.add(N)), kr(() => {
    var i = t();
    if (e === document.activeElement) {
      var a = (
        /** @type {Batch} */
        N
      );
      if (r.has(a))
        return;
    }
    un(e) && i === cn(e.value) || e.type === "date" && !i && !e.value || i !== e.value && (e.value = i ?? "");
  });
}
function un(e) {
  var t = e.type;
  return t === "number" || t === "range";
}
function cn(e) {
  return e === "" ? null : +e;
}
function Ue(e, t, n, r) {
  var i = (
    /** @type {V} */
    r
  ), a = !0, l = () => (a && (a = !1, i = /** @type {V} */
  r), i), f;
  f = /** @type {V} */
  e[t], f === void 0 && r !== void 0 && (f = l());
  var o;
  return o = () => {
    var u = (
      /** @type {V} */
      e[t]
    );
    return u === void 0 ? l() : (a = !0, u);
  }, o;
}
function Ta(e) {
  fe === null && fi(), mn(() => {
    const t = In(e);
    if (typeof t == "function") return (
      /** @type {() => void} */
      t
    );
  });
}
const Ca = "5";
typeof window < "u" && ((window.__svelte ??= {}).v ??= /* @__PURE__ */ new Set()).add(Ca);
var Aa = /* @__PURE__ */ Dr('<rect width="1" height="1"></rect>'), Ma = /* @__PURE__ */ Dr('<svg shape-rendering="crispEdges" aria-hidden="true"><!></svg>');
function mt(e, t) {
  let n = Ue(t, "size", 3, 16), r = Ue(t, "color", 3, "currentColor"), i = Ue(t, "class", 3, "");
  const a = {
    "chevron-right": ["##....", "####..", "######", "####..", "##...."],
    "chevron-down": ["#####", ".###.", "..#.."],
    check: [
      "...........##.",
      "..........###.",
      ".........###..",
      "##......###...",
      "###....###....",
      ".###..###.....",
      "..######......",
      "...####.......",
      "....##........"
    ],
    cross: [
      "##........##",
      "###......###",
      ".###....###.",
      "..###..###..",
      "...######...",
      "....####....",
      "....####....",
      "...######...",
      "..###..###..",
      ".###....###.",
      "###......###",
      "##........##"
    ],
    terminal: [
      "############",
      "#..........#",
      "#.##.......#",
      "#..####....#",
      "#.##.......#",
      "#.....####.#",
      "#..........#",
      "############"
    ],
    copy: [
      "######....",
      "#....#....",
      "#..######.",
      "#..#....#.",
      "######..#.",
      "...#....#.",
      "...######."
    ],
    warn: [
      ".....##.....",
      "....####....",
      "...##..##...",
      "...##..##...",
      "..##.##.##..",
      "..##.##.##..",
      ".##......##.",
      ".##..##..##.",
      "############"
    ],
    refresh: [
      "...######...",
      ".##......##.",
      "##...##...##",
      "##...#####..",
      "##..........",
      ".##......##.",
      "...######..."
    ],
    globe: [
      "....######....",
      "..##########..",
      ".###..##..###.",
      ".##...##...##.",
      "##....##....##",
      "##....##....##",
      "##############",
      "##....##....##",
      "##....##....##",
      ".##...##...##.",
      ".###..##..###.",
      "..##########..",
      "....######...."
    ],
    dashboard: [
      "############",
      "#..##....##.",
      "#..##....##.",
      "#...........",
      "#..####..##.",
      "#..####..##.",
      "############"
    ],
    rules: [
      "########..",
      "#......##.",
      "#.####.###",
      "#........#",
      "#.######.#",
      "#........#",
      "#.######.#",
      "#........#",
      "##########"
    ],
    changes: [
      "..####....",
      "..####....",
      "##########",
      "..####....",
      "..####....",
      "..........",
      "##########",
      "##########"
    ],
    updates: [
      "....##....",
      "...####...",
      "..##..##..",
      ".##....##.",
      "##########",
      "....##....",
      "....##....",
      "....##...."
    ],
    external: [
      ".....#####",
      ".....#..##",
      ".....#.###",
      "##...#####",
      "#.#...#...",
      "#..#..#...",
      "####..#...",
      "......####"
    ],
    iconset: [
      "....######",
      "....#....#",
      "######...#",
      "#....#...#",
      "#....#...#",
      "######...#",
      "....######"
    ]
  };
  let l = /* @__PURE__ */ X(() => a[t.name] || a["chevron-right"]), f = /* @__PURE__ */ X(() => s(l) ? Math.max(...s(l).map((y) => y.length)) : 0), o = /* @__PURE__ */ X(() => s(l) ? s(l).length : 0), u = /* @__PURE__ */ X(() => Math.max(s(f), s(o), 1)), d = /* @__PURE__ */ X(() => Math.floor((s(u) - s(f)) / 2)), _ = /* @__PURE__ */ X(() => Math.floor((s(u) - s(o)) / 2));
  var h = Ma(), c = g(h);
  {
    var v = (y) => {
      var p = Lt(), E = qe(p);
      le(E, 17, () => s(l), it, (k, w, S) => {
        var T = Lt(), R = qe(T);
        le(R, 17, () => s(w).split(""), it, (A, B, j) => {
          var I = Lt(), J = qe(I);
          {
            var ee = (W) => {
              var Y = Aa();
              U(() => {
                H(Y, "x", j + s(d)), H(Y, "y", S + s(_)), H(Y, "fill", r());
              }), C(W, Y);
            };
            se(J, (W) => {
              s(B) === "#" && W(ee);
            });
          }
          C(A, I);
        }), C(k, T);
      }), C(y, p);
    };
    se(c, (y) => {
      s(l) && y(v);
    });
  }
  U(() => {
    Re(h, 0, `px ${i() ?? ""}`), H(h, "width", n()), H(h, "height", n()), H(h, "viewBox", `0 0 ${s(u) ?? ""} ${s(u) ?? ""}`);
  }), C(e, h);
}
function Ra() {
  const e = document.getElementById("prm-data");
  if (!e?.textContent)
    throw new Error("public page data is missing");
  return JSON.parse(e.textContent);
}
function Xn(e, t) {
  return t === "geosite" ? e.geosite : e.rules;
}
function $t(e, t) {
  return t === "geosite" ? e.geosite : e.rules;
}
function En(e, t, n) {
  return e.options.find((r) => r.id === t && Xn(r, n)) ?? e.options.find((r) => Xn(r, n)) ?? e.options[0];
}
function Ur(...e) {
  return e.map((t) => encodeURIComponent(t)).join("/");
}
function Yt(e, t, n, r) {
  const i = r ? `${n}@${r}` : n;
  return `rules/${Ur(e.id, "geosite", t, i)}${e.ext}`;
}
function Ia(e, t) {
  return `static/icons/${Ur(e, t)}`;
}
function ge(e) {
  return e.toLocaleString("zh-CN");
}
function La(e) {
  return e === void 0 ? "" : e < 1024 ? `${e} B` : e < 1024 * 1024 ? `${(e / 1024).toFixed(1)} KB` : `${(e / 1024 / 1024).toFixed(2)} MB`;
}
function Na(e) {
  const t = new Date(e);
  return Number.isNaN(t.getTime()) ? e : t.toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}
async function Ln(e) {
  try {
    const t = new URL(e, location.href).href;
    if (navigator.clipboard?.writeText)
      return await navigator.clipboard.writeText(t), !0;
    const n = document.createElement("textarea");
    return n.value = t, document.body.appendChild(n), n.select(), document.execCommand("copy"), n.remove(), !0;
  } catch {
    return !1;
  }
}
var Pa = /* @__PURE__ */ D('<button type="button" role="radio"><span><strong> </strong><small> </small></span> <i></i></button>'), Oa = /* @__PURE__ */ D('<div class="client-menu" role="radiogroup"></div>'), Fa = /* @__PURE__ */ D('<div><button type="button"><img width="32" height="32" alt="" aria-hidden="true"/> <span class="ctext"><strong> </strong> <small> </small></span> <span class="client-state"><i></i> <!></span></button> <!></div>'), Da = /* @__PURE__ */ D('<section class="public-block client-section" aria-labelledby="client-heading"><div class="block-head"><h2 class="block-title" id="client-heading">选择客户端</h2> <span class="label">输出格式与下载目标</span></div> <div class="client-picker"></div></section>');
function za(e, t) {
  at(t, !0);
  let n = /* @__PURE__ */ V(null);
  function r(f) {
    $t(t.clients[f], t.view) && (t.onselectclient(f), x(n, t.clients[f].options.length > 1 && s(n) !== f ? f : null, !0));
  }
  function i(f, o, u) {
    f.stopPropagation(), t.onselecttarget(o, u), x(n, null);
  }
  var a = Da(), l = b(g(a), 2);
  le(l, 23, () => t.clients, (f) => f.id, (f, o, u) => {
    const d = /* @__PURE__ */ X(() => $t(s(o), t.view)), _ = /* @__PURE__ */ X(() => En(s(o), t.selectedTargets[s(o).id], t.view));
    var h = Fa();
    let c;
    var v = g(h);
    let y;
    var p = g(v), E = b(p, 2), k = g(E), w = g(k), S = b(k, 2), T = g(S), R = b(E, 2), A = b(g(R), 2);
    {
      var B = (J) => {
        mt(J, { name: "chevron-down", size: 10 });
      };
      se(A, (J) => {
        s(o).options.length > 1 && J(B);
      });
    }
    var j = b(v, 2);
    {
      var I = (J) => {
        var ee = Oa();
        le(ee, 21, () => s(o).options, (W) => W.id, (W, Y) => {
          const G = /* @__PURE__ */ X(() => t.view === "geosite" ? s(Y).geosite : s(Y).rules);
          var z = Pa();
          let P;
          var ae = g(z), de = g(ae), Te = g(de), Le = b(de), Ne = g(Le);
          U(() => {
            H(z, "aria-checked", s(_).id === s(Y).id), z.disabled = !s(G), P = Re(z, 1, "", null, P, {
              on: s(_).id === s(Y).id,
              off: !s(G)
            }), M(Te, s(Y).name), M(Ne, s(Y).ext);
          }), $("click", z, (Ce) => i(Ce, s(u), s(Y).id)), C(W, z);
        }), U(() => H(ee, "aria-label", `${s(o).name} 格式`)), C(J, ee);
      };
      se(j, (J) => {
        s(o).options.length > 1 && s(n) === s(u) && J(I);
      });
    }
    U(() => {
      c = Re(h, 1, "client-slot", null, c, { open: s(n) === s(u) }), y = Re(v, 1, "client-card", null, y, {
        on: t.selectedIndex === s(u) && s(d),
        off: !s(d)
      }), v.disabled = !s(d), H(v, "aria-expanded", s(o).options.length > 1 ? s(n) === s(u) : void 0), H(p, "src", `static/icons/${s(o).icon}.svg`), M(w, s(o).name), M(T, s(o).options.length > 1 ? `${s(_).name} · ${s(_).ext}` : `${s(_).ext} 格式`);
    }), $("click", v, () => r(s(u))), C(f, h);
  }), C(e, a), st();
}
lt(["click"]);
var Ba = /* @__PURE__ */ D("<button><!></button>");
function Vt(e, t) {
  let n = Ue(t, "variant", 3, "secondary"), r = Ue(t, "size", 3, "md"), i = Ue(t, "disabled", 3, !1), a = Ue(t, "type", 3, "button"), l = Ue(t, "class", 3, "");
  var f = Ba(), o = g(f);
  {
    var u = (d) => {
      var _ = Lt(), h = qe(_);
      wa(h, () => t.children), C(d, _);
    };
    se(o, (d) => {
      t.children && d(u);
    });
  }
  U(() => {
    H(f, "type", a()), f.disabled = i(), H(f, "title", t.title), H(f, "aria-label", t["aria-label"] || t.ariaLabel), Re(f, 1, `pixel-btn ${n() ?? ""} ${r() ?? ""} ${l() ?? ""}`, "svelte-snqfwj");
  }), $("click", f, function(...d) {
    t.onclick?.apply(this, d);
  }), C(e, f);
}
lt(["click"]);
const ct = /* @__PURE__ */ new Map(), Ua = 6;
var Ha = /* @__PURE__ */ D("<em> </em>"), ja = /* @__PURE__ */ D('<p class="modal-description"> </p>'), qa = /* @__PURE__ */ D('<a class="pill-btn" target="_blank" rel="noopener">打开文件</a>'), Ga = /* @__PURE__ */ D("<span> </span>"), Ya = /* @__PURE__ */ D('<div class="modal-backdrop" role="presentation"><div class="file-modal" role="dialog" aria-modal="true" aria-labelledby="preview-title"><header class="modal-head"><h2 id="preview-title"> </h2> <button type="button" class="modal-close" aria-label="关闭"><!></button></header> <div class="modal-tags"></div> <!> <div class="modal-meta"><img width="24" height="24" alt="" aria-hidden="true"/> <strong> </strong> <span> </span> <span> </span> <div class="modal-actions"><!> <!></div></div> <div class="preview-shell"><header><span><i></i> FILE PREVIEW</span><span> </span><b> </b></header> <pre></pre></div></div></div>');
function Va(e, t) {
  at(t, !0);
  let n = /* @__PURE__ */ V("loading"), r = /* @__PURE__ */ V("加载中…"), i = /* @__PURE__ */ V("READING"), a = /* @__PURE__ */ V(!1);
  const l = 500;
  function f(c, v) {
    ct.delete(c), ct.set(c, v), ct.size > Ua && ct.delete(ct.keys().next().value);
  }
  mn(() => {
    if (t.item) {
      const c = document.body.style.overflow;
      return document.body.style.overflow = "hidden", () => {
        document.body.style.overflow = c;
      };
    }
  }), mn(() => {
    const c = t.item;
    if (x(a, !1), !c) return;
    if (!c.path) {
      x(n, "empty"), x(r, "没有可预览的内容。"), x(i, "EMPTY");
      return;
    }
    const v = ct.get(c.path);
    if (v) {
      f(c.path, v), x(n, "ready"), x(r, v.text, !0), x(i, v.stat, !0);
      return;
    }
    const y = new AbortController();
    return x(n, "loading"), x(r, "加载中…"), x(i, "READING"), fetch(c.path, { signal: y.signal }).then((p) => {
      if (!p.ok) throw new Error(`http ${p.status}`);
      return p.text();
    }).then((p) => {
      const E = p.split(`
`), k = E.length, w = k > l, S = w ? `${E.slice(0, l).join(`
`)}
… 仅显示前 ${l} 行，完整内容请打开文件` : p, T = `${w ? `${l} / ` : ""}${k} LINES`;
      f(c.path, { text: S, stat: T }), x(n, "ready"), x(r, S, !0), x(i, T);
    }).catch((p) => {
      p instanceof DOMException && p.name === "AbortError" || (x(n, "error"), x(r, "无法加载预览。请通过 prm serve 访问，或点「打开文件」查看。"), x(i, "LOAD ERROR"));
    }), () => y.abort();
  });
  function o(c) {
    c.key === "Escape" && t.item && t.onclose();
  }
  async function u() {
    if (!t.item?.path) return;
    await Ln(t.item.path) && (x(a, !0), setTimeout(
      () => {
        x(a, !1);
      },
      1500
    ));
  }
  var d = Lt();
  ua("keydown", gn, o);
  var _ = qe(d);
  {
    var h = (c) => {
      var v = Ya(), y = g(v), p = g(y), E = g(p), k = g(E), w = b(E, 2), S = g(w);
      mt(S, { name: "cross", size: 12 });
      var T = b(p, 2);
      le(T, 21, () => t.item.tags, it, (m, L) => {
        var q = Ha(), K = g(q);
        U(() => M(K, s(L))), C(m, q);
      });
      var R = b(T, 2);
      {
        var A = (m) => {
          var L = ja(), q = g(L);
          U(() => M(q, t.item.description)), C(m, L);
        };
        se(R, (m) => {
          t.item.description && m(A);
        });
      }
      var B = b(R, 2), j = g(B), I = b(j, 2), J = g(I), ee = b(I, 2), W = g(ee), Y = b(ee, 2), G = g(Y), z = b(Y, 2), P = g(z);
      {
        let m = /* @__PURE__ */ X(() => !t.item.path);
        Vt(P, {
          size: "sm",
          get disabled() {
            return s(m);
          },
          onclick: u,
          children: (L, q) => {
            var K = Gt();
            U(() => M(K, s(a) ? "已复制" : "复制链接")), C(L, K);
          },
          $$slots: { default: !0 }
        });
      }
      var ae = b(P, 2);
      {
        var de = (m) => {
          var L = qa();
          U(() => H(L, "href", t.item.path)), C(m, L);
        };
        se(ae, (m) => {
          t.item.path && m(de);
        });
      }
      var Te = b(B, 2), Le = g(Te), Ne = b(g(Le)), Ce = g(Ne), ot = b(Ne), Et = g(ot), kt = b(Le, 2);
      le(kt, 21, () => s(r).split(`
`), it, (m, L, q) => {
        var K = Ga();
        H(K, "data-line", q + 1);
        var Q = g(K);
        U(() => M(Q, s(L) || " ")), C(m, K);
      }), U(
        (m, L, q) => {
          M(k, t.item.title), H(j, "src", `static/icons/${t.client.icon}.svg`), M(J, t.client.options.length > 1 ? `${t.client.name} · ${t.target.name}` : t.client.name), M(W, m), M(G, `${L ?? ""} 条`), H(Te, "data-state", s(n)), M(Ce, q), M(Et, s(i));
        },
        [
          () => La(t.item.size),
          () => ge(t.item.entries),
          () => t.item.path?.split("/").pop() || "NO FILE"
        ]
      ), $("click", v, (m) => {
        m.target === m.currentTarget && t.onclose();
      }), $("click", w, function(...m) {
        t.onclose?.apply(this, m);
      }), C(c, v);
    };
    se(_, (c) => {
      t.item && c(h);
    });
  }
  C(e, d), st();
}
lt(["click"]);
var Wa = /* @__PURE__ */ D("<em> </em>"), Ka = /* @__PURE__ */ D('<div class="geo-file-row"><span class="geo-tag"> </span><span> </span> <span class="row-actions"><!> <a class="pill-btn" target="_blank" rel="noopener">打开</a></span></div>'), Xa = /* @__PURE__ */ D('<div class="geo-detail"><div class="geo-file-row"><span class="geo-tag full">完整列表</span><span> </span> <span class="row-actions"><!> <a class="pill-btn" target="_blank" rel="noopener">打开</a></span></div> <!></div>'), Za = /* @__PURE__ */ D('<div><div class="geo-row"><button class="geo-toggle" type="button"><!> <strong> </strong></button> <span> </span> <span class="geo-tags"></span> <span class="row-actions"><!> <button class="pill-btn" type="button">复制链接</button> <a class="pill-btn" target="_blank" rel="noopener">打开</a></span></div> <!></div>'), Ja = /* @__PURE__ */ D('<div class="empty">没有匹配的列表。</div>'), Qa = /* @__PURE__ */ D('<button class="more-btn" type="button"> </button>'), $a = /* @__PURE__ */ D('<article class="geo-provider"><header><strong><!> </strong> <span> </span> <span>格式：<b> </b></span></header> <!> <!></article>'), es = /* @__PURE__ */ D('<div class="empty">还没有 Geosite 数据，先运行 prm update。</div>'), ts = /* @__PURE__ */ D('<section class="public-block" aria-labelledby="geosite-heading"><div class="block-head catalog-head"><h2 class="block-title" id="geosite-heading">Geosite 列表</h2> <input class="filter-input" type="search" placeholder="搜索列表…" spellcheck="false"/></div> <div class="geo-root"></div></section>');
function ns(e, t) {
  at(t, !0);
  let n = /* @__PURE__ */ V(""), r = /* @__PURE__ */ V(Fe({})), i = /* @__PURE__ */ V(Fe({}));
  const a = 100;
  function l(c) {
    x(r, { ...s(r), [c]: !s(r)[c] }, !0);
  }
  async function f(c, v) {
    if (await Ln(c)) {
      const p = v.textContent;
      v.textContent = "已复制", setTimeout(
        () => {
          v.textContent = p;
        },
        1500
      );
    }
  }
  function o(c, v, y, p) {
    t.onpreview({
      key: `geosite:${c}/${v}@${p || ""}:${t.target.id}`,
      title: `${v}${p ? ` @${p}` : ""}`,
      tags: ["geosite", c],
      path: Yt(t.target, c, v, p),
      entries: y,
      source: { kind: "geosite", provider: c, name: v, attr: p }
    });
  }
  var u = ts(), d = g(u), _ = b(g(d), 2), h = b(d, 2);
  le(
    h,
    21,
    () => t.catalogs,
    (c) => c.provider,
    (c, v) => {
      const y = /* @__PURE__ */ X(() => s(n).trim().toLowerCase()), p = /* @__PURE__ */ X(() => s(v).lists.filter((z) => !s(y) || z.name.toLowerCase().includes(s(y)))), E = /* @__PURE__ */ X(() => s(i)[s(v).provider] ? s(p) : s(p).slice(0, a)), k = /* @__PURE__ */ X(() => s(v).lists.reduce((z, P) => z + P.variants.length, 0));
      var w = $a(), S = g(w), T = g(S), R = g(T);
      mt(R, { name: "globe", size: 18 });
      var A = b(R), B = b(T, 2), j = g(B), I = b(B, 2), J = b(g(I)), ee = g(J), W = b(S, 2);
      le(
        W,
        17,
        () => s(E),
        (z) => z.name,
        (z, P) => {
          const ae = /* @__PURE__ */ X(() => `${s(v).provider}/${s(P).name}`), de = /* @__PURE__ */ X(() => Yt(t.target, s(v).provider, s(P).name));
          var Te = Za();
          let Le;
          var Ne = g(Te), Ce = g(Ne), ot = g(Ce);
          {
            let ve = /* @__PURE__ */ X(() => s(r)[s(ae)] ? "chevron-down" : "chevron-right");
            mt(ot, {
              get name() {
                return s(ve);
              },
              size: 9
            });
          }
          var Et = b(ot, 2), kt = g(Et), m = b(Ce, 2), L = g(m), q = b(m, 2);
          le(q, 21, () => s(P).variants.slice(0, 6), it, (ve, xt) => {
            var Ke = Wa(), Bt = g(Ke);
            U(() => M(Bt, `@${s(xt).attr ?? ""}`)), C(ve, Ke);
          });
          var K = b(q, 2), Q = g(K);
          Vt(Q, {
            size: "sm",
            onclick: () => o(s(v).provider, s(P).name, s(P).entries),
            children: (ve, xt) => {
              var Ke = Gt("预览");
              C(ve, Ke);
            },
            $$slots: { default: !0 }
          });
          var We = b(Q, 2), rn = b(We, 2), Hr = b(Ne, 2);
          {
            var jr = (ve) => {
              var xt = Xa(), Ke = g(xt), Bt = b(g(Ke)), qr = g(Bt), Gr = b(Bt, 2), Nn = g(Gr);
              Vt(Nn, {
                size: "sm",
                onclick: () => o(s(v).provider, s(P).name, s(P).entries),
                children: (Xe, ft) => {
                  var an = Gt("预览");
                  C(Xe, an);
                },
                $$slots: { default: !0 }
              });
              var Yr = b(Nn, 2), Vr = b(Ke, 2);
              le(Vr, 17, () => s(P).variants, (Xe) => Xe.attr, (Xe, ft) => {
                const an = /* @__PURE__ */ X(() => Yt(t.target, s(v).provider, s(P).name, s(ft).attr));
                var Pn = Ka(), On = g(Pn), Wr = g(On), Fn = b(On), Kr = g(Fn), Xr = b(Fn, 2), Dn = g(Xr);
                Vt(Dn, {
                  size: "sm",
                  onclick: () => o(s(v).provider, s(P).name, s(ft).entries, s(ft).attr),
                  children: (sn, ws) => {
                    var Jr = Gt("预览");
                    C(sn, Jr);
                  },
                  $$slots: { default: !0 }
                });
                var Zr = b(Dn, 2);
                U(
                  (sn) => {
                    M(Wr, `@${s(ft).attr ?? ""}`), M(Kr, `${sn ?? ""} 条`), H(Zr, "href", s(an));
                  },
                  [() => ge(s(ft).entries)]
                ), C(Xe, Pn);
              }), U(
                (Xe) => {
                  M(qr, `${Xe ?? ""} 条`), H(Yr, "href", s(de));
                },
                [() => ge(s(P).entries)]
              ), C(ve, xt);
            };
            se(Hr, (ve) => {
              s(r)[s(ae)] && ve(jr);
            });
          }
          U(
            (ve) => {
              Le = Re(Te, 1, "geo-item", null, Le, { open: s(r)[s(ae)] }), H(Ce, "aria-expanded", !!s(r)[s(ae)]), M(kt, s(P).name), M(L, `${ve ?? ""} 条`), H(rn, "href", s(de));
            },
            [() => ge(s(P).entries)]
          ), $("click", Ce, () => l(s(ae))), $("click", We, (ve) => f(s(de), ve.currentTarget)), C(z, Te);
        },
        (z) => {
          var P = Ja();
          C(z, P);
        }
      );
      var Y = b(W, 2);
      {
        var G = (z) => {
          var P = Qa(), ae = g(P);
          U((de) => M(ae, `显示全部 ${de ?? ""} 个`), [() => ge(s(p).length)]), $("click", P, () => {
            x(i, { ...s(i), [s(v).provider]: !0 }, !0);
          }), C(z, P);
        };
        se(Y, (z) => {
          !s(i)[s(v).provider] && s(p).length > a && z(G);
        });
      }
      U(
        (z, P) => {
          M(A, ` ${s(v).provider ?? ""}`), M(j, `${z ?? ""} 个列表 · ${P ?? ""} 个属性变体`), M(ee, `${t.client.name ?? ""} · ${t.target.name ?? ""}`);
        },
        [
          () => ge(s(v).lists.length),
          () => ge(s(k))
        ]
      ), C(c, w);
    },
    (c) => {
      var v = es();
      C(c, v);
    }
  ), Br(_, () => s(n), (c) => x(n, c)), C(e, u), st();
}
lt(["click"]);
var rs = /* @__PURE__ */ D('<article class="icon-card"><img loading="lazy"/> <strong> </strong> <span><button type="button">复制</button> <a>下载</a></span></article>'), is = /* @__PURE__ */ D('<button class="more-btn" type="button"> </button>'), as = /* @__PURE__ */ D('<button class="icon-back" type="button">［ 返回图标集 ］</button> <div class="block-head"><h2 class="block-title" id="icons-heading"> <span class="count"> </span></h2></div> <div class="icon-grid"></div> <!>', 1), ss = /* @__PURE__ */ D('<button class="icon-set-card" type="button"><!> <span><strong> </strong><small> </small></span></button>'), ls = /* @__PURE__ */ D('<div class="empty">还没有图标集。</div>'), os = /* @__PURE__ */ D('<div class="block-head"><h2 class="block-title" id="icons-heading">图标集 <span class="count"> </span></h2></div> <div class="icon-sets"></div>', 1), fs = /* @__PURE__ */ D('<section class="public-block" aria-labelledby="icons-heading"><!></section>');
function us(e, t) {
  at(t, !0);
  let n = /* @__PURE__ */ V(null), r = /* @__PURE__ */ V(60);
  function i(d) {
    x(n, d, !0), x(r, Math.min(60, d.icons.length), !0);
  }
  async function a(d, _) {
    if (await Ln(d)) {
      const c = _.textContent;
      _.textContent = "已复制", setTimeout(
        () => {
          _.textContent = c;
        },
        1500
      );
    }
  }
  var l = fs(), f = g(l);
  {
    var o = (d) => {
      var _ = as(), h = qe(_), c = b(h, 2), v = g(c), y = g(v), p = b(y), E = g(p), k = b(c, 2);
      le(k, 20, () => s(n).icons.slice(0, s(r)), (T) => T, (T, R) => {
        const A = /* @__PURE__ */ X(() => Ia(s(n).name, R));
        var B = rs(), j = g(B), I = b(j, 2), J = g(I), ee = b(I, 2), W = g(ee), Y = b(W, 2);
        U(
          (G, z) => {
            H(j, "src", s(A)), H(j, "alt", G), H(I, "title", R), M(J, z), H(Y, "href", s(A)), H(Y, "download", R);
          },
          [
            () => R.replace(/\.[^.]+$/, ""),
            () => R.replace(/\.[^.]+$/, "")
          ]
        ), $("click", W, (G) => a(s(A), G.currentTarget)), C(T, B);
      });
      var w = b(k, 2);
      {
        var S = (T) => {
          var R = is(), A = g(R);
          U((B, j) => M(A, `加载更多（${B ?? ""} / ${j ?? ""}）`), [
            () => ge(s(r)),
            () => ge(s(n).count)
          ]), $("click", R, () => {
            x(r, Math.min(s(r) + 60, s(n).icons.length), !0);
          }), C(T, R);
        };
        se(w, (T) => {
          s(r) < s(n).icons.length && T(S);
        });
      }
      U(
        (T) => {
          M(y, `${s(n).name ?? ""} `), M(E, T);
        },
        [() => ge(s(n).count)]
      ), $("click", h, () => {
        x(n, null);
      }), C(d, _);
    }, u = (d) => {
      var _ = os(), h = qe(_), c = g(h), v = b(g(c)), y = g(v), p = b(h, 2);
      le(
        p,
        21,
        () => t.sets,
        (E) => E.name,
        (E, k) => {
          var w = ss(), S = g(w);
          mt(S, { name: "iconset", size: 26 });
          var T = b(S, 2), R = g(T), A = g(R), B = b(R), j = g(B);
          U(
            (I) => {
              M(A, s(k).name), M(j, `${I ?? ""} 个图标`);
            },
            [() => ge(s(k).count)]
          ), $("click", w, () => i(s(k))), C(E, w);
        },
        (E) => {
          var k = ls();
          C(E, k);
        }
      ), U(() => M(y, t.sets.length)), C(d, _);
    };
    se(f, (d) => {
      s(n) ? d(o) : d(u, -1);
    });
  }
  C(e, l), st();
}
lt(["click"]);
var cs = /* @__PURE__ */ D('<button type="button"> </button>'), ds = /* @__PURE__ */ D("<em> </em>"), vs = /* @__PURE__ */ D('<tr class="rule-row" tabindex="0" role="button"><th scope="row"><button type="button" class="rule-name-btn"> </button></th><td><span class="rtags"></span></td><td class="num"> </td><td class="action"><button type="button" class="action-btn"><!></button></td></tr>'), hs = /* @__PURE__ */ D('<tr><td colspan="4" class="empty">没有匹配的规则。</td></tr>'), _s = /* @__PURE__ */ D('<section class="public-block" aria-labelledby="rules-heading"><div class="block-head catalog-head"><h2 class="block-title" id="rules-heading">规则 <span class="count"> </span></h2> <input class="filter-input" type="search" placeholder="搜索规则…" spellcheck="false"/></div> <div class="chips" aria-label="规则标签"></div> <div class="table-scroll" role="region" aria-labelledby="rules-heading"><table class="data-table rule-table"><thead><tr><th>名称</th><th>标签</th><th class="num">条目</th><th class="action">查看</th></tr></thead><tbody></tbody></table></div> <div class="target-hint"> </div></section>');
function ps(e, t) {
  at(t, !0);
  let n = /* @__PURE__ */ V(""), r = /* @__PURE__ */ V(Fe([]));
  const i = /* @__PURE__ */ X(() => {
    const k = s(n).trim().toLowerCase();
    return t.rules.filter((w) => {
      const S = !k || w.name.toLowerCase().includes(k) || w.id.toLowerCase().includes(k), T = w.tags.map((A) => A.toLowerCase()), R = s(r).length === 0 || s(r).some((A) => T.includes(A));
      return S && R;
    });
  });
  function a(k) {
    const w = k.toLowerCase();
    x(
      r,
      s(r).includes(w) ? s(r).filter((S) => S !== w) : [...s(r), w],
      !0
    );
  }
  var l = _s(), f = g(l), o = g(f), u = b(g(o)), d = g(u), _ = b(o, 2), h = b(f, 2);
  le(h, 21, () => t.tags, it, (k, w) => {
    var S = cs();
    let T;
    var R = g(S);
    U(
      (A) => {
        T = Re(S, 1, "", null, T, A), M(R, s(w));
      },
      [
        () => ({ on: s(r).includes(s(w).toLowerCase()) })
      ]
    ), $("click", S, () => a(s(w))), C(k, S);
  });
  var c = b(h, 2), v = g(c), y = b(g(v));
  le(
    y,
    21,
    () => s(i),
    (k) => k.id,
    (k, w) => {
      var S = vs(), T = g(S), R = g(T), A = g(R), B = b(T), j = g(B);
      le(j, 21, () => s(w).tags, it, (G, z) => {
        var P = ds(), ae = g(P);
        U(() => M(ae, s(z))), C(G, P);
      });
      var I = b(B), J = g(I), ee = b(I), W = g(ee), Y = g(W);
      mt(Y, { name: "chevron-right", size: 10 }), U(
        (G) => {
          H(S, "aria-label", `查看规则 ${s(w).name ?? ""}`), M(A, s(w).name), M(J, G), H(W, "aria-label", `预览 ${s(w).name ?? ""}`);
        },
        [() => ge(s(w).entries)]
      ), $("click", S, () => t.onpreview(s(w))), $("keydown", S, (G) => {
        (G.key === "Enter" || G.key === " ") && (G.preventDefault(), t.onpreview(s(w)));
      }), $("click", R, (G) => {
        G.stopPropagation(), t.onpreview(s(w));
      }), $("click", W, (G) => {
        G.stopPropagation(), t.onpreview(s(w));
      }), C(k, S);
    },
    (k) => {
      var w = hs();
      C(k, w);
    }
  );
  var p = b(c, 2), E = g(p);
  U(() => {
    M(d, `${s(i).length ?? ""} / ${t.rules.length ?? ""}`), M(E, `当前格式：${t.target.name ?? ""} · ${t.target.ext ?? ""}`);
  }), Br(_, () => s(n), (k) => x(n, k)), C(e, l), st();
}
lt(["click", "keydown"]);
var gs = /* @__PURE__ */ D('<a class="bracket-btn">［ 管理 ］</a>'), ms = /* @__PURE__ */ D('<div class="public-app"><header class="public-topbar"><div class="public-brand"><img src="static/icons/prm.svg" width="40" height="40" alt="PRM"/> <div><strong>PROXY RULE MANAGER</strong><small> </small></div></div> <div class="top-actions"><!> <button class="bracket-btn" type="button"> </button></div></header> <nav class="view-switch" aria-label="内容视图"><button type="button">规则</button> <button type="button">Geosite</button> <button type="button">图标</button></nav> <!> <!></div> <!>', 1);
function bs(e, t) {
  at(t, !0);
  let n = /* @__PURE__ */ V("rules"), r = /* @__PURE__ */ V(0), i = /* @__PURE__ */ V(Fe({})), a = /* @__PURE__ */ V("dark"), l = /* @__PURE__ */ V(null);
  const f = /* @__PURE__ */ X(() => t.data.clients[s(r)]), o = /* @__PURE__ */ X(() => s(f) ? En(s(f), s(i)[s(f).id], s(n)) : void 0);
  Ta(() => {
    try {
      const m = localStorage.getItem("prm-theme");
      (m === "dark" || m === "light") && x(a, m, !0);
      const L = Number.parseInt(localStorage.getItem("prm-client") || "", 10);
      Number.isInteger(L) && L >= 0 && L < t.data.clients.length && x(r, L, !0);
      const q = {};
      for (const K of t.data.clients) {
        const Q = localStorage.getItem(`prm-target-${K.id}`);
        Q && (q[K.id] = Q);
      }
      x(i, q, !0), u(s(n));
    } catch {
      u(s(n));
    }
  });
  function u(m) {
    if (m === "icons" || t.data.clients.length === 0) return;
    let L = s(r);
    if (!$t(t.data.clients[L], m)) {
      const Q = t.data.clients.findIndex((We) => $t(We, m));
      Q >= 0 && (L = Q);
    }
    x(r, L, !0);
    const q = t.data.clients[L], K = En(q, s(i)[q.id], m);
    x(i, { ...s(i), [q.id]: K.id }, !0);
    try {
      localStorage.setItem("prm-client", String(L)), localStorage.setItem(`prm-target-${q.id}`, K.id);
    } catch {
    }
  }
  function d(m) {
    x(n, m, !0), x(l, null), u(m);
  }
  function _(m) {
    x(r, m, !0), u(s(n));
  }
  function h(m, L) {
    const q = t.data.clients[m], K = q.options.find((Q) => Q.id === L);
    if (K) {
      x(r, m, !0), x(i, { ...s(i), [q.id]: L }, !0);
      try {
        localStorage.setItem("prm-client", String(m)), localStorage.setItem(`prm-target-${q.id}`, L);
      } catch {
      }
      if (s(l)?.source.kind === "rule") {
        const Q = s(l).source, We = t.data.rules.find((rn) => rn.id === Q.rule_id);
        We && x(l, c(We, K), !0);
      } else if (s(l)?.source.kind === "geosite") {
        const Q = s(l).source;
        x(
          l,
          {
            ...s(l),
            key: `geosite:${Q.provider}/${Q.name}@${Q.attr || ""}:${K.id}`,
            path: Yt(K, Q.provider, Q.name, Q.attr)
          },
          !0
        );
      }
    }
  }
  function c(m, L) {
    const q = m.files.find((K) => K.target_id === L.id);
    return {
      key: `rule:${m.id}:${L.id}`,
      title: m.name,
      tags: m.tags,
      description: m.description,
      path: q?.path,
      size: q?.size,
      entries: m.entries,
      source: { kind: "rule", rule_id: m.id }
    };
  }
  function v(m) {
    s(o) && x(l, c(m, s(o)), !0);
  }
  function y() {
    x(a, s(a) === "dark" ? "light" : "dark", !0), document.documentElement.classList.add("theme-switching"), document.documentElement.setAttribute("data-theme", s(a));
    try {
      localStorage.setItem("prm-theme", s(a));
    } catch {
    }
    requestAnimationFrame(() => requestAnimationFrame(() => document.documentElement.classList.remove("theme-switching")));
  }
  var p = ms(), E = qe(p), k = g(E), w = g(k), S = b(g(w), 2), T = b(g(S)), R = g(T), A = b(w, 2), B = g(A);
  {
    var j = (m) => {
      var L = gs();
      U(() => H(L, "href", t.data.admin_url)), C(m, L);
    };
    se(B, (m) => {
      t.data.admin_url && m(j);
    });
  }
  var I = b(B, 2), J = g(I), ee = b(k, 2), W = g(ee);
  let Y;
  var G = b(W, 2);
  let z;
  var P = b(G, 2);
  let ae;
  var de = b(ee, 2);
  {
    var Te = (m) => {
      za(m, {
        get clients() {
          return t.data.clients;
        },
        get view() {
          return s(n);
        },
        get selectedIndex() {
          return s(r);
        },
        get selectedTargets() {
          return s(i);
        },
        onselectclient: _,
        onselecttarget: h
      });
    };
    se(de, (m) => {
      s(n) !== "icons" && s(f) && s(o) && m(Te);
    });
  }
  var Le = b(de, 2);
  {
    var Ne = (m) => {
      ps(m, {
        get rules() {
          return t.data.rules;
        },
        get tags() {
          return t.data.tags;
        },
        get target() {
          return s(o);
        },
        onpreview: v
      });
    }, Ce = (m) => {
      ns(m, {
        get catalogs() {
          return t.data.geosite;
        },
        get client() {
          return s(f);
        },
        get target() {
          return s(o);
        },
        onpreview: (L) => {
          x(l, L, !0);
        }
      });
    }, ot = (m) => {
      us(m, {
        get sets() {
          return t.data.icon_sets;
        }
      });
    };
    se(Le, (m) => {
      s(n) === "rules" && s(f) && s(o) ? m(Ne) : s(n) === "geosite" && s(f) && s(o) ? m(Ce, 1) : s(n) === "icons" && m(ot, 2);
    });
  }
  var Et = b(E, 2);
  {
    var kt = (m) => {
      Va(m, {
        get item() {
          return s(l);
        },
        get client() {
          return s(f);
        },
        get target() {
          return s(o);
        },
        onclose: () => {
          x(l, null);
        }
      });
    };
    se(Et, (m) => {
      s(l) && s(f) && s(o) && m(kt);
    });
  }
  U(
    (m) => {
      M(R, `规则索引 · 更新于 ${m ?? ""}`), M(J, `［ ${s(a) === "dark" ? "亮色" : "暗色"} ］`), Y = Re(W, 1, "", null, Y, { on: s(n) === "rules" }), z = Re(G, 1, "", null, z, { on: s(n) === "geosite" }), ae = Re(P, 1, "", null, ae, { on: s(n) === "icons" });
    },
    [() => Na(t.data.updated_at)]
  ), $("click", I, y), $("click", W, () => d("rules")), $("click", G, () => d("geosite")), $("click", P, () => d("icons")), C(e, p), st();
}
lt(["click"]);
const Es = ha(bs, {
  target: document.getElementById("public-app"),
  props: { data: Ra() }
});
export {
  Es as default
};
