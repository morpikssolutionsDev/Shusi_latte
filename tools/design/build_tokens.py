"""Write Sushi Latte DTCG token files and a flattened CSS preview."""
import json, os, re, sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..'))
sys.path.insert(0, HERE)
import palette

OUT = os.path.join(ROOT, 'docs', 'design', 'tokens')
CSS_OUT = os.path.join(HERE, '.build', 'tokens.css')
os.makedirs(os.path.dirname(CSS_OUT), exist_ok=True)
SCALES = {n: {str(k): v for k, v in s.items()} for n, s in palette.SCALES.items()}


def c(v, d=None):
    t = {'$type': 'color', '$value': v}
    if d: t['$description'] = d
    return t


def dim(v, d=None):
    t = {'$type': 'dimension', '$value': v}
    if d: t['$description'] = d
    return t


def num(v, d=None):
    t = {'$type': 'number', '$value': v}
    if d: t['$description'] = d
    return t


def ref(path, typ='color', d=None):
    t = {'$type': typ, '$value': '{' + path + '}'}
    if d: t['$description'] = d
    return t


# ---------------------------------------------------------------- primitive
primitive = {
    '$description': 'Sushi Latte primitive tokens: raw values only. Never use directly in components; go through semantic tokens.',
    'color': {name: {step: c(hexv) for step, hexv in scale.items()} for name, scale in SCALES.items()},
    'font': {
        'family': {
            'display': {'$type': 'fontFamily', '$value': ['Poppins', 'Plus Jakarta Sans', 'Segoe UI', 'Helvetica Neue', 'Arial', 'sans-serif'], '$description': 'Headings, wordmark, Product names on customer surfaces'},
            'ui': {'$type': 'fontFamily', '$value': ['Plus Jakarta Sans', 'Segoe UI', 'Helvetica Neue', 'Arial', 'sans-serif'], '$description': 'All interface and body text'},
            'mono': {'$type': 'fontFamily', '$value': ['JetBrains Mono', 'Consolas', 'Menlo', 'monospace'], '$description': 'Bill numbers, codes, audit IDs'},
        },
        'weight': {k: {'$type': 'fontWeight', '$value': v} for k, v in
                   {'regular': 400, 'medium': 500, 'semibold': 600, 'bold': 700, 'extrabold': 800}.items()},
        'lineHeight': {k: num(v) for k, v in {'tight': 1.1, 'snug': 1.25, 'normal': 1.5, 'relaxed': 1.65}.items()},
        'letterSpacing': {k: dim(v) for k, v in {'tighter': '-0.02em', 'tight': '-0.01em', 'normal': '0em', 'wide': '0.04em', 'caps': '0.12em'}.items()},
    },
    'space': {k: dim(v) for k, v in {'0': '0px', '1': '4px', '2': '8px', '3': '12px', '4': '16px', '5': '20px', '6': '24px',
                                      '8': '32px', '10': '40px', '12': '48px', '16': '64px', '20': '80px', '24': '96px'}.items()},
    'radius': {k: dim(v) for k, v in {'none': '0px', 'xs': '4px', 'sm': '8px', 'md': '10px', 'lg': '12px', 'xl': '20px', '2xl': '28px', 'pill': '9999px'}.items()},
    'shadow': {
        'sm': {'$type': 'shadow', '$value': [{'color': '#1E2B2514', 'offsetX': '0px', 'offsetY': '1px', 'blur': '2px', 'spread': '0px'}]},
        'md': {'$type': 'shadow', '$value': [{'color': '#1E2B251A', 'offsetX': '0px', 'offsetY': '4px', 'blur': '12px', 'spread': '-2px'},
                                              {'color': '#1E2B250F', 'offsetX': '0px', 'offsetY': '2px', 'blur': '4px', 'spread': '0px'}]},
        'lg': {'$type': 'shadow', '$value': [{'color': '#1E2B252E', 'offsetX': '0px', 'offsetY': '12px', 'blur': '32px', 'spread': '-8px'}]},
        'xl': {'$type': 'shadow', '$value': [{'color': '#1E2B253D', 'offsetX': '0px', 'offsetY': '24px', 'blur': '48px', 'spread': '-12px'}]},
        'glow': {'$type': 'shadow', '$value': [{'color': '#F2885B66', 'offsetX': '0px', 'offsetY': '0px', 'blur': '24px', 'spread': '2px'}], '$description': 'Celebration moments only'},
    },
    'duration': {k: {'$type': 'duration', '$value': v} for k, v in
                 {'instant': '80ms', 'fast': '150ms', 'base': '200ms', 'moderate': '300ms', 'slow': '450ms',
                  'expressive': '600ms', 'hero': '900ms', 'celebrate': '1200ms', 'loop': '2800ms', 'slide': '8000ms'}.items()},
    'easing': {k: {'$type': 'cubicBezier', '$value': v} for k, v in
               {'standard': [0.2, 0, 0, 1], 'decelerate': [0.05, 0.7, 0.1, 1], 'accelerate': [0.3, 0, 0.8, 0.15],
                'expressive': [0.22, 1, 0.36, 1], 'overshoot': [0.34, 1.56, 0.64, 1], 'linear': [0, 0, 1, 1]}.items()},
    'spring': {
        '$description': 'Spring presets for the Motion library: stiffness / damping / mass.',
        'gentle': {'stiffness': num(170), 'damping': num(26), 'mass': num(1)},
        'bouncy': {'stiffness': num(300), 'damping': num(18), 'mass': num(1)},
        'snappy': {'stiffness': num(500), 'damping': num(35), 'mass': num(1)},
    },
    'opacity': {k: num(v) for k, v in {'disabled': 0.4, 'soldOut': 0.45, 'scrim': 0.6, 'hover': 0.08, 'pressed': 0.14}.items()},
    'zIndex': {k: num(v) for k, v in {'base': 0, 'sticky': 10, 'header': 20, 'overlay': 40, 'sheet': 50, 'dialog': 60, 'toast': 70, 'notification': 80}.items()},
    'breakpoint': {k: dim(v) for k, v in {'phone': '360px', 'phoneLg': '430px', 'tablet': '768px', 'tabletLg': '1024px', 'desktop': '1280px', 'wide': '1920px'}.items()},
    'size': {
        'touch': {k: dim(v) for k, v in {'desktop': '40px', 'phone': '44px', 'tablet': '48px', 'kitchen': '64px'}.items()},
        'icon': {k: dim(v) for k, v in {'xs': '14px', 'sm': '16px', 'md': '20px', 'lg': '24px', 'xl': '32px', '2xl': '48px'}.items()},
    },
}

