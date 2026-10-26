import json, os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import palette as p

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..'))
S = {n: {str(k): v for k, v in s.items()} for n, s in p.SCALES.items()}
TOKENS_CSS = open(os.path.join(HERE, '.build', 'tokens.css'), encoding='utf-8').read()
OUT = os.path.join(ROOT, 'docs', 'design', 'styleguide.html')

ICONS = {
    'clock': '<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>',
    'check': '<polyline points="20 6 9 17 4 12"/>',
    'flame': '<path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.07-2.14-.22-4.05 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.15.43-2.29 1-3a2.5 2.5 0 0 0 2.5 2.5z"/>',
    'bell': '<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>',
    'utensils': '<path d="M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2"/><path d="M7 2v20"/><path d="M21 15V2a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3zm0 0v7"/>',
    'x': '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
    'ban': '<circle cx="12" cy="12" r="10"/><path d="m4.9 4.9 14.2 14.2"/>',
    'alert': '<circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/>',
    'timer': '<line x1="10" x2="14" y1="2" y2="2"/><line x1="12" x2="15" y1="14" y2="11"/><circle cx="12" cy="14" r="8"/>',
    'plus': '<path d="M5 12h14"/><path d="M12 5v14"/>',
    'milk': '<path d="M8 2h8"/><path d="M9 2v2.8a4 4 0 0 1-.7 2.2l-.6.9A4 4 0 0 0 7 10.2V20a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2v-9.8a4 4 0 0 0-.7-2.2l-.6-.9A4 4 0 0 1 15 4.8V2"/><path d="M7 15a6.47 6.47 0 0 1 5 0 6.47 6.47 0 0 0 5 0"/>',
}


def icon(name, size=16):
    return (f'<svg class="ic" width="{size}" height="{size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" '
            f'stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">{ICONS[name]}</svg>')


def swatch_rows():
    rows = []
    for name, scale in S.items():
        cells = []
        for step, hexv in scale.items():
            fg = '#FFFFFF' if p.contrast('#FFFFFF', hexv) >= p.contrast('#1E2B25', hexv) else '#1E2B25'
            cells.append(f'<div class="sw" style="background:{hexv};color:{fg}"><b>{step}</b><span>{hexv}</span></div>')
        rows.append(f'<div class="scale"><div class="scale-name">{name}</div><div class="scale-cells">{"".join(cells)}</div></div>')
    return '\n'.join(rows)


STATUS = [('pending', 'Menunggu', 'clock'), ('confirmed', 'Dikonfirmasi', 'check'), ('preparing', 'Disiapkan', 'flame'),
          ('ready', 'Siap', 'bell'), ('served', 'Disajikan', 'utensils'), ('cancelled', 'Dibatalkan', 'x'), ('voided', 'Void', 'ban')]


def chips():
    out = []
    for key, label, ic in STATUS:
        cls = ' strike' if key == 'cancelled' else ''
        out.append(f'<span class="chip" style="background:var(--sl-color-status-{key}-bg);color:var(--sl-color-status-{key}-fg);'
                   f'border-color:var(--sl-color-status-{key}-border)">{icon(ic, 14)}<span class="{cls.strip()}">{label}</span></span>')
    out.append('<span class="chip" style="background:var(--sl-color-dietary-warning-bg);color:var(--sl-color-dietary-warning-fg);'
               f'border-color:var(--sl-color-dietary-warning-border)">{icon("milk", 14)}Mengandung susu</span>')
    return '\n'.join(out)


