# Design generators

Rebuild every design artefact after changing a colour, token or contrast rule:

```sh
python tools/design/build.py
```

| Script | Reads | Writes |
|---|---|---|
| `palette.py` | Brand anchor colours, OKLCH step targets | (module) 11 colour scales, WCAG contrast function |
| `build_tokens.py` | `palette.py`, token definitions in the script | `docs/design/tokens/*.json`, `tools/design/.build/tokens.css` (git-ignored) |
| `build_color_doc.py` | `palette.py` | `docs/design/COLOR.md` (scales + contrast proof) |
| `build_styleguide.py` | `.build/tokens.css`, `palette.py` | `docs/design/styleguide.html` |

Python 3.10+, standard library only.

Rules:
- Change brand anchors or scale shape in `palette.py`; change roles, surfaces and component tokens in `build_tokens.py`. Never hand-edit the generated JSON, `COLOR.md` tables or the style guide.
- After a rebuild, check `COLOR.md` §5: any ❌ outside "Forbidden combinations" means the change breaks contrast and must be reverted or fixed.
- The app's own token pipeline (Style Dictionary → CSS variables, Tailwind theme, shadcn variables) reads the same `docs/design/tokens/*.json`; see [TOKENS.md](../../docs/design/TOKENS.md#5-pipeline).