# ---------------------------------------------------------------- semantic
STATUS = {'pending': 'amber', 'confirmed': 'blue', 'preparing': 'indigo', 'ready': 'green',
          'served': 'neutral', 'cancelled': 'neutral', 'voided': 'red'}
FEEDBACK = {'success': 'green', 'warning': 'amber', 'danger': 'red', 'info': 'blue'}


def tone_light(h):
    return {'bg': ref(f'color.{h}.100'), 'fg': ref(f'color.{h}.800'), 'border': ref(f'color.{h}.300'),
            'icon': ref(f'color.{h}.600'), 'solid': ref(f'color.{h}.600'), 'onSolid': ref('color.white')}


def tone_dark(h):
    return {'bg': ref(f'color.{h}.950'), 'fg': ref(f'color.{h}.200'), 'border': ref(f'color.{h}.700'),
            'icon': ref(f'color.{h}.400'), 'solid': ref(f'color.{h}.400'), 'onSolid': ref(f'color.{h}.950')}


primitive['color']['white'] = c('#FFFFFF')
primitive['color']['black'] = c('#000000')
primitive['color']['illustration'] = {
    '$description': 'Food tints for the illustration kit only; never for UI.',
    'matcha': c('#7FA65A'), 'tamago': c('#F2C14E'), 'tuna': c('#C7463F'), 'avocado': c('#9BBF5A'),
    'cucumber': c('#5E9E6B'), 'chocolate': c('#5A3622'), 'steam': c('#C9B79C'), 'riceWhite': c('#FFFCF5'),
    'salmonFat': c('#FFD9C7'),
}