TYPE_ROWS = [
    ('display', 'Poppins 700', 'font-family:var(--sl-font-family-display);font-weight:700;font-size:32px;line-height:1.1;letter-spacing:-0.02em', 'Mau makan apa hari ini?'),
    ('title', 'Poppins 600', 'font-family:var(--sl-font-family-display);font-weight:600;font-size:24px;line-height:1.25', 'Sushi &amp; Nigiri'),
    ('heading', 'Jakarta 700', 'font-family:var(--sl-font-family-ui);font-weight:700;font-size:20px;line-height:1.25', 'Meja 5 · 3 item'),
    ('body', 'Jakarta 400', 'font-family:var(--sl-font-family-ui);font-size:16px;line-height:1.5', 'Salmon segar di atas nasi cuka hangat, disajikan dengan wasabi dan jahe.'),
    ('label', 'Jakarta 600', 'font-family:var(--sl-font-family-ui);font-weight:600;font-size:14px', 'Tambah ke pesanan'),
    ('caption', 'Jakarta 500', 'font-family:var(--sl-font-family-ui);font-weight:500;font-size:12px', '2 mnt lalu · Andi'),
    ('overline', 'Jakarta 700 caps', 'font-family:var(--sl-font-family-ui);font-weight:700;font-size:12px;letter-spacing:0.12em;text-transform:uppercase', 'Sushi &amp; Coffee'),
    ('price', 'Jakarta 700 tabular', 'font-family:var(--sl-font-family-ui);font-weight:700;font-size:20px;font-variant-numeric:tabular-nums;color:var(--sl-color-text-brand)', 'Rp 38.000'),
    ('amount', 'Jakarta 800 tabular', 'font-family:var(--sl-font-family-ui);font-weight:800;font-size:32px;font-variant-numeric:tabular-nums', 'Rp 186.450'),
    ('code', 'JetBrains Mono 600', 'font-family:var(--sl-font-family-mono);font-weight:600;font-size:14px', 'B-0427 · W001'),
]


def type_rows():
    return '\n'.join(f'<div class="type-row"><div class="type-meta"><b>{n}</b><span>{m}</span></div><div style="{s}">{t}</div></div>'
                     for n, m, s, t in TYPE_ROWS)


SPACE = [('1', 4), ('2', 8), ('3', 12), ('4', 16), ('5', 20), ('6', 24), ('8', 32), ('10', 40), ('12', 48), ('16', 64)]
RADIUS = [('xs', 4), ('sm', 8), ('md', 10), ('lg', 12), ('xl', 20), ('2xl', 28), ('pill', 9999)]
EASING = [('standard', '0.2, 0, 0, 1'), ('decelerate', '0.05, 0.7, 0.1, 1'), ('accelerate', '0.3, 0, 0.8, 0.15'),
          ('expressive', '0.22, 1, 0.36, 1'), ('overshoot', '0.34, 1.56, 0.64, 1')]

