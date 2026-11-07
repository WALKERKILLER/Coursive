#!/usr/bin/env python3
"""subset-export-font.py — rebuild app/assets/fonts/NotoSansSC-{Regular,Bold}.ttf.

Input: NotoSansSC[wght].ttf from https://github.com/google/fonts/tree/main/ofl/notosanssc
(SIL OFL 1.1). Needs fontTools. The PDF exporter needs TrueType outlines, so the
variable font is instanced at weights 400 and 700 and subset to GB2312, Latin,
Greek, punctuation, arrows, math operators, ticks and full-width forms.

    python3 scripts/subset-export-font.py path/to/NotoSansSC[wght].ttf
"""
import os
import subprocess
import sys
import tempfile

RANGES = [(0x20, 0x7E), (0xA0, 0x17F), (0x370, 0x3FF), (0x2000, 0x206F), (0x20A0, 0x20BF), (0x2100, 0x218F),
          (0x2190, 0x22FF), (0x2460, 0x257F), (0x25A0, 0x27BF), (0x3000, 0x303F), (0xFF00, 0xFFEF)]


def charset():
    chars = set()
    for hi in range(0xA1, 0xF8):
        for lo in range(0xA1, 0xFF):
            try:
                chars.add(bytes([hi, lo]).decode("gb2312"))
            except UnicodeDecodeError:
                pass
    for a, b in RANGES:
        chars.update(chr(c) for c in range(a, b + 1))
    return "".join(sorted(chars))


def main(source):
    out = os.path.join(os.path.dirname(__file__), "..", "app", "assets", "fonts")
    with tempfile.TemporaryDirectory() as tmp:
        text = os.path.join(tmp, "chars.txt")
        with open(text, "w", encoding="utf-8") as f:
            f.write(charset())
        for weight, name in ((400, "Regular"), (700, "Bold")):
            instance = os.path.join(tmp, f"{weight}.ttf")
            subprocess.run([sys.executable, "-m", "fontTools.varLib.instancer", source, f"wght={weight}",
                            "--update-name-table", "-q", "-o", instance], check=True)
            subprocess.run([sys.executable, "-m", "fontTools.subset", instance, f"--text-file={text}",
                            f"--output-file={os.path.join(out, f'NotoSansSC-{name}.ttf')}",
                            "--layout-features=kern,liga,locl,vert,halt,palt", "--no-hinting", "--desubroutinize",
                            "--name-IDs=*", "--name-languages=*", "--notdef-outline", "--drop-tables+=BASE,STAT,DSIG"],
                           check=True)


if __name__ == "__main__":
    main(sys.argv[1])
