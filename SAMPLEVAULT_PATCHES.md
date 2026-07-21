# SampleVault-local patches to this Wails fork

This fork (branch `feat/macos-file-dragout`) carries SampleVault-specific
patches on top of upstream Wails v3. All are marked in-code with a
`SAMPLEVAULT PATCH` comment. Full rationale lives in the app repo at
`docs/DRAG_IN.md` and `docs/DRAG_OUT.md`.

## Drag-OUT (NSDraggingSource)
Pre-existing feature work; see the branch history and `docs/DRAG_OUT.md`.

## Drag-IN multi-folder fix (added later)

Problem: dragging multiple folders from Finder silently did nothing because
`HandlePlatformFileDrop` resolved the drop target purely from
`document.elementFromPoint(x, y)`, and the native→CSS coordinate conversion
could place `y` outside the webview (multi-item drag badge offset + inset
title bar), so `elementFromPoint` returned null and the drop was dropped.

Files patched:

1. `v3/internal/runtime/desktop/@wailsio/runtime/src/window.ts`
   - `HandlePlatformFileDrop`: fall back to the first
     `[data-file-drop-target]` in the document when `elementFromPoint`
     misses; added `console.debug` tracing.
   - After editing, rebuild bundles: `wails3 task runtime:build`
     (regenerates `v3/internal/assetserver/bundledassets/runtime.js` and
     `runtime.debug.js`). Both bundle files are committed and reflect this
     patch.

2. `v3/pkg/application/webview_window_darwin_drag.m`
   - `WebviewDrag performDragOperation:`: convert the drop point against the
     WKWebView frame (via `[sender draggingDestinationWindow]`) instead of the
     window contentView, matching `webview_window_darwin.m`; added `NSLog`
     tracing. Also imports `webview_window_darwin.h` + `<WebKit/WebKit.h>`.
