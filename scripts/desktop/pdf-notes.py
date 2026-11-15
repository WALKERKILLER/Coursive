"""Local Mac PDF editor. Render the opening revision using the PDFKit helper.

The installer packages this module with pypdf (BSD-3-Clause, see its LICENSE).
Page coordinates from Flutter use the displayed crop width/height as units,
measured from the top left, allowing x > 1 on an expanded note margin.
"""
import fcntl
import hashlib
import json
import math
import os
import subprocess
import sys
import tempfile
from pathlib import Path

from pypdf import PdfReader, PdfWriter
from pypdf.generic import ArrayObject, DictionaryObject, FloatObject, NameObject, NumberObject, RectangleObject


def digest(file):
    h = hashlib.sha256()
    with file.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def array(values):
    return ArrayObject([FloatObject(v) for v in values])


def geometry(page):
    box = page.cropbox
    w, h = float(box.width), float(box.height)
    return (h, w) if page.rotation % 180 else (w, h)


def pdf_point(box, rotation, x, y):
    u, v = {0: (x, 1-y), 90: (y, x), 180: (1-x, y), 270: (1-y, 1-x)}[rotation % 360]
    return float(box.left) + u * float(box.width), float(box.bottom) + v * float(box.height)


def enlarged(box, rotation, fraction):
    x0, y0, x1, y1 = map(float, box)
    if rotation % 360 == 0:
        x1 += (x1-x0) * fraction
    elif rotation % 360 == 90:
        y1 += (y1-y0) * fraction
    elif rotation % 360 == 180:
        x0 -= (x1-x0) * fraction
    else:
        y0 -= (y1-y0) * fraction
    return RectangleObject((x0, y0, x1, y1))


def add_stroke(writer, original, item, margin):
    index = item["page"] - 1
    if not 0 <= index < len(original.pages):
        raise ValueError("页码无效")
    page = original.pages[index]
    pts = item["points"]
    if not pts or not all(math.isfinite(x) and math.isfinite(y) and 0<=x<=1+margin/100+1e-7 and 0<=y<=1 for x,y in pts):
        raise ValueError("笔迹超出页面")
    points = [pdf_point(page.cropbox, page.rotation, x, y) for x,y in pts]
    if all(p == points[0] for p in points):
        points.append((points[0][0]+0.05, points[0][1]+0.05))
    xs, ys = zip(*points)
    argb = int(item.get("argb", 0xff1f3b91))
    fraction = float(item.get("width", .003))
    if not math.isfinite(fraction) or fraction <= 0 or not 0 <= argb <= 0xffffffff:
        raise ValueError("笔迹样式无效")
    width = geometry(page)[0] * fraction
    rgb = [((argb >> shift) & 255) / 255 for shift in (16, 8, 0)]
    opacity = (argb >> 24) / 255
    square = bool(item.get("square"))
    marker = bool(item.get("marker"))
    rect = (min(xs)-width,min(ys)-width,max(xs)+width,max(ys)+width)
    annot = DictionaryObject({
        NameObject("/Type"): NameObject("/Annot"), NameObject("/Subtype"): NameObject("/Ink"),
        NameObject("/CoursiveInk"): NumberObject(1),
        NameObject("/Rect"): array(rect), NameObject("/F"): NumberObject(4),
        NameObject("/C"): array(rgb),
        NameObject("/CoursiveARGB"): NumberObject(argb),
        NameObject("/CoursiveWidth"): FloatObject(fraction),
        NameObject("/CoursiveSquare"): NumberObject(int(square)),
        NameObject("/CoursiveMarker"): NumberObject(int(marker)),
        NameObject("/BS"): DictionaryObject({NameObject("/W"):FloatObject(width),NameObject("/S"):NameObject("/S")}),
        NameObject("/InkList"): ArrayObject([array(v for p in points for v in p)]),
    })
    # Explicit appearance for readers that don't synthesize ink appearances.
    from pypdf.generic import DecodedStreamObject
    appearance=DecodedStreamObject()
    appearance.update({NameObject("/Type"):NameObject("/XObject"),NameObject("/Subtype"):NameObject("/Form"),
                       NameObject("/BBox"):array([0,0,rect[2]-rect[0],rect[3]-rect[1]]),
                       NameObject("/Resources"):DictionaryObject()})
    style = DictionaryObject({NameObject("/Type"): NameObject("/ExtGState"),
                            NameObject("/CA"): FloatObject(opacity),
                            NameObject("/BM"): NameObject("/Darken" if marker else "/Normal")})
    appearance["/Resources"][NameObject("/ExtGState")] = DictionaryObject({NameObject("/Pen"): style})
    ops = [f"/Pen gs {rgb[0]:.6f} {rgb[1]:.6f} {rgb[2]:.6f} RG {width:.5f} w {2 if square else 1} J 1 j"]
    for i,(x,y) in enumerate(points):
        ops.append(f"{x-rect[0]:.5f} {y-rect[1]:.5f} {'l' if i else 'm'}")
    appearance.set_data(("\n".join(ops)+"\nS\n").encode())
    annot[NameObject("/AP")]=DictionaryObject({NameObject("/N"):writer._add_object(appearance)})
    writer.add_annotation(index,annot)


