//go:build windows && !server

package application

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/wailsapp/wails/v3/internal/webview2/pkg/edge"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

var (
	fileDragOutWindows      sync.Map
	fileDragOutShell        = syscall.NewLazyDLL("shell32.dll")
	createDragDataObject    = fileDragOutShell.NewProc("SHCreateDataObject")
	shellDoDragDrop         = fileDragOutShell.NewProc("SHDoDragDrop")
	fileDragOutOLE          = syscall.NewLazyDLL("ole32.dll")
	initializeFileDragOLE   = fileDragOutOLE.NewProc("OleInitialize")
	uninitializeFileDragOLE = fileDragOutOLE.NewProc("OleUninitialize")
	fileDragOutChildren     = syscall.NewLazyDLL("user32.dll").NewProc("EnumChildWindows")
	fileDragOutMouseUp      = syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		child := w32.HWND(hwnd)
		className := w32.GetClassName(child)
		globalApplication.debug("File drag input child", "class", className)
		if className != "Chrome_RenderWidgetHostHWND" {
			return 1
		}
		x, y, _ := w32.GetCursorPos()
		x, y, _ = w32.ScreenToClient(child, x, y)
		point := uintptr(uint16(x)) | uintptr(uint16(y))<<16
		w32.PostMessage(child, w32.WM_CANCELMODE, 0, 0)
		w32.PostMessage(child, w32.WM_LBUTTONUP, 0, point)
		return 0
	})
	fileDragOutTimer = syscall.NewCallback(func(hwnd uintptr, _ uint32, _ uintptr, _ uint32) uintptr {
		if value, ok := fileDragOutWindows.Load(hwnd); ok {
			value.(*windowsWebviewWindow).pollFileDragOut()
		}
		return 0
	})
)

const fileDragOutTimerID = 0x5744464f

func (w *windowsWebviewWindow) armFileDragOut(paths []string, imagePath string) {
	InvokeSync(func() {
		if w.parent.isDestroyed() || (w.fileDragOut != nil && w.fileDragOut.active) {
			return
		}
		if err := getNativeApplication().initializeFileDragApartment(); err != nil {
			globalApplication.error("File drag-out unavailable: %v", err)
			w.parent.fileDragOutEnded(false)
			return
		}
		w.clearFileDragOut()
		w.fileDragOut = &fileDragOutState{paths: paths, imagePath: imagePath}
		fileDragOutWindows.Store(uintptr(w.hwnd), w)
		w.fileDragOut.timer = w32.SetTimer(w.hwnd, fileDragOutTimerID, 16, fileDragOutTimer)
	})
}

func (w *windowsWebviewWindow) pollFileDragOut() {
	s := w.fileDragOut
	if s == nil || w.parent.isDestroyed() {
		w.clearFileDragOut()
		return
	}
	x, y, ok := w32.GetCursorPos()
	if !ok {
		return
	}
	if w32.GetForegroundWindow() != w.hwnd {
		s.pressed = false
		return
	}
	cx, cy, inside := w32.ScreenToClient(w.hwnd, x, y)
	bounds := w32.GetClientRect(w.hwnd)
	if !inside || bounds == nil || cx < 0 || cy < 0 || cx >= int(bounds.Right) || cy >= int(bounds.Bottom) {
		s.pressed = false
		return
	}
	down := w32.GetAsyncKeyState(w32.VK_LBUTTON)&0x8000 != 0
	if !down {
		s.pressed = false
		return
	}
	if !s.pressed {
		s.pressed = true
		s.x, s.y = x, y
		return
	}
	if x-s.x > 4 || s.x-x > 4 || y-s.y > 4 || s.y-y > 4 {
		w.beginFileDragOut()
	}
}

func (w *windowsWebviewWindow) clearFileDragOut() {
	w32.KillTimer(w.hwnd, fileDragOutTimerID)
	fileDragOutWindows.Delete(uintptr(w.hwnd))
	if w.fileDragOut != nil && !w.fileDragOut.active {
		w.fileDragOut = nil
	}
}

func (w *windowsWebviewWindow) disarmFileDragOut() { InvokeSync(w.clearFileDragOut) }

func (w *windowsWebviewWindow) beginFileDragOut() {
	s := w.fileDragOut
	if s == nil || s.active {
		return
	}
	s.active = true
	globalApplication.debug("Starting native file drag-out")
	w.clearFileDragOut()
	w32.ReleaseCapture()
	performed, err := dragWindowsFiles(w.hwnd, s.paths)
	w.fileDragOut = nil
	w.releaseFileDragMouse()
	if err != nil {
		globalApplication.error("File drag-out failed: %v", err)
	}
	w.parent.fileDragOutEnded(performed)
}

