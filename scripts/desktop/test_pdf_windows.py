"""Exercise the PDFium renderer and smoke-test its packaged Windows entry."""
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from pypdf import PdfWriter
from pypdf.generic import DecodedStreamObject, DictionaryObject, NameObject

from pdf_windows import render_study

PNG_HEADER = b"\x89PNG\r\n\x1a\n"


def write_pdf(source, text_lines):
    writer = PdfWriter()
    page = writer.add_blank_page(width=1200, height=850)
    font = DictionaryObject({
        NameObject("/Type"): NameObject("/Font"),
        NameObject("/Subtype"): NameObject("/Type1"),
        NameObject("/BaseFont"): NameObject("/Helvetica"),
    })
    page[NameObject("/Resources")] = DictionaryObject({
        NameObject("/Font"): DictionaryObject({NameObject("/F1"): font}),
    })
    content = DecodedStreamObject()
    lines = "".join(f"({line}) Tj T*\n" for line in text_lines)
    content.set_data(f"BT /F1 5 Tf 6 TL 15 830 Td\n{lines}ET\n".encode("ascii"))
    page[NameObject("/Contents")] = writer._add_object(content)
    writer.write(source)


class PDFiumTests(unittest.TestCase):
    def test_dense_page_keeps_text_in_both_import_modes(self):
        with tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp)
            source = folder / "中文 目录 讲义.pdf"
            word = "course"
            lines = ((" ".join([word] * 32),) * 128)
            write_pdf(source, lines)

            render_study(source, folder, 1)
            page = json.loads((folder / "page.json").read_text(encoding="utf-8"))
            self.assertEqual(page["count"], 1)
            self.assertGreater(len(page["text"]), 20_000)
            first = page["boxes"][0]
            self.assertEqual(page["text"][first["start"]:first["end"]], word)
            self.assertTrue((folder / page["file"]).read_bytes().startswith(PNG_HEADER))

            render_study(source, folder, None)
            pages = json.loads((folder / "pages.json").read_text(encoding="utf-8"))
            self.assertEqual(pages[0]["text"], page["text"])


def smoke_helper(helper):
    with tempfile.TemporaryDirectory() as tmp:
        folder = Path(tmp)
        source = folder / "笔记 中文路径.pdf"
        work = folder / "未创建 的副本.pdf"
        write_pdf(source, ["Coursive package check"])

        def call(*args):
            return subprocess.run([str(helper), *map(str, args)], check=True,
                                  capture_output=True, timeout=30).stdout

        call("probe")
        call("study", source, folder, 1)
        page = json.loads((folder / "page.json").read_text(encoding="utf-8"))
        if page["count"] != 1 or "Coursive package check" not in page["text"]:
            raise RuntimeError("Packaged helper did not extract the expected PDF page")
        if not call("render", source, work, 1).startswith(PNG_HEADER):
            raise RuntimeError("Packaged helper did not render a PNG")
        if not json.loads(call("info", source, work))["sizes"]:
            raise RuntimeError("Packaged helper did not read the note page dimensions")


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--helper":
        smoke_helper(Path(sys.argv[2]))
    else:
        unittest.main()
