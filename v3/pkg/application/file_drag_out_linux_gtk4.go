//go:build linux && !gtk3 && !android && !server

package application

import (
	"github.com/ebitengine/purego"
	"unsafe"
)

const fileDragGTKLibrary = "libgtk-4.so.1"
const fileDragButtonPress, fileDragButtonRelease = 2, 3

var fileDragGTK4 struct {
	width         func(unsafe.Pointer) int32
	height        func(unsafe.Pointer) int32
	allocate      func(unsafe.Pointer, int32, int32, int32, unsafe.Pointer)
	remove        func(unsafe.Pointer, unsafe.Pointer)
	append        func(unsafe.Pointer, unsafe.Pointer)
	setChild      func(unsafe.Pointer, unsafe.Pointer)
	newArea       func() unsafe.Pointer
	newLegacy     func() unsafe.Pointer
	newSource     func() unsafe.Pointer
	addController func(unsafe.Pointer, unsafe.Pointer)
	setActions    func(unsafe.Pointer, uint32)
	setContent    func(unsafe.Pointer, unsafe.Pointer)
	bytes         func(unsafe.Pointer, uintptr) unsafe.Pointer
	bytesUnref    func(unsafe.Pointer)
	provider      func(string, unsafe.Pointer) unsafe.Pointer
	action        func(unsafe.Pointer) uint32
	icon          func(unsafe.Pointer) unsafe.Pointer
	image         func(string) unsafe.Pointer
	picture       func(string) unsafe.Pointer
	setIconChild  func(unsafe.Pointer, unsafe.Pointer)
}

var fileDragGTK4Click = purego.NewCallback(func(_ unsafe.Pointer, event unsafe.Pointer, id uintptr) int32 {
	observeLinuxFileDragClick(id, event)
	return 0
})
var fileDragGTK4Finished = purego.NewCallback(func(_ unsafe.Pointer, drag unsafe.Pointer, _ int32, _ uintptr) {
	completeLinuxFileDrag(drag, fileDragGTK4.action(drag) != 0)
})
var fileDragGTK4Cancelled = purego.NewCallback(func(_ unsafe.Pointer, drag unsafe.Pointer, _ int32, _ uintptr) int32 {
	cancelLinuxFileDrag(drag)
	return 0
})

func loadFileDragGTKPlatform() {
	for name, fn := range map[string]any{
		"gtk_widget_get_width": &fileDragGTK4.width, "gtk_widget_get_height": &fileDragGTK4.height,
		"gtk_widget_allocate": &fileDragGTK4.allocate,
		"gtk_box_remove":      &fileDragGTK4.remove, "gtk_box_append": &fileDragGTK4.append,
		"gtk_overlay_set_child": &fileDragGTK4.setChild, "gtk_drawing_area_new": &fileDragGTK4.newArea,
		"gtk_event_controller_legacy_new": &fileDragGTK4.newLegacy,
		"gdk_event_ref":                   &fileDragGTK.copyEvent, "gdk_event_unref": &fileDragGTK.freeEvent,
		"gtk_drag_source_new": &fileDragGTK4.newSource, "gtk_widget_add_controller": &fileDragGTK4.addController,
		"gtk_drag_source_set_actions": &fileDragGTK4.setActions, "gtk_drag_source_set_content": &fileDragGTK4.setContent,
		"g_bytes_new": &fileDragGTK4.bytes, "g_bytes_unref": &fileDragGTK4.bytesUnref,
		"gdk_content_provider_new_for_bytes": &fileDragGTK4.provider, "gdk_drag_get_selected_action": &fileDragGTK4.action,
		"gtk_drag_icon_get_for_drag": &fileDragGTK4.icon, "gtk_image_new_from_icon_name": &fileDragGTK4.image,
		"gtk_picture_new_for_filename": &fileDragGTK4.picture, "gtk_drag_icon_set_child": &fileDragGTK4.setIconChild,
	} {
		purego.RegisterLibFunc(fn, fileDragGTK.handle, name)
	}
}

func (w *linuxWebviewWindow) showNativeFileDragOutOverlay() {
	fileDragGTK.visible(w.fileDragOutOverlay, 1)
	view := unsafe.Pointer(w.webview)
	fileDragGTK4.allocate(w.fileDragOutOverlay, fileDragGTK4.width(view), fileDragGTK4.height(view), -1, nil)
}

func (w *linuxWebviewWindow) installNativeFileDragOutOverlay() {
	view := unsafe.Pointer(w.webview)
	parent := fileDragGTK.parent(view)
	fileDragGTK.ref(view)
	fileDragGTK4.remove(parent, view)
	overlay := fileDragGTK.newOverlay()
	fileDragGTK4.setChild(overlay, view)
	fileDragGTK.unref(view)
	fileDragGTK.expandX(overlay, 1)
	fileDragGTK.expandY(overlay, 1)
	fileDragGTK4.append(parent, overlay)
	w.fileDragOutOverlay = fileDragGTK4.newArea()
	fileDragGTK.addOverlay(overlay, w.fileDragOutOverlay)
	legacy := fileDragGTK4.newLegacy()
	fileDragGTK.connect(legacy, "event", fileDragGTK4Click, uintptr(w.parent.id), 0, 0)
	fileDragGTK4.addController(w.fileDragOutOverlay, legacy)
	source := fileDragGTK4.newSource()
	fileDragGTK4.setActions(source, 1)
	fileDragGTK.connect(source, "drag-begin", beginLinuxFileDrag, uintptr(w.parent.id), 0, 0)
	fileDragGTK.connect(source, "drag-end", fileDragGTK4Finished, 0, 0, 0)
	fileDragGTK.connect(source, "drag-cancel", fileDragGTK4Cancelled, 0, 0, 0)
	fileDragGTK4.addController(w.fileDragOutOverlay, source)
	w.fileDragOutSource = source
}

func (w *linuxWebviewWindow) prepareNativeFileDragOut() {
	data := []byte(fileDragOutURIs(w.fileDragOut.paths))
	bytes := fileDragGTK4.bytes(unsafe.Pointer(&data[0]), uintptr(len(data)))
	provider := fileDragGTK4.provider("text/uri-list", bytes)
	fileDragGTK4.bytesUnref(bytes)
	fileDragGTK4.setContent(w.fileDragOutSource, provider)
	fileDragGTK.unref(provider)
}

func (w *linuxWebviewWindow) clearNativeFileDragOut() {
	fileDragGTK4.setContent(w.fileDragOutSource, nil)
}

func (w *linuxWebviewWindow) replayFileDragOutClick(press, release unsafe.Pointer) {
	display := fileDragGTK.display(unsafe.Pointer(w.webview))
	fileDragGTK.putEvent(display, press)
	fileDragGTK.putEvent(display, release)
}
func (w *linuxWebviewWindow) setNativeFileDragIcon(drag unsafe.Pointer) {
	var image unsafe.Pointer
	if w.fileDragOut.imagePath != "" {
		image = fileDragGTK4.picture(w.fileDragOut.imagePath)
	} else {
		image = fileDragGTK4.image("text-x-generic")
	}
	fileDragGTK4.setIconChild(fileDragGTK4.icon(drag), image)
}