// OLE consumes mouse-up. Clear WebView2's pressed state before the next gesture.
func (w *windowsWebviewWindow) releaseFileDragMouse() {
	if w.parent.isDestroyed() || w.chromium == nil {
		return
	}
	if w.chromium.CompositionControllerReady() {
		x, y, _ := w32.GetCursorPos()
		x, y, _ = w32.ScreenToClient(w.hwnd, x, y)
		if err := w.chromium.SendMouseInput(edge.COREWEBVIEW2_MOUSE_EVENT_KIND_LEFT_BUTTON_UP, 0, 0, x, y); err != nil {
			globalApplication.error("Reset file drag mouse state: %v", err)
		}
		return
	}
	fileDragOutChildren.Call(uintptr(w.hwnd), fileDragOutMouseUp, 0)
}

// DROPFILES has a fixed 20-byte header on both 32- and 64-bit Windows.
func fileDragOutHDROP(paths []string) []byte {
	var names []uint16
	for _, path := range paths {
		names = append(names, utf16.Encode([]rune(path))...)
		names = append(names, 0)
	}
	names = append(names, 0)
	data := make([]byte, 20+2*len(names))
	binary.LittleEndian.PutUint32(data, 20)
	binary.LittleEndian.PutUint32(data[16:], 1)
	for i, c := range names {
		binary.LittleEndian.PutUint16(data[20+2*i:], c)
	}
	return data
}

func newWindowsFileDragData(paths []string) (*w32.IDataObject, error) {
	iid := syscall.GUID{Data1: 0x10e, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	var object *w32.IDataObject
	hr, _, _ := createDragDataObject.Call(0, 0, 0, 0, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&object)))
	if int32(hr) < 0 {
		return nil, fmt.Errorf("SHCreateDataObject: HRESULT %#x", uint32(hr))
	}
	failed := true
	defer func() {
		if failed {
			object.Vtbl.Release.Call(uintptr(unsafe.Pointer(object)))
		}
	}()
	data := fileDragOutHDROP(paths)
	memory := w32.GlobalAlloc(w32.GMEM_MOVEABLE, uint32(len(data)))
	if memory == 0 {
		return nil, fmt.Errorf("allocate file drag data")
	}
	owned := true
	defer func() {
		if owned {
			w32.GlobalFree(memory)
		}
	}()
	buffer := w32.GlobalLock(memory)
	if buffer == nil {
		return nil, fmt.Errorf("lock file drag data")
	}
	copy(unsafe.Slice((*byte)(buffer), len(data)), data)
	w32.GlobalUnlock(memory)
	format := w32.FORMATETC{CfFormat: w32.CF_HDROP, DwAspect: 1, Lindex: -1, Tymed: w32.TYMED_HGLOBAL}
	medium := struct {
		Tymed uint32
		Data  uintptr
		Owner uintptr
	}{Tymed: 1, Data: uintptr(memory)}
	hr, _, _ = object.Vtbl.SetData.Call(uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&medium)), 1)
	if int32(hr) < 0 {
		return nil, fmt.Errorf("set file drag data: HRESULT %#x", uint32(hr))
	}
	owned = false
	failed = false
	return object, nil
}

func dragWindowsFiles(hwnd w32.HWND, paths []string) (bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	object, err := newWindowsFileDragData(paths)
	if err != nil {
		return false, err
	}
	defer object.Vtbl.Release.Call(uintptr(unsafe.Pointer(object)))
	var effect uint32
	hr, _, _ := shellDoDragDrop.Call(uintptr(hwnd), uintptr(unsafe.Pointer(object)), 0, 1, uintptr(unsafe.Pointer(&effect)))
	if int32(hr) < 0 {
		return false, fmt.Errorf("SHDoDragDrop: HRESULT %#x", uint32(hr))
	}
	return hr == 0x40100 && effect != 0, nil
}

// OLE drag/drop state belongs to the UI apartment, not an individual session.
func (m *windowsApp) initializeFileDragApartment() error {
	if m.fileDragOLEInitialized {
		return nil
	}
	hr, _, _ := initializeFileDragOLE.Call(0)
	if int32(hr) < 0 {
		return fmt.Errorf("OleInitialize: HRESULT %#x", uint32(hr))
	}
	m.fileDragOLEInitialized = true
	return nil
}
