param(
    [Parameter(Mandatory)][string]$Destination,
    [string]$Python = 'python'
)

$ErrorActionPreference = 'Stop'
$build = Join-Path ([IO.Path]::GetTempPath()) ('coursive-pdf-' + [guid]::NewGuid())
New-Item -ItemType Directory $build | Out-Null

try {
    & $Python -m venv (Join-Path $build 'venv')
    if ($LASTEXITCODE -ne 0) { throw 'Creating the PDF helper environment failed' }
    $venvPython = Join-Path $build 'venv\Scripts\python.exe'
    & $venvPython -m pip install --disable-pip-version-check -r (Join-Path $PSScriptRoot 'pdf-windows-requirements.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Installing the PDF helper dependencies failed' }

    Push-Location $build
    try {
        & $venvPython -m PyInstaller --noconfirm --onedir --name coursive-pdf-notes `
            --collect-all pypdfium2 --collect-all pypdfium2_raw `
            --copy-metadata pypdfium2 --copy-metadata pypdf --copy-metadata Pillow `
            (Join-Path $PSScriptRoot 'pdf-notes.py')
        if ($LASTEXITCODE -ne 0) { throw 'Building the PDF helper failed' }
    } finally {
        Pop-Location
    }

    $bundle = Join-Path $build 'dist\coursive-pdf-notes'
    & $venvPython (Join-Path $PSScriptRoot 'test_pdf_windows.py') --helper (Join-Path $bundle 'coursive-pdf-notes.exe')
    if ($LASTEXITCODE -ne 0) { throw 'Packaged PDF helper smoke test failed' }

    if (Test-Path $Destination) { throw "PDF helper destination already exists: $Destination" }
    Copy-Item -Recurse $bundle $Destination
} finally {
    Remove-Item -Recurse -Force $build
}
