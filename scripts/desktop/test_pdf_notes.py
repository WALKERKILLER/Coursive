"""Run with the pinned pdf-notes requirements installed: python -m unittest discover -s scripts/desktop -p 'test_*.py'."""
import json
import subprocess
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path

from pypdf import PdfReader, PdfWriter
from pypdf.generic import ArrayObject, DictionaryObject, FloatObject, NameObject, NumberObject

HERE = Path(__file__).parent


class WorkingCopyTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.source = self.directory / "source.pdf"
        self.dest = self.directory / "notes.pdf"
        writer = PdfWriter()
        writer.add_blank_page(200, 100).rotate(90)
        writer.add_metadata({"/Title": "Synthetic lecture"})
        writer.add_outline_item("Chapter", 0)
        writer.write(self.source)
        self.original = self.source.read_bytes()

    def call(self, op, edit=None, helper=None):
        result = subprocess.run(
            [sys.executable, str(helper or HERE / "pdf-notes.py"), op, str(self.source), str(self.dest)],
            input=json.dumps(edit) if edit is not None else "",
            text=True, capture_output=True, check=False,
        )
        return result

    def test_rotated_margin_ink_preserves_source_outlines_and_foreign_annotations(self):
        initial = json.loads(self.call("info").stdout)
        self.assertEqual(initial["sizes"], [[100, 200]])
        stroke = {"page": 1, "points": [[.1, .2], [1.1, .8]], "argb": 0x77123456,
                  "width": .004, "marker": True, "square": True}
        saved = self.call("save", {"revision": "", "margin": 25, "replaceInk": True, "strokes": [stroke]})
        self.assertEqual(saved.returncode, 0, saved.stderr)
        old_revision = json.loads(saved.stdout)["revision"]
        info = json.loads(self.call("info").stdout)
        self.assertEqual(info["margin"], 25)
        self.assertEqual(info["ink"][0]["points"], stroke["points"])
        self.assertEqual(info["ink"][0]["argb"], stroke["argb"])
        self.assertTrue(info["ink"][0]["marker"])
        doc = PdfReader(self.dest)
        self.assertEqual(doc.metadata.title, "Synthetic lecture")
        self.assertEqual(doc.outline[0].title, "Chapter")
        self.assertEqual(float(doc.pages[0].cropbox.height), 125)
        self.assertIn("/AP", doc.pages[0].annotations[0].get_object())

        # A similarly colored annotation from another reader must remain owned
        # by that reader, even after erasing all Coursive strokes.
        writer = PdfWriter(clone_from=doc)
        foreign = DictionaryObject({NameObject("/Type"): NameObject("/Annot"),
            NameObject("/Subtype"): NameObject("/Ink"), NameObject("/F"): NumberObject(4),
            NameObject("/Rect"): ArrayObject([FloatObject(v) for v in [1, 1, 30, 30]]),
            NameObject("/C"): ArrayObject([FloatObject(v) for v in [.12, .23, .57]]),
            NameObject("/InkList"): ArrayObject([ArrayObject([FloatObject(v) for v in [2, 2, 20, 20]])])})
        writer.add_annotation(0, foreign)
        writer.write(self.dest)
        before = self.dest.read_bytes()
        stale = self.call("save", {"revision": old_revision, "margin": 25, "replaceInk": True, "strokes": []})
        self.assertNotEqual(stale.returncode, 0)
        self.assertEqual(self.dest.read_bytes(), before)
        current = json.loads(self.call("info").stdout)
        erased = self.call("save", {"revision": current["revision"], "margin": 25, "replaceInk": True, "strokes": []})
        self.assertEqual(erased.returncode, 0, erased.stderr)
        self.assertEqual(len(PdfReader(self.dest).pages[0].annotations), 1)
        self.assertEqual(self.source.read_bytes(), self.original)

    def test_cropped_page_refuses_expansion_and_publishes_no_working_copy(self):
        writer = PdfWriter(clone_from=self.source)
        writer.pages[0].cropbox.upper_right = (180, 80)
        writer.write(self.source)
        result = self.call("save", {"revision": "", "margin": 25, "replaceInk": True, "strokes": []})
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.dest.exists())

    def test_packaged_entry_contains_dependency_licenses_and_runs(self):
        helper = self.directory / "coursive-pdf-notes"
        subprocess.run([sys.executable, str(HERE / "build-pdf-notes.py"), str(helper)], check=True)
        with zipfile.ZipFile(helper) as archive:
            self.assertIn("__main__.py", archive.namelist())
            self.assertTrue(any("pypdf" in name and name.endswith("LICENSE") for name in archive.namelist()))
        result = self.call("info", helper=helper)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["revision"], "")


if __name__ == "__main__":
    unittest.main()
