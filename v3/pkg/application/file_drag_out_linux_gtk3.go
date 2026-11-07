//go:build linux && gtk3 && !android && !server

package application

import (
	"github.com/ebitengine/purego"
	"unsafe"
)

const fileDragGTKLibrary = "libgtk-3.so.0"
const fileDragButtonPress, fileDragButtonRelease = 4, 7

var fileDragGTK3 struct {
	noShowAll     func(unsafe.Pointer, int32)
	width         func(unsafe.Pointer) int32
	height        func(unsafe.Pointer) int32
	allocate      func(unsafe.Pointer, unsafe.Pointer)
	remove        func(unsafe.Pointer, unsafe.Pointer)
	add           func(unsafe.Pointer, unsafe.Pointer)
	pack          func(unsafe.Pointer, unsafe.Pointer, int32, int32, uint32)
	newArea       func() unsafe.Pointer
	visibleWindow func(unsafe.Pointer, int32)
	events        func(unsafe.Pointer, uint32)
	window        func(unsafe.Pointer) unsafe.Pointer
	widgetEvent   func(unsafe.Pointer, unsafe.Pointer) int32
	setSource     func(unsafe.Pointer, uint32, unsafe.Pointer, int32, uint32)
	unsetSource   func(unsafe.Pointer)
	setTargets    func(unsafe.Pointer, unsafe.Pointer)
	newTargets    func(unsafe.Pointer, uint32) unsafe.Pointer
	addTarget     func(unsafe.Pointer, unsafe.Pointer, uint32, uint32)
	unrefTargets  func(unsafe.Pointer)
	atom          func(string, int32) unsafe.Pointer
	selection     func(unsafe.Pointer, unsafe.Pointer, int32, unsafe.Pointer, int32)
	action        func(unsafe.Pointer) uint32
	icon          func(unsafe.Pointer, string, int32, int32)
}

var fileDragGTK3Click = purego.NewCallback(func(_ unsafe.Pointer, event unsafe.Pointer, id uintptr) int32 {
	observeLinuxFileDragClick(id, event)
	return 0
})
var fileDragGTK3Data = purego.NewCallback(func(_ unsafe.Pointer, drag unsafe.Pointer, selection unsafe.Pointer, _ uint32, _ uint32, _ uintptr) {
	value, ok := fileDragLinuxSessions.Load(uintptr(drag))
	if !ok {
		return
	}
	s := value.(*linuxWebviewWindow).fileDragOut
	if s == nil {
		return
	}
	data := []byte(fileDragOutURIs(s.paths))
	fileDragGTK3.selection(selection, fileDragGTK3.atom("text/uri-list", 0), 8, unsafe.Pointer(&data[0]), int32(len(data)))
})
var fileDragGTK3End = purego.NewCallback(func(_ unsafe.Pointer, drag unsafe.Pointer, _ uintptr) {
	completeLinuxFileDrag(drag, fileDragGTK3.action(drag) != 0)
})
var fileDragGTK3Failed = purego.NewCallback(func(_ unsafe.Pointer, drag unsafe.Pointer, _ int32, _ uintptr) int32 {
	cancelLinuxFileDrag(drag)
	return 0
})

func loadFileDragGTKPlatform() {
	for name, fn := range map[string]any{
		"gtk_widget_set_no_show_all":     &fileDragGTK3.noShowAll,
		"gtk_widget_get_allocated_width": &fileDragGTK3.width, "gtk_widget_get_allocated_height": &fileDragGTK3.height,
		"gtk_widget_size_allocate": &fileDragGTK3.allocate,
		"gtk_container_remove":     &fileDragGTK3.remove, "gtk_container_add": &fileDragGTK3.add,
		"gtk_box_pack_start": &fileDragGTK3.pack, "gtk_event_box_new": &fileDragGTK3.newArea,
		"gtk_event_box_set_visible_window": &fileDragGTK3.visibleWindow, "gtk_widget_add_events": &fileDragGTK3.events,
		"gtk_widget_get_window": &fileDragGTK3.window, "gtk_widget_event": &fileDragGTK3.widgetEvent,
		"gdk_event_copy": &fileDragGTK.copyEvent, "gdk_event_free": &fileDragGTK.freeEvent,
		"gtk_drag_source_set": &fileDragGTK3.setSource, "gtk_drag_source_unset": &fileDragGTK3.unsetSource,
		"gtk_drag_source_set_target_list": &fileDragGTK3.setTargets, "gtk_target_list_new": &fileDragGTK3.newTargets,
		"gtk_target_list_add": &fileDragGTK3.addTarget, "gtk_target_list_unref": &fileDragGTK3.unrefTargets,
		"gdk_atom_intern": &fileDragGTK3.atom, "gtk_selection_data_set": &fileDragGTK3.selection,
		"gdk_drag_context_get_selected_action": &fileDragGTK3.action, "gtk_drag_set_icon_name": &fileDragGTK3.icon,
	} {
		purego.RegisterLibFunc(fn, fileDragGTK.handle, name)
	}
}