def semantic(mode):
    light = mode == 'light'
    tone = tone_light if light else tone_dark
    s = {
        'bg': {
            'canvas': ref('color.neutral.50' if light else 'color.nori.950', d='App background (staff); customer surfaces override with Rice'),
            'canvasWarm': ref('color.rice.50' if light else 'color.nori.950', d='Customer surface background'),
            'surface': ref('color.white' if light else 'color.nori.900', d='Cards, sheets, panels'),
            'raised': ref('color.white' if light else 'color.nori.800', d='Popovers, menus, raised cards'),
            'sunken': ref('color.neutral.100' if light else 'color.neutral.950', d='Wells, table stripes, inputs on surface'),
            'inverse': ref('color.nori.900' if light else 'color.rice.100'),
            'scrim': ref('color.nori.950', d='Use with opacity.scrim'),
        },
        'text': {
            'primary': ref('color.nori.900' if light else 'color.rice.50'),
            'secondary': ref('color.neutral.700' if light else 'color.neutral.300'),
            'muted': ref('color.neutral.600' if light else 'color.neutral.400', d='Minimum for readable text; never lighter'),
            'inverse': ref('color.rice.50' if light else 'color.nori.950'),
            'brand': ref('color.espresso.700' if light else 'color.espresso.300', d='"latte" in wordmark, prices on customer surfaces'),
            'link': ref('color.blue.700' if light else 'color.blue.300'),
            'disabled': ref('color.neutral.500' if light else 'color.neutral.600', d='Use with opacity.disabled on the control'),
        },
        'border': {
            'subtle': ref('color.neutral.200' if light else 'color.nori.800', d='Dividers; decorative only'),
            'default': ref('color.neutral.300' if light else 'color.neutral.700', d='Card outlines'),
            'input': ref('color.neutral.500', d='Form controls; meets 3:1'),
            'strong': ref('color.nori.900' if light else 'color.rice.100'),
            'focus': ref('color.blue.600' if light else 'color.blue.400', d='Focus ring, 2px + 2px offset'),
        },
        'action': {
            'brand': {
                '$description': 'Customer primary: Salmon Deep',
                'bg': ref('color.salmon.600' if light else 'color.salmon.400'),
                'bgHover': ref('color.salmon.700' if light else 'color.salmon.300'),
                'bgPressed': ref('color.salmon.800' if light else 'color.salmon.200'),
                'fg': ref('color.white' if light else 'color.nori.950'),
            },
            'neutral': {
                '$description': 'Staff primary: Nori',
                'bg': ref('color.nori.900' if light else 'color.rice.100'),
                'bgHover': ref('color.nori.800' if light else 'color.rice.200'),
                'bgPressed': ref('color.nori.950' if light else 'color.rice.300'),
                'fg': ref('color.white' if light else 'color.nori.950'),
            },
            'secondary': {
                'bg': ref('color.neutral.100' if light else 'color.nori.800'),
                'bgHover': ref('color.neutral.200' if light else 'color.nori.700'),
                'fg': ref('color.nori.900' if light else 'color.rice.50'),
                'border': ref('color.neutral.300' if light else 'color.neutral.700'),
            },
            'danger': {
                'bg': ref('color.red.600' if light else 'color.red.400'),
                'bgHover': ref('color.red.700' if light else 'color.red.300'),
                'fg': ref('color.white' if light else 'color.red.950'),
            },
        },
        'status': {k: tone(h) for k, h in STATUS.items()},
        'feedback': {k: tone(h) for k, h in FEEDBACK.items()},
        'timer': {
            'onTime': ref(f'color.green.{600 if light else 400}'),
            'warning': ref(f'color.amber.{600 if light else 400}', d='>= 80% of Prep Time'),
            'late': ref(f'color.red.{600 if light else 400}', d='>= 100% of Prep Time'),
        },
        'dietary': {'warning': tone('plum'), '$description': 'Plum is reserved for Dietary Warnings and nothing else'},
        'bill': {
            'settling': ref(f'color.blue.{600 if light else 400}'),
            'unpaid': ref(f'color.red.{600 if light else 400}'),
        },
        'soldOut': {
            'fg': ref('color.neutral.600' if light else 'color.neutral.400'),
            'badgeBg': ref('color.nori.900' if light else 'color.rice.100'),
            'badgeFg': ref('color.rice.50' if light else 'color.nori.950'),
        },
        'brand': {
            'nori': ref('color.nori.900'), 'noriLight': ref('color.nori.800'), 'rice': ref('color.rice.100'),
            'espresso': ref('color.espresso.700'), 'crema': ref('color.espresso.600'),
            'salmon': ref('color.salmon.400', d='Decorative only: fails text contrast on light'),
            'salmonDeep': ref('color.salmon.600'), 'steam': c('#C9B79C'),
        },
    }
    return {'$description': f'Sushi Latte semantic colour roles, {mode} mode.', 'color': s}


# ---------------------------------------------------------------- surfaces
def type_scale(sizes):
    return {k: dim(f'{v}px') for k, v in sizes.items()}