html = f'''<!doctype html>
<html lang="id" data-theme="light">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sushi Latte Styleguide</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&family=Plus+Jakarta+Sans:wght@400;500;600;700;800&family=Poppins:wght@600;700&display=swap" rel="stylesheet">
<style>
/* Generated from docs/design/tokens/*.json — do not edit token values here. */
{TOKENS_CSS}
@media (prefers-color-scheme: dark) {{ :root:not([data-theme="light"]) {{ color-scheme: dark; }} }}
:root[data-theme="dark"] {{ color-scheme: dark; }}
* {{ box-sizing: border-box; }}
body {{ margin: 0; background: var(--sl-color-bg-canvasWarm); color: var(--sl-color-text-primary);
  font-family: var(--sl-font-family-ui); font-size: 16px; line-height: 1.5; }}
.wrap {{ max-width: 1120px; margin: 0 auto; padding: 0 16px 96px; }}
header.top {{ position: sticky; top: 0; z-index: 20; background: var(--sl-color-bg-canvasWarm); border-bottom: 1px solid var(--sl-color-border-subtle); }}
header.top .wrap {{ display: flex; align-items: center; justify-content: space-between; gap: 16px; padding-top: 12px; padding-bottom: 12px; }}
header.top img {{ height: 56px; }}
nav.toc {{ display: flex; gap: 6px; flex-wrap: wrap; }}
nav.toc a {{ color: var(--sl-color-text-secondary); text-decoration: none; font-size: 13px; font-weight: 600; padding: 6px 10px; border-radius: 999px; }}
nav.toc a:hover {{ background: var(--sl-color-action-secondary-bg); color: var(--sl-color-text-primary); }}
.toggle {{ font: inherit; font-weight: 600; font-size: 14px; border: 1px solid var(--sl-color-border-input); background: var(--sl-color-bg-surface);
  color: var(--sl-color-text-primary); border-radius: 999px; padding: 8px 14px; cursor: pointer; min-height: 40px; }}
.hero {{ padding: 48px 0 24px; display: grid; grid-template-columns: 1fr; gap: 24px; align-items: center; }}
.hero h1 {{ font-family: var(--sl-font-family-display); font-size: clamp(32px, 6vw, 56px); line-height: 1.05; letter-spacing: -0.02em; margin: 0 0 12px; }}
.hero p {{ color: var(--sl-color-text-secondary); max-width: 60ch; margin: 0; }}
.hero img {{ width: min(100%, 560px); }}
section {{ padding-top: 48px; }}
h2 {{ font-family: var(--sl-font-family-display); font-size: 28px; margin: 0 0 4px; }}
h3 {{ font-size: 16px; margin: 24px 0 12px; }}
.lead {{ color: var(--sl-color-text-secondary); margin: 0 0 20px; }}
.overline {{ font-size: 12px; font-weight: 700; letter-spacing: .12em; text-transform: uppercase; color: var(--sl-color-brand-salmonDeep); }}
.grid {{ display: grid; gap: 16px; }}
.card {{ background: var(--sl-color-bg-surface); border: 1px solid var(--sl-color-border-subtle); border-radius: 20px; padding: 20px; }}
.scale {{ display: grid; grid-template-columns: 90px 1fr; gap: 12px; align-items: center; margin-bottom: 8px; }}
.scale-name {{ font-weight: 700; font-size: 14px; text-transform: capitalize; }}
.scale-cells {{ display: grid; grid-template-columns: repeat(11, minmax(0, 1fr)); border-radius: 12px; overflow: hidden; }}
.sw {{ padding: 10px 6px; min-height: 64px; display: flex; flex-direction: column; justify-content: flex-end; font-size: 10px; line-height: 1.3; }}
.sw b {{ font-size: 12px; }}
.sw span {{ font-family: var(--sl-font-family-mono); }}
@media (max-width: 720px) {{ .scale {{ grid-template-columns: 1fr; gap: 4px; }} .sw span {{ display: none; }} .sw {{ min-height: 44px; }} }}
.chips {{ display: flex; gap: 8px; flex-wrap: wrap; }}
.chip {{ display: inline-flex; align-items: center; gap: 6px; height: 28px; padding: 0 12px; border-radius: 999px; border: 1px solid; font-size: 13px; font-weight: 600; }}
.strike {{ text-decoration: line-through; }}
.ic {{ flex: none; }}
.dark-panel {{ background: #12221B; color: #FAF6F1; border-radius: 20px; padding: 20px; }}
.type-row {{ display: grid; grid-template-columns: 160px 1fr; gap: 16px; padding: 14px 0; border-bottom: 1px solid var(--sl-color-border-subtle); align-items: baseline; }}
.type-meta {{ display: flex; flex-direction: column; font-size: 12px; color: var(--sl-color-text-muted); }}
.type-meta b {{ color: var(--sl-color-text-primary); font-size: 14px; }}
@media (max-width: 720px) {{ .type-row {{ grid-template-columns: 1fr; gap: 4px; }} }}
.space-row {{ display: flex; align-items: center; gap: 12px; font-size: 13px; margin-bottom: 6px; }}
.space-row .bar {{ height: 12px; background: var(--sl-color-brand-salmon); border-radius: 3px; }}
.space-row code, .mono {{ font-family: var(--sl-font-family-mono); font-size: 12px; color: var(--sl-color-text-secondary); min-width: 84px; }}
.radius-grid {{ display: grid; grid-template-columns: repeat(auto-fill, minmax(110px, 1fr)); gap: 12px; }}
.radius-box {{ height: 80px; background: var(--sl-color-bg-surface); border: 2px solid var(--sl-color-border-strong); display: grid; place-items: center; font-size: 12px; font-weight: 600; }}
.shadow-grid {{ display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 24px; }}
.shadow-box {{ height: 96px; border-radius: 16px; background: var(--sl-color-bg-surface); display: grid; place-items: center; font-size: 13px; font-weight: 600; }}
.btn {{ font: inherit; font-weight: 600; font-size: 15px; display: inline-flex; align-items: center; gap: 8px; border: 1px solid transparent; cursor: pointer;
  min-height: 44px; padding: 0 18px; transition: transform 80ms, background-color 150ms; }}
.btn:active {{ transform: scale(.98); }}
.btn:focus-visible, .toggle:focus-visible, a:focus-visible {{ outline: 2px solid var(--sl-color-border-focus); outline-offset: 2px; }}
.btn.brand {{ background: var(--sl-color-action-brand-bg); color: var(--sl-color-action-brand-fg); border-radius: 999px; }}
.btn.brand:hover {{ background: var(--sl-color-action-brand-bgHover); }}
.btn.neutral {{ background: var(--sl-color-action-neutral-bg); color: var(--sl-color-action-neutral-fg); border-radius: 10px; }}
.btn.neutral:hover {{ background: var(--sl-color-action-neutral-bgHover); }}
.btn.secondary {{ background: var(--sl-color-action-secondary-bg); color: var(--sl-color-action-secondary-fg); border-color: var(--sl-color-action-secondary-border); border-radius: 10px; }}
.btn.danger {{ background: var(--sl-color-action-danger-bg); color: var(--sl-color-action-danger-fg); border-radius: 10px; }}
.btn-row {{ display: flex; gap: 12px; flex-wrap: wrap; align-items: center; }}
.product {{ width: 220px; background: var(--sl-color-bg-surface); border-radius: 20px; overflow: hidden; box-shadow: var(--sl-shadow-md); }}
.product .img {{ aspect-ratio: 1/1; background: var(--sl-color-rice-100); display: grid; place-items: center; position: relative; overflow: hidden; }}
.product .img img {{ width: 100%; height: 100%; object-fit: cover; }}
.product .img img.ph {{ width: 62%; height: auto; }}
.product .body {{ padding: 12px 14px 14px; }}
.product .name {{ font-weight: 600; }}
.product .desc {{ font-size: 12px; color: var(--sl-color-text-muted); margin: 2px 0 10px; }}
.product .foot {{ display: flex; justify-content: space-between; align-items: center; }}
.product .price {{ font-weight: 700; font-size: 18px; color: var(--sl-color-text-brand); font-variant-numeric: tabular-nums; }}
.add {{ width: 44px; height: 44px; border-radius: 999px; border: 0; background: var(--sl-color-action-brand-bg); color: var(--sl-color-action-brand-fg); display: grid; place-items: center; cursor: pointer; }}
.badge-tl {{ position: absolute; top: 10px; left: 10px; }}
.product.soldout .img img {{ filter: grayscale(1); opacity: .45; }}
.soldout-label {{ position: absolute; inset: auto 10px 10px auto; background: var(--sl-color-soldOut-badgeBg); color: var(--sl-color-soldOut-badgeFg); font-size: 12px; font-weight: 700; padding: 4px 10px; border-radius: 999px; }}
.products {{ display: flex; gap: 16px; flex-wrap: wrap; }}
.ticket {{ width: 340px; border-radius: 8px; overflow: hidden; background: #1E2B25; border: 3px solid #4E5A55; font-size: 20px; }}
.ticket.late {{ border-color: #FF6F63; }}
.ticket .head {{ padding: 12px 16px; display: flex; flex-wrap: wrap; gap: 4px 12px; justify-content: space-between; font-weight: 800; }}
.pin button:empty {{ visibility: hidden; }}
@media (min-width: 900px) {{ .hero {{ grid-template-columns: 1.1fr 1fr; }} }}
.ticket .items {{ padding: 12px 16px; display: grid; gap: 8px; }}
.ticket .mod {{ font-size: 16px; color: #BAC1BE; padding-left: 28px; }}
.ticket .diet {{ background: #D87FC6; color: #34092D; font-weight: 800; padding: 8px 16px; font-size: 18px; }}
.ticket .timer {{ display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; font-weight: 800; font-size: 40px; font-variant-numeric: tabular-nums; }}
.ticket .actions {{ display: grid; grid-template-columns: 1fr 1fr; gap: 8px; padding: 0 16px 16px; }}
.ticket .actions button {{ min-height: 64px; font: inherit; font-weight: 700; font-size: 20px; border-radius: 8px; border: 0; cursor: pointer; }}
.tickets {{ display: flex; gap: 24px; flex-wrap: wrap; }}
.tracker {{ display: flex; gap: 24px; align-items: center; flex-wrap: wrap; }}
.tracker .eta {{ font-size: 32px; font-weight: 800; font-variant-numeric: tabular-nums; }}
.steam path {{ animation: steam 2.8s ease-in-out infinite both; }}
.steam path:nth-child(2) {{ animation-delay: .4s; }} .steam path:nth-child(3) {{ animation-delay: .8s; }}
@keyframes steam {{ 0% {{ transform: translateY(10px); opacity: 0; }} 35% {{ opacity: .9; }} 100% {{ transform: translateY(-8px); opacity: 0; }} }}
.ease-lane {{ display: grid; grid-template-columns: 110px 1fr; gap: 12px; align-items: center; margin-bottom: 8px; font-size: 13px; }}
.ease-track {{ height: 28px; background: var(--sl-color-bg-sunken); border-radius: 999px; position: relative; }}
.ease-dot {{ position: absolute; left: 2px; top: 2px; width: 24px; height: 24px; border-radius: 999px; background: var(--sl-color-brand-salmonDeep); }}
.ease-track.go .ease-dot {{ transform: translateX(calc(var(--w) - 28px)); }}
.pin {{ display: grid; grid-template-columns: repeat(3, 72px); gap: 12px; }}
.pin button {{ height: 72px; border-radius: 12px; font: inherit; font-size: 24px; font-weight: 700; background: var(--sl-color-bg-surface); color: var(--sl-color-text-primary); border: 1px solid var(--sl-color-border-default); cursor: pointer; }}
.dots {{ display: flex; gap: 12px; margin-bottom: 16px; }}
.dots span {{ width: 14px; height: 14px; border-radius: 999px; border: 2px solid var(--sl-color-border-strong); }}
.dots span.on {{ background: var(--sl-color-text-primary); }}
.two {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 16px; }}
footer {{ padding-top: 64px; color: var(--sl-color-text-muted); font-size: 13px; }}
@media (prefers-reduced-motion: reduce) {{ * {{ animation: none !important; transition: none !important; }} }}
</style>
</head>
<body>
<header class="top"><div class="wrap">
  <img src="../brand/sushi-latte-lockup.svg" alt="Sushi Latte">
  <nav class="toc" aria-label="Sections">
    <a href="#color">Colour</a><a href="#status">Status</a><a href="#type">Type</a><a href="#space">Space &amp; shape</a>
    <a href="#motion">Motion</a><a href="#components">Components</a>
  </nav>
  <button class="toggle" id="themeToggle" type="button" aria-pressed="false">Dark mode</button>
</div></header>

<main class="wrap">
  <div class="hero">
    <div>
      <div class="overline">Design system</div>
      <h1>Warm front, calm back.</h1>
      <p>Living reference for Sushi Latte tokens and core components. Values come from <span class="mono">docs/design/tokens/*.json</span>; see the Markdown guides in <span class="mono">docs/design/</span> for rules and rationale.</p>
    </div>
    <img src="../brand/sushi-latte-lockup-animated.svg" alt="Animated Sushi Latte logo">
  </div>

  <section id="color">
    <h2>Colour scales</h2>
    <p class="lead">OKLCH-generated, logo colours locked to their nearest step. Salmon 600 is Salmon Deep; Plum is reserved for Dietary Warnings.</p>
    <div class="card">{swatch_rows()}</div>
    <h3>Actions</h3>
    <div class="btn-row">
      <button class="btn brand" type="button">{icon("plus")}Tambah ke pesanan</button>
      <button class="btn neutral" type="button">Klaim pesanan</button>
      <button class="btn secondary" type="button">Lihat</button>
      <button class="btn danger" type="button">Void item</button>
    </div>
  </section>

  <section id="status">
    <h2>Status &amp; domain colours</h2>
    <p class="lead">Always colour + icon + label. The dark panel shows the Kitchen tone set.</p>
    <div class="two">
      <div class="card"><div class="chips">{chips()}</div></div>
      <div class="dark-panel" data-theme="dark"><div class="chips">{chips()}</div></div>
    </div>
  </section>

  <section id="type">
    <h2>Typography</h2>
    <p class="lead">Poppins for brand moments, Plus Jakarta Sans for everything else with tabular numbers, JetBrains Mono for codes. Sizes shown at customer/staff scale.</p>
    <div class="card">{type_rows()}</div>
  </section>

  <section id="space">
    <h2>Space, radius, elevation</h2>
    <div class="two">
      <div class="card"><h3 style="margin-top:0">Spacing (4 px base)</h3>
        {''.join(f'<div class="space-row"><code>space.{k}</code><div class="bar" style="width:{v*3}px"></div>{v}px</div>' for k, v in SPACE)}
      </div>
      <div class="card"><h3 style="margin-top:0">Radius</h3>
        <div class="radius-grid">{''.join(f'<div class="radius-box" style="border-radius:{min(v, 40)}px">{k} · {v if v < 9999 else "pill"}</div>' for k, v in RADIUS)}</div>
      </div>
    </div>
    <h3>Elevation (warm Nori-tinted)</h3>
    <div class="shadow-grid">
      {''.join(f'<div class="shadow-box" style="box-shadow:var(--sl-shadow-{s})">shadow.{s}</div>' for s in ['sm', 'md', 'lg', 'xl', 'glow'])}
    </div>
  </section>

  <section id="motion">
    <h2>Motion</h2>
    <p class="lead">Press <b>Play</b> to compare easings at the expressive duration (600 ms). Staff screens use standard/decelerate at 150–200 ms.</p>
    <div class="card">
      {''.join(f'<div class="ease-lane"><code class="mono">{n}</code><div class="ease-track" data-ease="cubic-bezier({c})"><div class="ease-dot"></div></div></div>' for n, c in EASING)}
      <div class="btn-row" style="margin-top:12px"><button class="btn neutral" id="playEase" type="button">Play</button></div>
    </div>
  </section>

  <section id="components">
    <h2>Components</h2>
    <h3>ProductCard (customer)</h3>
    <div class="products">
      <article class="product">
        <div class="img"><img src="../images/products/food/salmon-aburi-nigiri.jpg" alt="Salmon Aburi Nigiri"></div>
        <div class="body"><div class="name">Salmon Aburi Nigiri</div><div class="desc">Torched salmon, kewpie mayo, tare</div>
          <div class="foot"><span class="price">Rp 48.000</span><button class="add" type="button" aria-label="Tambah Salmon Aburi Nigiri">{icon("plus", 20)}</button></div></div>
      </article>
      <article class="product">
        <div class="img"><img src="../images/products/beverages/iced-matcha-latte.jpg" alt="Iced Uji Matcha Latte">
          <span class="badge-tl chip" style="background:var(--sl-color-dietary-warning-bg);color:var(--sl-color-dietary-warning-fg);border-color:var(--sl-color-dietary-warning-border)">{icon("milk", 14)}Susu</span></div>
        <div class="body"><div class="name">Iced Uji Matcha Latte</div><div class="desc">Ceremonial matcha · oat milk option</div>
          <div class="foot"><span class="price">Rp 45.000</span><button class="add" type="button" aria-label="Tambah Iced Uji Matcha Latte">{icon("plus", 20)}</button></div></div>
      </article>
      <article class="product soldout">
        <div class="img"><img src="../images/products/food/unagi-kabayaki-nigiri.jpg" alt="Unagi Kabayaki Nigiri"><span class="soldout-label">Habis · Sold out</span></div>
        <div class="body"><div class="name">Unagi Kabayaki Nigiri</div><div class="desc">Grilled eel, tare glaze</div>
          <div class="foot"><span class="price">Rp 58.000</span></div></div>
      </article>
      <article class="product">
        <div class="img"><img class="ph" src="../brand/sushi-latte-logo.svg" alt=""></div>
        <div class="body"><div class="name">Iced Espresso Tonic</div><div class="desc">Placeholder (no photo yet)</div>
          <div class="foot"><span class="price">Rp 45.000</span><button class="add" type="button" aria-label="Tambah Iced Espresso Tonic">{icon("plus", 20)}</button></div></div>
      </article>
    </div>

    <h3>OrderStatusTracker — cup-fill Estimated Wait</h3>
    <div class="card tracker">
      <svg width="140" height="140" viewBox="0 0 140 140" role="progressbar" aria-label="Perkiraan waktu" aria-valuemin="0" aria-valuemax="100" aria-valuenow="40" id="cup">
        <defs><clipPath id="cupClip"><path d="M30 50H110L104 104Q101 118 88 118H52Q39 118 36 104Z"/></clipPath></defs>
        <g class="steam" fill="none" stroke="#C9B79C" stroke-width="5" stroke-linecap="round">
          <path d="M55 40C48 32 62 26 55 16"/><path d="M70 36C63 28 77 22 70 12"/><path d="M85 40C78 32 92 26 85 16"/></g>
        <path d="M108 62C130 60 132 98 104 96" fill="none" stroke="#1E2B25" stroke-width="9" stroke-linecap="round"/>
        <path d="M30 50H110L104 104Q101 118 88 118H52Q39 118 36 104Z" fill="#E2D8C7"/>
        <rect id="latte" clip-path="url(#cupClip)" x="20" y="88" width="100" height="40" fill="#8C5634" style="transition: y .6s cubic-bezier(.22,1,.36,1), height .6s cubic-bezier(.22,1,.36,1)"/>
        <path d="M30 50H110L104 104Q101 118 88 118H52Q39 118 36 104Z" fill="none" stroke="#1E2B25" stroke-width="5"/>
        <ellipse cx="70" cy="126" rx="52" ry="6" fill="#1E2B25"/>
      </svg>
      <div>
        <div class="overline">Perkiraan waktu</div>
        <div class="eta" id="eta">± 9 mnt</div>
        <div class="chips" style="margin-top:8px">
          <span class="chip" style="background:var(--sl-color-status-preparing-bg);color:var(--sl-color-status-preparing-fg);border-color:var(--sl-color-status-preparing-border)">{icon("flame", 14)}Disiapkan</span>
        </div>
        <div class="btn-row" style="margin-top:12px"><button class="btn secondary" id="tick" type="button">Simulate +2 mnt</button></div>
      </div>
    </div>

    <h3>KitchenTicket (Kitchen, dark)</h3>
    <div class="dark-panel"><div class="tickets">
      <div class="ticket">
        <div class="head" style="background:#9296FE;color:#181546"><span>Meja 5</span><span>19.42 · Budi</span></div>
        <div class="items"><div><b>2×</b> Salmon Nigiri</div><div class="mod">tanpa wasabi</div><div><b>1×</b> Chashu Ramen</div></div>
        <div class="diet">⚠ SUSU — pelanggan menghindari</div>
        <div class="timer" style="color:#D99302"><span>{icon("timer", 32)}</span><span>07:42</span></div>
        <div class="actions"><button style="background:#59BB72;color:#00260C" type="button">Siap</button><button style="background:#2E4238;color:#FAF6F1" type="button">Batalkan</button></div>
      </div>
      <div class="ticket late">
        <div class="head" style="background:#FF6F63;color:#410002"><span>Bawa pulang · Sari</span><span>TERLAMBAT</span></div>
        <div class="items"><div><b>1×</b> Iced Latte</div><div class="mod">oat milk, less sugar</div></div>
        <div class="timer" style="color:#FF6F63"><span>{icon("timer", 32)}</span><span>11:05</span></div>
        <div class="actions"><button style="background:#59BB72;color:#00260C" type="button">Siap</button><button style="background:#2E4238;color:#FAF6F1" type="button">Batalkan</button></div>
      </div>
    </div></div>

    <h3>PinPad (staff)</h3>
    <div class="card" style="display:inline-block">
      <div class="dots" aria-label="3 dari 6 digit"><span class="on"></span><span class="on"></span><span class="on"></span><span></span><span></span><span></span></div>
      <div class="pin">{''.join(f'<button type="button">{k}</button>' for k in ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', '⌫'])}</div>
    </div>
  </section>

  <footer>Sushi Latte design system · generated from DTCG tokens · see docs/design/*.md</footer>
</main>

<script>
(function () {{
  var root = document.documentElement, btn = document.getElementById('themeToggle');
  function set(mode) {{ root.setAttribute('data-theme', mode); btn.setAttribute('aria-pressed', String(mode === 'dark'));
    btn.textContent = mode === 'dark' ? 'Light mode' : 'Dark mode';
    try {{ localStorage.setItem('sl-theme', mode); }} catch (e) {{}} }}
  var saved = null; try {{ saved = localStorage.getItem('sl-theme'); }} catch (e) {{}}
  set(saved || (window.matchMedia && matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'));
  btn.addEventListener('click', function () {{ set(root.getAttribute('data-theme') === 'dark' ? 'light' : 'dark'); }});

  document.getElementById('playEase').addEventListener('click', function () {{
    document.querySelectorAll('.ease-track').forEach(function (t) {{
      t.style.setProperty('--w', t.clientWidth + 'px');
      var d = t.querySelector('.ease-dot');
      d.style.transition = 'none'; t.classList.remove('go'); void d.offsetWidth;
      d.style.transition = 'transform 600ms ' + t.dataset.ease; t.classList.add('go');
    }});
  }});

  var level = 40, eta = 9, latte = document.getElementById('latte'), cup = document.getElementById('cup');
  function draw() {{ var h = 68 * level / 100; latte.setAttribute('y', 118 - h); latte.setAttribute('height', h + 10);
    cup.setAttribute('aria-valuenow', String(level)); document.getElementById('eta').textContent = level >= 100 ? 'Siap!' : '± ' + eta + ' mnt'; }}
  document.getElementById('tick').addEventListener('click', function () {{
    if (level >= 100) {{ level = 10; eta = 14; }} else {{ eta = Math.max(0, eta - 2); level = eta === 0 ? 100 : Math.min(95, level + 15); }}
    draw();
  }});
  draw();
}})();
</script>
</body>
</html>
'''
open(OUT, 'w', encoding='utf-8', newline='\n').write(html)
print('written', len(html))
