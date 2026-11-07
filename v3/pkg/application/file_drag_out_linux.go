//go:build linux && !android && !server

package application

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var fileDragGTK struct {
	once       sync.Once
	err        error
	handle     uintptr
	connect    func(unsafe.Pointer, string, uintptr, uintptr, uintptr, uint32) uint64
	ref        func(unsafe.Pointer) unsafe.Pointer
	unref      func(unsafe.Pointer)
	visible    func(unsafe.Pointer, int32)
	expandX    func(unsafe.Pointer, int32)
	expandY    func(unsafe.Pointer, int32)
	newOverlay func() unsafe.Pointer
	addOverlay func(unsafe.Pointer, unsafe.Pointer)
	parent     func(unsafe.Pointer) unsafe.Pointer
	idle       func(uintptr, uintptr) uint32
	display    func(unsafe.Pointer) unsafe.Pointer
	putEvent   func(unsafe.Pointer, unsafe.Pointer)
	eventType  func(unsafe.Pointer) int32
	copyEvent  func(unsafe.Pointer) unsafe.Pointer
	freeEvent  func(unsafe.Pointer)
}

var fileDragLinuxWindows sync.Map  // armed window ID -> *linuxWebviewWindow
var fileDragLinuxSessions sync.Map // native drag -> *linuxWebviewWindow

var showLinuxFileDragOverlay = purego.NewCallback(func(id uintptr) int32 {
	if value, ok := fileDragLinuxWindows.Load(id); ok {
		w := value.(*linuxWebviewWindow)
		if w.fileDragOut != nil && !w.fileDragOut.active && !w.parent.isDestroyed() {
			w.showNativeFileDragOutOverlay()
		}
	}
	return 0
})

func loadFileDragGTK() error {
	fileDragGTK.once.Do(func() {
		defer func() {
			if err := recover(); err != nil {
				fileDragGTK.err = fmt.Errorf("load file drag-out APIs: %v", err)
			}
		}()
		fileDragGTK.handle, fileDragGTK.err = purego.Dlopen(fileDragGTKLibrary, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if fileDragGTK.err != nil {
			return
		}
		for name, fn := range map[string]any{
			"g_signal_connect_data": &fileDragGTK.connect, "g_object_ref": &fileDragGTK.ref, "g_object_unref": &fileDragGTK.unref,
			"gtk_widget_set_visible": &fileDragGTK.visible, "gtk_widget_set_hexpand": &fileDragGTK.expandX,
			"gtk_widget_set_vexpand": &fileDragGTK.expandY, "gtk_overlay_new": &fileDragGTK.newOverlay,
			"gtk_overlay_add_overlay": &fileDragGTK.addOverlay, "gtk_widget_get_parent": &fileDragGTK.parent,
			"g_idle_add": &fileDragGTK.idle, "gtk_widget_get_display": &fileDragGTK.display,
			"gdk_display_put_event": &fileDragGTK.putEvent, "gdk_event_get_event_type": &fileDragGTK.eventType,
		} {
			purego.RegisterLibFunc(fn, fileDragGTK.handle, name)
		}
		loadFileDragGTKPlatform()
	})
	return fileDragGTK.err
}

func (w *linuxWebviewWindow) installFileDragOutOverlay() {
	if err := loadFileDragGTK(); err != nil {
		globalApplication.error("File drag-out unavailable: %v", err)
		return
	}
	w.installNativeFileDragOutOverlay()
	fileDragGTK.expandX(w.fileDragOutOverlay, 1)
	fileDragGTK.expandY(w.fileDragOutOverlay, 1)
	fileDragGTK.visible(w.fileDragOutOverlay, 0)
}

func (w *linuxWebviewWindow) armFileDragOut(paths []string, imagePath string) {
	InvokeSync(func() {
		if w.parent.isDestroyed() || w.fileDragOutOverlay == nil || (w.fileDragOut != nil && w.fileDragOut.active) {
			return
		}
		w.clearFileDragOut()
		w.fileDragOut = &fileDragOutState{paths: paths, imagePath: imagePath}
		fileDragLinuxWindows.Store(uintptr(w.parent.id), w)
		w.prepareNativeFileDragOut()
		w.showNativeFileDragOutOverlay()
	})
}

func (w *linuxWebviewWindow) releaseFileDragPress() {
	if w.fileDragOutPress != nil {
		fileDragGTK.freeEvent(w.fileDragOutPress)
		w.fileDragOutPress = nil
	}
}

func (w *linuxWebviewWindow) clearFileDragOut() {
	if w.fileDragOut != nil && w.fileDragOut.active {
		return
	}
	fileDragLinuxWindows.Delete(uintptr(w.parent.id))
	w.releaseFileDragPress()
	if w.fileDragOutOverlay != nil && !w.parent.isDestroyed() {
		fileDragGTK.visible(w.fileDragOutOverlay, 0)
		w.clearNativeFileDragOut()
	}
	w.fileDragOut = nil
}

func (w *linuxWebviewWindow) disarmFileDragOut() { InvokeSync(w.clearFileDragOut) }

func observeLinuxFileDragClick(id uintptr, event unsafe.Pointer) {
	value, ok := fileDragLinuxWindows.Load(id)
	if !ok {
		return
	}
	w := value.(*linuxWebviewWindow)
	if w.fileDragOut == nil || w.fileDragOut.active {
		return
	}
	switch fileDragGTK.eventType(event) {
	case fileDragButtonPress:
		w.releaseFileDragPress()
		w.fileDragOutPress = fileDragGTK.copyEvent(event)
	case fileDragButtonRelease:
		if w.fileDragOutPress == nil {
			return
		}
		fileDragGTK.visible(w.fileDragOutOverlay, 0)
		w.replayFileDragOutClick(w.fileDragOutPress, event)
		w.releaseFileDragPress()
		fileDragGTK.idle(showLinuxFileDragOverlay, id)
	}
}

var beginLinuxFileDrag = purego.NewCallback(func(_ unsafe.Pointer, drag unsafe.Pointer, id uintptr) {
	value, ok := fileDragLinuxWindows.Load(id)
	if !ok {
		return
	}
	w := value.(*linuxWebviewWindow)
	if w.fileDragOut == nil {
		return
	}
	w.fileDragOut.active = true
	globalApplication.debug("Starting native file drag-out")
	w.releaseFileDragPress()
	fileDragLinuxSessions.Store(uintptr(drag), w)
	w.setNativeFileDragIcon(drag)
})

var finishLinuxFileDrag = purego.NewCallback(func(key uintptr) int32 {
	value, ok := fileDragLinuxSessions.LoadAndDelete(key)
	if !ok {
		return 0
	}
	w := value.(*linuxWebviewWindow)
	performed := w.fileDragOut.performed && !w.fileDragOut.cancelled
	w.fileDragOut.active = false
	w.clearFileDragOut()
	w.parent.fileDragOutEnded(performed)
	return 0
})

func completeLinuxFileDrag(drag unsafe.Pointer, performed bool) {
	value, ok := fileDragLinuxSessions.Load(uintptr(drag))
	if !ok {
		return
	}
	state := value.(*linuxWebviewWindow).fileDragOut
	if state == nil || state.ending {
		return
	}
	state.ending = true
	state.performed = performed
	fileDragGTK.idle(finishLinuxFileDrag, uintptr(drag))
}

func cancelLinuxFileDrag(drag unsafe.Pointer) {
	if value, ok := fileDragLinuxSessions.Load(uintptr(drag)); ok {
		if state := value.(*linuxWebviewWindow).fileDragOut; state != nil {
			state.cancelled = true
		}
	}
}
