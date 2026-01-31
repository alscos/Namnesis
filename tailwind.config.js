module.exports = {
  content: [
    "./web/templates/**/*.{html,tmpl}",
    "./web/static/**/*.{js,html}",
  ],
  safelist: [
    // core layout primitives so UI never “collapses”
    "flex", "grid", "hidden", "block",
    "items-center", "justify-between", "justify-center",
    "w-full", "h-full", "min-h-screen",
    "gap-2", "gap-3", "gap-4", "gap-6",
    "p-2", "p-3", "p-4", "p-6",
    "m-2", "m-3", "m-4",
    "rounded", "rounded-lg", "rounded-xl", "rounded-2xl",
    "border", "border-white/10",
    "bg-black", "bg-zinc-900", "bg-zinc-800",
    "text-white", "text-zinc-200", "text-zinc-400",
  ],
  theme: { extend: {} },
  plugins: [],
};

