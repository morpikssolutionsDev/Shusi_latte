"""Regenerate every design artefact from the palette and token definitions.

    python tools/design/build.py

Order matters: tokens first (the style guide embeds their CSS), then COLOR.md, then the style guide.
"""
import os, runpy

HERE = os.path.dirname(os.path.abspath(__file__))

for script in ('build_tokens.py', 'build_color_doc.py', 'build_styleguide.py'):
    print(f'-> {script}')
    runpy.run_path(os.path.join(HERE, script), run_name='__main__')
