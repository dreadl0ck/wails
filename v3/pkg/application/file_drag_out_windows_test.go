//go:build windows && !server

package application

import (
	"encoding/binary"
	"runtime"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/w32"
)

func TestFileDragOutHDROP(t *testing.T) {
	data := fileDragOutHDROP([]string{`C:\samples\kick.wav`, `D:\another folder\ü.wav`})
	if binary.LittleEndian.Uint32(data) != 20 || binary.LittleEndian.Uint32(data[16:]) != 1 {
		t.Fatal("invalid DROPFILES header")
	}
	var names []uint16
	for i := 20; i < len(data); i += 2 {
		names = append(names, binary.LittleEndian.Uint16(data[i:]))
	}
	if got := string(utf16.Decode(names)); got != "C:\\samples\\kick.wav\x00D:\\another folder\\ü.wav\x00\x00" {
		t.Fatalf("invalid file list %q", got)
	}
}

func TestFileDragOutWindowsDataObject(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	w32.OleInitialise()
	defer syscall.NewLazyDLL("ole32.dll").NewProc("OleUninitialize").Call()
	paths := []string{`C:\samples\kick.wav`, `D:\another folder\ü.wav`}
	object, err := newWindowsFileDragData(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer object.Vtbl.Release.Call(uintptr(unsafe.Pointer(object)))
	format := w32.FORMATETC{CfFormat: w32.CF_HDROP, DwAspect: 1, Lindex: -1, Tymed: w32.TYMED_HGLOBAL}
	var medium struct {
		Tymed uint32
		Data  uintptr
		Owner uintptr
	}
	hr, _, _ := object.Vtbl.GetData.Call(uintptr(unsafe.Pointer(object)), uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&medium)))
	if int32(hr) < 0 {
		t.Fatalf("GetData returned HRESULT %#x", uint32(hr))
	}
	defer syscall.NewLazyDLL("ole32.dll").NewProc("ReleaseStgMedium").Call(uintptr(unsafe.Pointer(&medium)))
	_, count := w32.DragQueryFile(medium.Data, 0xffffffff)
	if count != uint(len(paths)) {
		t.Fatalf("native data object returned %d files", count)
	}
	for i, want := range paths {
		got, _ := w32.DragQueryFile(medium.Data, uint(i))
		if got != want {
			t.Fatalf("native file path %q != %q", got, want)
		}
	}
}
