// util.js
(() => {
  const U = {};

  U.debounce = function debounce(fn, delayMs) {
    let t = null;
    return (...args) => {
      if (t) clearTimeout(t);
      t = setTimeout(() => fn(...args), delayMs);
    };
  };

  U.fmtNumber = function fmtNumber(n) {
    if (!Number.isFinite(n)) return String(n);
    const abs = Math.abs(n);
    let s;
    if (abs >= 1000) s = n.toFixed(0);
    else if (abs >= 100) s = n.toFixed(1);
    else if (abs >= 10) s = n.toFixed(2);
    else s = n.toFixed(3);
    return s.replace(/\.?0+$/, '');
  };

  // Minimal “unit guessing” (safe default). Later we can drive this from DumpConfig ValueFormat.
  U.withUnit = function withUnit(paramName, rawValue) {
    if (typeof rawValue === 'string' && rawValue.startsWith('"')) {
      return rawValue.replace(/^"|"$/g, '');
    }

    const n = Number(rawValue);
    if (!Number.isFinite(n)) return String(rawValue);

    if (/Thresh|Gain|Level|Volume/i.test(paramName)) return `${U.fmtNumber(n)} dB`;
    if (/Freq|High|Low|Tone/i.test(paramName)) {
      if (n >= 1000) return `${U.fmtNumber(n / 1000)} kHz`;
      return `${U.fmtNumber(n)} Hz`;
    }
    return U.fmtNumber(n);
  };

  window.NAMNESIS = window.NAMNESIS || {};
  window.NAMNESIS.util = U;
})();
