"""Generate OKLCH-based 11-step scales anchored on Sushi Latte brand colours,
and report WCAG contrast for the pairs used by semantic tokens."""
import json, math, sys

STEPS = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950]
L_TARGET = [0.975, 0.945, 0.885, 0.805, 0.715, 0.625, 0.545, 0.465, 0.385, 0.305, 0.235]
C_CURVE = [0.18, 0.32, 0.55, 0.78, 0.94, 1.0, 1.0, 0.93, 0.82, 0.68, 0.56]


def srgb_to_lin(c):
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def lin_to_srgb(c):
    return 12.92 * c if c <= 0.0031308 else 1.055 * c ** (1 / 2.4) - 0.055


def hex_to_rgb(h):
    h = h.lstrip('#')
    return tuple(int(h[i:i + 2], 16) / 255 for i in (0, 2, 4))


def rgb_to_hex(rgb):
    return '#' + ''.join(f'{round(max(0, min(1, c)) * 255):02X}' for c in rgb)


def rgb_to_oklch(rgb):
    r, g, b = (srgb_to_lin(c) for c in rgb)
    l = 0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b
    m = 0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b
    s = 0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b
    l_, m_, s_ = (math.copysign(abs(v) ** (1 / 3), v) for v in (l, m, s))
    L = 0.2104542553 * l_ + 0.7936177850 * m_ - 0.0040720468 * s_
    a = 1.9779984951 * l_ - 2.4285922050 * m_ + 0.4505937099 * s_
    bb = 0.0259040371 * l_ + 0.7827717662 * m_ - 0.8086757660 * s_
    return L, math.hypot(a, bb), math.degrees(math.atan2(bb, a)) % 360


def oklch_to_rgb_lin(L, C, H):
    a, b = C * math.cos(math.radians(H)), C * math.sin(math.radians(H))
    l_ = L + 0.3963377774 * a + 0.2158037573 * b
    m_ = L - 0.1055613458 * a - 0.0638541728 * b
    s_ = L - 0.0894841775 * a - 1.2914855480 * b
    l, m, s = l_ ** 3, m_ ** 3, s_ ** 3
    return (4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
            -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
            -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s)


def oklch_to_hex(L, C, H):
    lo, hi = 0.0, C
    for _ in range(30):  # largest in-gamut chroma <= C
        mid = (lo + hi) / 2
        if all(-1e-4 <= v <= 1 + 1e-4 for v in oklch_to_rgb_lin(L, mid, H)):
            lo = mid
        else:
            hi = mid
    return rgb_to_hex(tuple(lin_to_srgb(max(0, min(1, v))) for v in oklch_to_rgb_lin(L, lo, H)))


def luminance(hexv):
    r, g, b = (srgb_to_lin(c) for c in hex_to_rgb(hexv))
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def contrast(a, b):
    la, lb = sorted((luminance(a), luminance(b)), reverse=True)
    return (la + 0.05) / (lb + 0.05)


def scale(hue, peak_c, anchors=None):
    out = {}
    for step, L, k in zip(STEPS, L_TARGET, C_CURVE):
        out[step] = oklch_to_hex(L, peak_c * k, hue)
    for hexv in (anchors or []):
        L = rgb_to_oklch(hex_to_rgb(hexv))[0]
        step = min(STEPS, key=lambda s: abs(L_TARGET[STEPS.index(s)] - L))
        out[step] = hexv.upper()
    return out


def hue_of(hexv):
    return rgb_to_oklch(hex_to_rgb(hexv))[2]


SCALES = {
    'nori': scale(hue_of('#1E2B25'), 0.045, ['#1E2B25', '#2E4238']),
    'neutral': scale(hue_of('#1E2B25'), 0.012),
    'rice': scale(hue_of('#F7F0E4'), 0.045, ['#F7F0E4']),
    'espresso': scale(hue_of('#6F4428'), 0.10, ['#6F4428', '#8C5634']),
    'salmon': scale(hue_of('#F2885B'), 0.16, ['#F2885B', '#B9512B']),
    'amber': scale(75, 0.16),
    'blue': scale(255, 0.16),
    'indigo': scale(280, 0.16),
    'green': scale(150, 0.15),
    'red': scale(27, 0.19),
    'plum': scale(335, 0.15),
}

if __name__ == '__main__':
    json.dump(SCALES, sys.stdout, indent=1)