def managed(page):
    for i, ref in enumerate(page.get("/Annots") or []):
        annotation = ref.get_object()
        if annotation.get("/Subtype") == "/Ink" and annotation.get("/CoursiveInk") == 1:
            yield i, annotation


def read_points(page, coords):
    box = page.cropbox
    result = []
    for x,y in zip(coords[::2],coords[1::2]):
        u = (float(x)-float(box.left))/float(box.width)
        v = (float(y)-float(box.bottom))/float(box.height)
        a,b = {0:(u,1-v),90:(v,u),180:(1-u,v),270:(1-v,1-u)}[page.rotation%360]
        result.append([a,b])
    return result


def handle(op, source, dest):
    original=PdfReader(source)
    if original.is_encrypted or not 1 <= len(original.pages) <= 5000:
        raise ValueError("无法编辑加密 PDF 或超大文档")
    exists=dest.exists()
    doc=PdfReader(dest) if exists else original
    origin=digest(source)
    meta=doc.metadata or {}
    if exists and (meta.get("/CoursiveSourceSHA256")!=origin or len(doc.pages)!=len(original.pages)):
        raise ValueError("同名批注文件与当前原件不匹配")
    old=int(meta.get("/CoursiveRightMargin",0)) if exists else 0
    rev=digest(dest) if exists else ""
    if op=="info":
        ink = []
        if exists:
            for n,(page,ref) in enumerate(zip(doc.pages,original.pages),1):
                for index,mark in managed(page):
                    for coords in mark["/InkList"]:
                        ink.append(dict(page=n, index=index, points=read_points(ref,coords),
                            argb=int(mark.get("/CoursiveARGB", 0xff1f3b91)),
                            width=float(mark.get("/CoursiveWidth", .003)),
                            square=bool(mark.get("/CoursiveSquare", 0)), marker=bool(mark.get("/CoursiveMarker", 0))))
        return dict(ink=ink,revision=rev,margin=old,canExpand=all(list(p.cropbox)==list(p.mediabox) for p in original.pages),sizes=[geometry(p) for p in original.pages])
    edit=json.load(sys.stdin)
    if edit["revision"]!=rev:
        raise ValueError("批注文件已被其他窗口修改；当前笔迹仍在窗口中")
    margin=edit["margin"]
    if margin not in (0,25,50) or margin<old:
        raise ValueError("白边不可缩小，以免裁掉笔迹")
    writer=PdfWriter(clone_from=doc)
    if margin!=old:
        for page,ref in zip(writer.pages,original.pages):
            if list(ref.cropbox)!=list(ref.mediabox):
                raise ValueError("此文件含隐藏的裁剪区域，无法安全扩边；可在原页上书写")
            box=enlarged(ref.cropbox,ref.rotation,margin/100)
            page.mediabox=box;page.cropbox=RectangleObject(box)
    if exists and edit.get("replaceInk"):
        for page, ref in zip(writer.pages,original.pages):
            for i,_ in reversed(list(managed(page))):
                del page.annotations[i]
    for stroke in edit["strokes"]:
        add_stroke(writer,original,stroke,margin)
    writer.add_metadata({"/CoursiveSourceSHA256":origin,"/CoursiveRightMargin":str(margin)})
    with tempfile.NamedTemporaryFile(dir=dest.parent,prefix=".coursive-pdf-",delete=False) as tmp:
        staging=Path(tmp.name)
    try:
        writer.write(staging)
        if len(PdfReader(staging).pages)!=len(original.pages):
            raise ValueError("保存后页数不一致")
        os.replace(staging,dest)
    finally:
        staging.unlink(missing_ok=True)
    return dict(revision=digest(dest))


def main():
    op,src,dst,*rest=sys.argv[1:]
    source,dest=Path(src),Path(dst)
    if op=="render":
        renderer=Path(sys.argv[0]).resolve().with_name("coursive-pdf-notes-render")
        subprocess.run([renderer,dest if dest.is_file() and dest!=source else source,str(int(rest[0])), *rest[1:]],check=True)
        return
    if op not in ("info","save") or source.resolve()==dest.resolve():
        raise ValueError("无效的 PDF 操作")
    # A persistent, private lock path prevents two first writes from racing.
    lockdir=Path.home()/"Library/Caches/Coursive/PDFLocks"
    lockdir.mkdir(mode=0o700,parents=True,exist_ok=True)
    lock=lockdir/hashlib.sha256(os.fsencode(dest.resolve())).hexdigest()
    with lock.open("a") as stream:
        fcntl.flock(stream,fcntl.LOCK_EX if op=="save" else fcntl.LOCK_SH)
        print(json.dumps(handle(op,source,dest)))


if __name__=="__main__":
    try:
        main()
    except Exception as error:
        sys.exit(str(error))