func (w *linuxWebviewWindow) installNativeFileDragOutOverlay() {
	view := unsafe.Pointer(w.webview)
	parent := fileDragGTK.parent(view)
	fileDragGTK.ref(view)
	fileDragGTK3.remove(parent, view)
	overlay := fileDragGTK.newOverlay()
	fileDragGTK3.add(overlay, view)
	fileDragGTK.unref(view)
	fileDragGTK3.pack(parent, overlay, 1, 1, 0)
	fileDragGTK.visible(overlay, 1)
	w.fileDragOutOverlay = fileDragGTK3.newArea()
	fileDragGTK3.noShowAll(w.fileDragOutOverlay, 1)
	fileDragGTK3.visibleWindow(w.fileDragOutOverlay, 0)
	fileDragGTK3.events(w.fileDragOutOverlay, (1<<8)|(1<<9))
	fileDragGTK.addOverlay(overlay, w.fileDragOutOverlay)
	fileDragGTK.connect(w.fileDragOutOverlay, "button-press-event", fileDragGTK3Click, uintptr(w.parent.id), 0, 0)
	fileDragGTK.connect(w.fileDragOutOverlay, "button-release-event", fileDragGTK3Click, uintptr(w.parent.id), 0, 0)
	fileDragGTK.connect(w.fileDragOutOverlay, "drag-begin", beginLinuxFileDrag, uintptr(w.parent.id), 0, 0)
	fileDragGTK.connect(w.fileDragOutOverlay, "drag-data-get", fileDragGTK3Data, 0, 0, 0)
	fileDragGTK.connect(w.fileDragOutOverlay, "drag-end", fileDragGTK3End, 0, 0, 0)
	fileDragGTK.connect(w.fileDragOutOverlay, "drag-failed", fileDragGTK3Failed, 0, 0, 0)
}

func (w *linuxWebviewWindow) showNativeFileDragOutOverlay() {
	fileDragGTK.visible(w.fileDragOutOverlay, 1)
	view := unsafe.Pointer(w.webview)
	allocation := [4]int32{0, 0, fileDragGTK3.width(view), fileDragGTK3.height(view)}
	fileDragGTK3.allocate(w.fileDragOutOverlay, unsafe.Pointer(&allocation[0]))
}

func (w *linuxWebviewWindow) prepareNativeFileDragOut() {
	fileDragGTK3.setSource(w.fileDragOutOverlay, 1<<8, nil, 0, 1<<1)
	targets := fileDragGTK3.newTargets(nil, 0)
	fileDragGTK3.addTarget(targets, fileDragGTK3.atom("text/uri-list", 0), 0, 0)
	fileDragGTK3.setTargets(w.fileDragOutOverlay, targets)
	fileDragGTK3.unrefTargets(targets)
}

func (w *linuxWebviewWindow) replayFileDragOutClick(press, release unsafe.Pointer) {
	for _, event := range []unsafe.Pointer{press, release} {
		copy := fileDragGTK.copyEvent(event)
		header := (*struct {
			Type   int32
			Window unsafe.Pointer
		})(copy)
		fileDragGTK.unref(header.Window)
		header.Window = fileDragGTK.ref(fileDragGTK3.window(unsafe.Pointer(w.webview)))
		fileDragGTK3.widgetEvent(unsafe.Pointer(w.webview), copy)
		fileDragGTK.freeEvent(copy)
	}
}

func (w *linuxWebviewWindow) clearNativeFileDragOut() { fileDragGTK3.unsetSource(w.fileDragOutOverlay) }
func (w *linuxWebviewWindow) setNativeFileDragIcon(drag unsafe.Pointer) {
	fileDragGTK3.icon(drag, "text-x-generic", 0, 0)
}
