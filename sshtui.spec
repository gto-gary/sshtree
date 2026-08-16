# -*- mode: python ; coding: utf-8 -*-

import sys

# --onefile is a single downloadable file, but on macOS it extracts to a
# fresh random temp dir on every launch, which defeats Gatekeeper's
# verification cache and adds several seconds to every single startup.
# --onedir (a folder of already-unpacked files) avoids that re-extraction,
# so macOS builds use it instead; Linux has no such penalty and keeps the
# single-file build.
onefile = sys.platform != "darwin"

a = Analysis(
    ['packaging/entry.py'],
    pathex=['.'],
    binaries=[],
    datas=[('sshtui/app.css', 'sshtui')],
    hiddenimports=[],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[],
    noarchive=False,
    optimize=0,
)
pyz = PYZ(a.pure)

if onefile:
    exe = EXE(
        pyz,
        a.scripts,
        a.binaries,
        a.datas,
        [],
        name='sshtui',
        debug=False,
        bootloader_ignore_signals=False,
        strip=False,
        upx=False,
        upx_exclude=[],
        runtime_tmpdir=None,
        console=True,
        disable_windowed_traceback=False,
        argv_emulation=False,
        target_arch=None,
        codesign_identity=None,
        entitlements_file=None,
    )
else:
    exe = EXE(
        pyz,
        a.scripts,
        [],
        exclude_binaries=True,
        name='sshtui',
        debug=False,
        bootloader_ignore_signals=False,
        strip=False,
        upx=False,
        console=True,
        disable_windowed_traceback=False,
        argv_emulation=False,
        target_arch=None,
        codesign_identity=None,
        entitlements_file=None,
    )
    coll = COLLECT(
        exe,
        a.binaries,
        a.datas,
        strip=False,
        upx=False,
        upx_exclude=[],
        name='sshtui',
    )