SURFACES = {
    'customer': {
        'desc': 'Client Menu (phone, portrait). Expressive, warm, light only.',
        'mode': 'light', 'canvas': 'color.bg.canvasWarm', 'primary': 'color.action.brand',
        'type': {'xs': 12, 'sm': 14, 'base': 16, 'lg': 20, 'xl': 24, '2xl': 32},
        'radius': {'control': 'radius.pill', 'card': 'radius.xl', 'sheet': 'radius.2xl', 'chip': 'radius.pill'},
        'touch': 'size.touch.phone', 'motion': 'expressive', 'density': 'comfortable',
    },
    'staff': {
        'desc': 'Waiter (8" tablet portrait) and Cashier (10-11" landscape or PC). Functional, calm.',
        'mode': 'light (dark available)', 'canvas': 'color.bg.canvas', 'primary': 'color.action.neutral',
        'type': {'xs': 12, 'sm': 14, 'base': 16, 'lg': 20, 'xl': 24, '2xl': 28, 'amountMd': 24, 'amountLg': 32, 'amountXl': 40},
        'radius': {'control': 'radius.md', 'card': 'radius.lg', 'sheet': 'radius.xl', 'chip': 'radius.pill'},
        'touch': 'size.touch.tablet', 'motion': 'functional', 'density': 'compact',
    },
    'admin': {
        'desc': 'Admin Panel and HRIS (desktop >= 1280px, usable on landscape tablet).',
        'mode': 'light (dark available)', 'canvas': 'color.bg.canvas', 'primary': 'color.action.neutral',
        'type': {'xs': 12, 'sm': 13, 'base': 14, 'lg': 16, 'xl': 20, '2xl': 24, '3xl': 28},
        'radius': {'control': 'radius.sm', 'card': 'radius.lg', 'sheet': 'radius.xl', 'chip': 'radius.pill'},
        'touch': 'size.touch.desktop', 'motion': 'functional', 'density': 'dense',
    },
    'kitchen': {
        'desc': 'Kitchen display per Prep Area (1920x1080). Dark default, readable from ~3 m.',
        'mode': 'dark', 'canvas': 'color.bg.canvas', 'primary': 'color.action.neutral',
        'type': {'sm': 20, 'base': 24, 'lg': 30, 'xl': 36, '2xl': 48},
        'radius': {'control': 'radius.sm', 'card': 'radius.sm', 'sheet': 'radius.lg', 'chip': 'radius.pill'},
        'touch': 'size.touch.kitchen', 'motion': 'functional', 'density': 'spacious',
    },
    'display': {
        'desc': 'Product Display TV (1920x1080, 4K safe). Expressive, dark, brand-led.',
        'mode': 'dark', 'canvas': 'color.bg.canvas', 'primary': 'color.action.brand',
        'type': {'base': 32, 'lg': 40, 'xl': 56, '2xl': 72, '3xl': 96, '4xl': 120},
        'radius': {'control': 'radius.pill', 'card': 'radius.2xl', 'sheet': 'radius.2xl', 'chip': 'radius.pill'},
        'touch': None, 'motion': 'expressive', 'density': 'spacious',
    },
}


def surface(name, cfg):
    t = {
        '$description': cfg['desc'],
        'surface': {
            'mode': {'$type': 'string', '$value': cfg['mode']} if False else None,
        },
    }
    out = {
        '$description': cfg['desc'] + f" Mode: {cfg['mode']}. Motion profile: {cfg['motion']}. Density: {cfg['density']}.",
        'color': {
            'canvas': ref(cfg['canvas']),
            'primary': {k: ref(f"{cfg['primary']}.{k}") for k in ('bg', 'bgHover', 'bgPressed', 'fg')},
        },
        'font': {'size': type_scale(cfg['type'])},
        'radius': {k: ref(v, 'dimension') for k, v in cfg['radius'].items()},
    }
    if cfg['touch']:
        out['size'] = {'touchMin': ref(cfg['touch'], 'dimension')}
    return out


