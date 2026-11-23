"""Crop-aware image and text rendering through PDFium.

pypdfium2 handles the page-to-bitmap transform, including PDF rotation.
https://pypdfium2.readthedocs.io/en/stable/python_api.html
"""
import io
import json
import math
import sys
from unicodedata import east_asian_width
from contextlib import closing

import pypdfium2 as pdfium
from pypdf import PdfReader, PdfWriter


class PageLimit(ValueError):
    pass


def bitmap(page, pixel_limit):
    largest = max(page.get_size())
    if not math.isfinite(largest) or largest <= 0:
        raise ValueError("PDF page has invalid dimensions")
    return page.render(scale=min(2, pixel_limit / largest), draw_annots=True)


def render_notes(source, number, hidden):
    if hidden:
        writer = PdfWriter()
        page = writer.add_page(PdfReader(source).pages[number - 1])
        for i in sorted({int(v) for v in hidden.split(",")}, reverse=True):
            if 0 <= i < len(page.annotations or []):
                del page.annotations[i]
        data = io.BytesIO()
        writer.write(data)
        source = data.getvalue()
        number = 1
    with pdfium.PdfDocument(source) as document, closing(document[number - 1]) as page:
        with closing(bitmap(page, 1500)) as rendered:
            rendered.to_pil().save(sys.stdout.buffer, "PNG")


def page_text(page, rendered):
    with closing(page.get_textpage()) as textpage:
        if textpage.count_chars() > 20000:
            raise PageLimit("PDF page exceeds character limit")
        position = rendered.get_posconv(page)
        text, boxes = [], []
        offset = 0
        word = None
        for i in range(textpage.count_chars()):
            value = textpage.get_text_range(i, 1)
            if not value:
                continue
            if value.isspace():
                word = None
                if not text or text[-1] == " ":
                    continue
                value = " "
            start = offset
            offset += len(value.encode("utf-16-le", errors="replace")) // 2
            text.append(value)
            if value == " ":
                continue
            try:
                x0, y0, x1, y1 = textpage.get_charbox(i)
            except pdfium.PdfiumError:
                word = None
                continue
            x0, y0 = position.to_bitmap(x0, y0)
            x1, y1 = position.to_bitmap(x1, y1)
            left, right = sorted((x0 / rendered.width, x1 / rendered.width))
            top, bottom = sorted((y0 / rendered.height, y1 / rendered.height))
            if left >= right or top >= bottom:
                continue
            # CJK words do not always have spaces. Keep their selectable
            # regions close to the individual displayed glyphs.
            wide = east_asian_width(value[0]) in ("F", "W")
            if word is None or wide:
                word = dict(start=start, end=offset, x=left, y=top,
                            w=right-left, h=bottom-top)
                boxes.append(word)
            else:
                right = max(right, word["x"] + word["w"])
                bottom = max(bottom, word["y"] + word["h"])
                word["x"] = min(left, word["x"])
                word["y"] = min(top, word["y"])
                word["w"] = right - word["x"]
                word["h"] = bottom - word["y"]
                word["end"] = offset
            if wide:
                word = None
        return "".join(text), boxes


def render_study(source, output, number):
    with pdfium.PdfDocument(source) as doc:
        count = len(doc)
        if not 1 <= count <= (5000 if number else 100):
            raise PageLimit("PDF page count is outside the supported range")
        if number and not 1 <= number <= count:
            raise PageLimit("PDF page is outside the document")
        pages, total = [], 0
        for i in ([number - 1] if number else range(count)):
            filename = f"page-{i + 1}.png"
            with closing(doc[i]) as page, closing(bitmap(page, 1400)) as rendered:
                text, boxes = page_text(page, rendered)
                image = output / filename
                rendered.to_pil().save(image, "PNG")
                total += image.stat().st_size
                if total > 300 * 2**20:
                    raise PageLimit("Rendered PDF exceeds the import limit")
                info = dict(file=filename, width=rendered.width, height=rendered.height, text=text)
                if number:
                    info.update(count=count, boxes=boxes)
                pages.append(info)
        name = "page.json" if number else "pages.json"
        (output / name).write_text(json.dumps(pages[0] if number else pages), encoding="utf-8")
