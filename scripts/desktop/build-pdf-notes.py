"""Package pdf-notes.py with the installed pypdf, retaining its license."""
import sys
import zipfile
from pathlib import Path
from importlib.metadata import distribution

dist=distribution("pypdf")
out=Path(sys.argv[1])
with out.open("wb") as stream:
    stream.write(b"#!/usr/bin/python3\n")
    with zipfile.ZipFile(stream,"w",compression=zipfile.ZIP_DEFLATED) as archive:
        archive.write(Path(__file__).with_name("pdf-notes.py"),"__main__.py")
        typing=distribution("typing_extensions")
        for member in typing.files:
            if str(member)=="typing_extensions.py" or "/licenses/LICENSE" in str(member):
                archive.write(typing.locate_file(member),str(member))
        for member in dist.files:
            name=str(member)
            if name.startswith("pypdf/") and name.endswith(".py") or name.endswith("/licenses/LICENSE"):
                archive.write(dist.locate_file(member),name)
out.chmod(0o755)