# ---------------------------------------------------------------- components
component = {
    '$description': 'Component tokens. Values reference semantic or surface tokens; surface radius/size resolve per app.',
    'button': {
        'height': {'sm': dim('36px'), 'md': dim('44px'), 'lg': dim('52px'), 'xl': dim('64px', 'Kitchen and Payment confirm')},
        'paddingX': {'sm': ref('space.3', 'dimension'), 'md': ref('space.4', 'dimension'), 'lg': ref('space.6', 'dimension')},
        'radius': ref('radius.control', 'dimension'),
        'fontWeight': ref('font.weight.semibold', 'fontWeight'),
    },
    'input': {
        'height': dim('44px'), 'radius': ref('radius.control', 'dimension'),
        'border': ref('color.border.input'), 'borderFocus': ref('color.border.focus'), 'bg': ref('color.bg.surface'),
    },
    'card': {'radius': ref('radius.card', 'dimension'), 'padding': ref('space.4', 'dimension'),
             'bg': ref('color.bg.surface'), 'border': ref('color.border.subtle')},
    'chip': {'height': dim('28px'), 'paddingX': ref('space.3', 'dimension'), 'radius': ref('radius.chip', 'dimension')},
    'sheet': {'radius': ref('radius.sheet', 'dimension'), 'bg': ref('color.bg.surface')},
    'productCard': {'imageRatio': {'$type': 'string', '$value': '1/1'},
                    'displayImageRatio': {'$type': 'string', '$value': '16/9', '$description': 'Product Display slides'}, 'radius': ref('radius.xl', 'dimension'),
                    'shadow': ref('shadow.md', 'shadow'), 'priceColor': ref('color.text.brand')},
    'dietaryBadge': {'bg': ref('color.dietary.warning.bg'), 'fg': ref('color.dietary.warning.fg'),
                     'border': ref('color.dietary.warning.border'), 'icon': ref('color.dietary.warning.icon')},
    'kitchenTicket': {'radius': ref('radius.sm', 'dimension'), 'headerHeight': dim('56px'), 'width': dim('360px'),
                      'remakeBorder': ref('color.salmon.400'), 'lateBorder': ref('color.timer.late')},
    'pinPad': {'keySize': dim('72px'), 'gap': ref('space.3', 'dimension'), 'radius': ref('radius.lg', 'dimension')},
    'paymentPad': {'keyHeight': dim('64px'), 'amountSize': dim('40px')},
    'toast': {'radius': ref('radius.lg', 'dimension'), 'shadow': ref('shadow.lg', 'shadow')},
    'focusRing': {'width': dim('2px'), 'offset': dim('2px'), 'color': ref('color.border.focus')},
}

os.makedirs(OUT, exist_ok=True)


def write(name, data):
    with open(os.path.join(OUT, name), 'w', encoding='utf-8', newline='\n') as f:
        json.dump(data, f, indent=2, ensure_ascii=False)
        f.write('\n')


write('primitive.json', primitive)
write('semantic.light.json', semantic('light'))
write('semantic.dark.json', semantic('dark'))
for n, cfg in SURFACES.items():
    write(f'surface.{n}.json', surface(n, cfg))
write('component.json', component)

# ---------------------------------------------------------------- CSS preview (resolved) for the style guide
def flatten(node, prefix=''):
    for k, v in node.items():
        if k.startswith('$'):
            continue
        if isinstance(v, dict) and '$value' in v:
            yield prefix + k, v
        elif isinstance(v, dict):
            yield from flatten(v, prefix + k + '.')


def resolve(tokens, path, seen=0):
    v = tokens[path]['$value']
    if isinstance(v, str) and v.startswith('{'):
        return resolve(tokens, v[1:-1], seen + 1)
    return v


def css_value(v):
    if isinstance(v, list) and v and isinstance(v[0], dict):
        return ', '.join(f"{s['offsetX']} {s['offsetY']} {s['blur']} {s['spread']} {s['color']}" for s in v)
    if isinstance(v, list) and all(isinstance(x, (int, float)) for x in v):
        return 'cubic-bezier(' + ', '.join(str(x) for x in v) + ')'
    if isinstance(v, list):
        return ', '.join(f'"{x}"' if ' ' in x else x for x in v)
    return str(v)


def var(path):
    return '--sl-' + re.sub(r'[^a-zA-Z0-9]+', '-', path)


prim = dict(flatten(primitive))
light = dict(flatten(semantic('light')))
dark = dict(flatten(semantic('dark')))
comp_lines = []
with open(CSS_OUT, 'w', encoding='utf-8', newline='\n') as f:
    f.write(':root {\n')
    for p, t in prim.items():
        f.write(f'  {var(p)}: {css_value(t["$value"])};\n')
    all_l = {**prim, **light}
    for p in light:
        f.write(f'  {var(p)}: {css_value(resolve(all_l, p))};\n')
    f.write('}\n[data-theme="dark"] {\n')
    all_d = {**prim, **dark}
    for p in dark:
        f.write(f'  {var(p)}: {css_value(resolve(all_d, p))};\n')
    f.write('}\n')
print('ok')
