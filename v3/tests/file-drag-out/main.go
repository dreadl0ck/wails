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
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Wails file drag source", Width: 300, Height: 220, EnableFileDragOut: true,
	})
	app.Event.On("file-drag-out:ready", func(*application.CustomEvent) {
		app.Event.Emit("file-drag-out:files", os.Args[1:])
		fmt.Println("file-drag-out ready")
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
import { Events, Window } from '/wails/runtime.js';
let files = [], armed = false;
function arm() {
  if (armed || !files.length) return;
  armed = true;
  Window.ArmFileDragOut(files).then(() => Events.Emit('file-drag-out:trace', 'armed')).catch(error => { document.querySelector('#files').textContent = String(error); armed = false; });
}
Events.On('file-drag-out:files', event => { files = event.data; document.querySelector('#files').textContent = 'Drag ' + files.length + ' files out'; arm(); });
document.addEventListener('pointermove', event => { if (!(event.buttons & 1)) arm(); });
document.addEventListener('click', () => Events.Emit('file-drag-out:trace', 'click'));
window.addEventListener('wails:file-drag-out-ended', event => { armed = false; Events.Emit('file-drag-out:ended', event.detail); });
Events.Emit('file-drag-out:ready');
</script></html>`
