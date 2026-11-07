package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	app := application.New(application.Options{
		Name:     "File drag-out test",
		LogLevel: slog.LevelDebug,
		Assets: application.AssetOptions{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, page)
		})},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Wails file drag source", Width: 300, Height: 220, EnableFileDragOut: true,
	})
	app.Event.On("file-drag-out:ready", func(*application.CustomEvent) {
		fmt.Printf("file-drag-out native: %d\n", uintptr(win.NativeWindow()))
		app.Event.Emit("file-drag-out:files", len(os.Args)-1)
		fmt.Println("file-drag-out ready")
	})
	app.Event.On("file-drag-out:arm", func(*application.CustomEvent) {
		if err := win.ArmFileDragOut(os.Args[1:], ""); err != nil {
			app.Event.Emit("file-drag-out:error", err.Error())
			return
		}
		app.Event.Emit("file-drag-out:armed")
	})
	app.Event.On("file-drag-out:ended", func(event *application.CustomEvent) {
		fmt.Printf("file-drag-out ended: %v\n", event.Data)
	})
	app.Event.On("file-drag-out:trace", func(event *application.CustomEvent) { fmt.Printf("file-drag-out trace: %v\n", event.Data) })
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

const page = `<!doctype html><html lang="en"><meta charset="utf-8"><title>File drag-out test</title>
<style>body{margin:0;display:grid;place-items:center;height:100vh;user-select:none;font:18px sans-serif;background:#eee}#files{padding:24px;border:2px dashed #777}</style>
<div id="files">Waiting for files…</div>
<script type="module">
import { Events } from '/wails/runtime.js';
let count = 0, armed = false, lastButtons = -1;
function arm() {
  if (armed || !count) return;
  armed = true;
  Events.Emit('file-drag-out:arm');
}
Events.On('file-drag-out:armed', () => Events.Emit('file-drag-out:trace', 'armed'));
Events.On('file-drag-out:error', event => { document.querySelector('#files').textContent = String(event.data); armed = false; });
Events.On('file-drag-out:files', event => { count = event.data; document.querySelector('#files').textContent = 'Drag ' + count + ' files out'; arm(); });
document.addEventListener('pointermove', event => {
  if (event.buttons !== lastButtons) { lastButtons = event.buttons; Events.Emit('file-drag-out:trace', {buttons: event.buttons, x: event.clientX, y: event.clientY}); }
  if (!(event.buttons & 1)) arm();
});
document.addEventListener('pointerup', arm);
document.addEventListener('click', () => { Events.Emit('file-drag-out:trace', 'click'); arm(); });
window.addEventListener('wails:file-drag-out-ended', event => { armed = false; Events.Emit('file-drag-out:ended', event.detail); });
Events.Emit('file-drag-out:ready');
</script></html>`
