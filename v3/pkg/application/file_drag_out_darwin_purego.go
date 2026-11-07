//go:build darwin && purego && !ios && !server

package application

import (
	"github.com/ebitengine/purego/objc"
	"sync"
)

type puregoFileDrag struct {
	window    *macosWebviewWindow
	paths     []string
	imagePath string
	monitor   id
	pressed   bool
	origin    NSPoint
	active    bool
}

var puregoFileDrags sync.Map       // NSDraggingSource -> *puregoFileDrag
var puregoFileDragWindows sync.Map // window ID -> *puregoFileDrag
var puregoFileDragClassOnce sync.Once
var puregoFileDragClass id

func registerFileDragSource() id {
	puregoFileDragClassOnce.Do(func() {
		puregoFileDragClass = registerDelegateClass("WailsFileDragSource", "NSObject", nil, []objc.MethodDef{
			{Cmd: sel_("draggingSession:sourceOperationMaskForDraggingContext:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID, _ int) uint { return 1 }},
			{Cmd: sel_("draggingSession:endedAtPoint:operation:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID, _ NSPoint, operation uint) {
				value, ok := puregoFileDrags.LoadAndDelete(uintptr(self))
				if !ok {
					return
				}
				drag := value.(*puregoFileDrag)
				puregoFileDragWindows.Delete(drag.window.parent.id)
				drag.window.parent.fileDragOutEnded(operation != 0)
				id(self).send("release")
			}},
		})
	})
	return puregoFileDragClass
}

func (w *macosWebviewWindow) armFileDragOut(paths []string, imagePath string) {
	InvokeAsync(func() {
		if w.parent.isDestroyed() {
			return
		}
		if value, ok := puregoFileDragWindows.Load(w.parent.id); ok && value.(*puregoFileDrag).active {
			return
		}
		w.clearPuregoFileDrag()
		drag := &puregoFileDrag{window: w, paths: paths, imagePath: imagePath}
		puregoFileDragWindows.Store(w.parent.id, drag)
		app := class("NSApplication").send("sharedApplication")
		event := app.send("currentEvent")
		if get[uint](class("NSEvent"), "pressedMouseButtons")&1 != 0 && event != 0 {
			typ := get[uint](event, "type")
			if typ == 1 || typ == 6 {
				drag.begin(event)
				return
			}
		}
		block := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
			e := id(event)
			if e.send("window").ptr() != uintptr(w.nsWindow) {
				return event
			}
			typ := get[uint](e, "type")
			point := get[NSPoint](e, "locationInWindow")
			if typ == 1 {
				drag.pressed = true
				drag.origin = point
			}
			if typ == 2 {
				drag.pressed = false
			}
			if typ == 6 && drag.pressed {
				dx, dy := point.X-drag.origin.X, point.Y-drag.origin.Y
				if dx*dx+dy*dy >= 16 {
					drag.begin(e)
					return 0
				}
			}
			return event
		})
		drag.monitor = class("NSEvent").send("addLocalMonitorForEventsMatchingMask:handler:", uint64((1<<1)|(1<<2)|(1<<6)), block)
		block.Release()
	})
}

func (w *macosWebviewWindow) clearPuregoFileDrag() {
	if value, ok := puregoFileDragWindows.Load(w.parent.id); ok {
		drag := value.(*puregoFileDrag)
		if drag.monitor != 0 {
			class("NSEvent").send("removeMonitor:", drag.monitor)
			drag.monitor = 0
		}
		if !drag.active {
			puregoFileDragWindows.Delete(w.parent.id)
		}
	}
}

func (w *macosWebviewWindow) disarmFileDragOut() { InvokeAsync(w.clearPuregoFileDrag) }

func (d *puregoFileDrag) begin(event id) {
	if d.active {
		return
	}
	d.active = true
	d.window.clearPuregoFileDrag()
	view := id(uintptr(d.window.wkWebView))
	point := get[NSPoint](view, "convertPoint:fromView:", get[NSPoint](event, "locationInWindow"), objc.ID(0))
	items := class("NSMutableArray").send("arrayWithCapacity:", uint(len(d.paths)))
	var image id
	if d.imagePath != "" {
		image = class("NSImage").send("alloc").send("initWithContentsOfFile:", nsString(d.imagePath))
	}
	if image == 0 {
		image = class("NSWorkspace").send("sharedWorkspace").send("iconForFile:", nsString(d.paths[0]))
		image.send("retain")
	}
	defer image.send("release")
	for _, path := range d.paths {
		url := class("NSURL").send("fileURLWithPath:", nsString(path))
		item := class("NSDraggingItem").send("alloc").send("initWithPasteboardWriter:", url)
		item.send("setDraggingFrame:contents:", rect(CGFloat(point.X), CGFloat(point.Y), 64, 64), image)
		items.send("addObject:", item)
		item.send("release")
	}
	source := registerFileDragSource().send("new")
	puregoFileDrags.Store(source.ptr(), d)
	view.send("beginDraggingSessionWithItems:event:source:", items, event, source)
}
