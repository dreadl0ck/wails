//go:build darwin && !ios

#import <AppKit/AppKit.h>

// WebviewDragOut is a transparent overlay view that implements NSDraggingSource.
//
// It enables "drag-out": dragging one or more files from the webview out to
// other applications (Finder, DAWs such as Ableton Live / Pro Tools, etc.) as a
// native file drag. Because macOS requires a drag session to be started
// synchronously from within a real mouse event, the frontend must "arm" the
// view with the file paths (and an optional drag image) *before* the user
// begins dragging. When armed, the view becomes hit-testable and starts an
// NSDraggingSession on the next mouse drag. When not armed it is transparent to
// all pointer events so normal webview interaction is unaffected.
@interface WebviewDragOut : NSView <NSDraggingSource> {
    NSArray<NSString *> *_files;
    NSImage *_dragImage;
    BOOL _armed;
}
@property unsigned int windowId;

// arm makes the view hit-testable and stores the files/image to drag.
- (void)arm:(NSArray<NSString *> *)files image:(NSImage *)image;
// disarm makes the view transparent to pointer events again.
- (void)disarm;
@end
