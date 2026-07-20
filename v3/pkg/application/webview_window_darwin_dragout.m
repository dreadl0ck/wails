//go:build darwin && !ios

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import "webview_window_darwin_dragout.h"

// Implemented in Go (see application_darwin.go).
extern void macosOnDragOutEnded(unsigned int windowId, bool performed);

@implementation WebviewDragOut

- (instancetype)initWithFrame:(NSRect)frameRect {
    self = [super initWithFrame:frameRect];
    if (self) {
        _files = nil;
        _dragImage = nil;
        _armed = NO;
    }
    return self;
}

- (void)dealloc {
    [_files release];
    [_dragImage release];
    [super dealloc];
}

- (void)arm:(NSArray<NSString *> *)files image:(NSImage *)image {
    NSArray<NSString *> *oldFiles = _files;
    NSImage *oldImage = _dragImage;
    _files = [files retain];
    _dragImage = [image retain];
    _armed = (files != nil && [files count] > 0);
    [oldFiles release];
    [oldImage release];
}

- (void)disarm {
    _armed = NO;
    NSArray<NSString *> *oldFiles = _files;
    NSImage *oldImage = _dragImage;
    _files = nil;
    _dragImage = nil;
    [oldFiles release];
    [oldImage release];
}

// hitTest: return nil unless armed, so normal clicks/scroll pass through to the
// webview. When armed we intercept the pointer so we can start the drag session.
- (NSView *)hitTest:(NSPoint)point {
    if (!_armed) {
        return nil;
    }
    return [super hitTest:point];
}

// buildDragItems builds the NSDraggingItems for the currently armed files,
// centred on the given point (in this view's coordinates).
- (NSArray<NSDraggingItem *> *)buildDragItemsAtPoint:(NSPoint)mouseInView {
    NSMutableArray<NSDraggingItem *> *dragItems = [NSMutableArray arrayWithCapacity:[_files count]];

    NSImage *image = _dragImage;
    if (image == nil) {
        // Fall back to the generic document icon.
        image = [[NSWorkspace sharedWorkspace] iconForFileType:NSFileTypeForHFSTypeCode(kGenericDocumentIcon)];
    }
    NSSize imageSize = image.size;
    if (imageSize.width <= 0 || imageSize.height <= 0) {
        imageSize = NSMakeSize(64, 64);
    }

    for (NSUInteger i = 0; i < [_files count]; i++) {
        NSString *path = _files[i];
        NSURL *fileURL = [NSURL fileURLWithPath:path];
        if (fileURL == nil) {
            continue;
        }
        NSDraggingItem *item = [[[NSDraggingItem alloc] initWithPasteboardWriter:fileURL] autorelease];
        // Stack multiple items with a small offset so the count is visible.
        CGFloat offset = (CGFloat)i * 6.0;
        NSRect frame = NSMakeRect(mouseInView.x - imageSize.width / 2.0 + offset,
                                  mouseInView.y - imageSize.height / 2.0 - offset,
                                  imageSize.width,
                                  imageSize.height);
        [item setDraggingFrame:frame contents:image];
        [dragItems addObject:item];
    }
    return dragItems;
}

// mouseDown: while armed, we don't immediately steal the click. We run a short
// event-tracking loop to see whether the user drags (start a native file drag)
// or releases without moving (a plain click, which we forward to the webview so
// row selection still works). This keeps normal clicking intact even though the
// overlay is hit-testable while armed.
- (void)mouseDown:(NSEvent *)event {
    if (!_armed || _files == nil || [_files count] == 0) {
        [super mouseDown:event];
        return;
    }

    NSPoint startInWindow = event.locationInWindow;
    const CGFloat dragThreshold = 4.0; // points

    NSEvent *nextEvent = event;
    BOOL dragged = NO;

    while (nextEvent) {
        nextEvent = [[self window] nextEventMatchingMask:(NSEventMaskLeftMouseDragged | NSEventMaskLeftMouseUp)];
        if (nextEvent == nil) {
            break;
        }
        if ([nextEvent type] == NSEventTypeLeftMouseUp) {
            break;
        }
        // LeftMouseDragged: check whether we've moved past the threshold.
        NSPoint now = nextEvent.locationInWindow;
        CGFloat dx = now.x - startInWindow.x;
        CGFloat dy = now.y - startInWindow.y;
        if ((dx * dx + dy * dy) >= (dragThreshold * dragThreshold)) {
            dragged = YES;
            break;
        }
    }

    if (!dragged) {
        // Treat as a click: forward the original mousedown to whatever is
        // beneath the overlay (the webview) so selection/click handlers run.
        NSView *contentView = [[self window] contentView];
        NSView *below = nil;
        if (contentView) {
            // Temporarily disarm hit-testing so we find the view underneath.
            BOOL wasArmed = _armed;
            _armed = NO;
            below = [contentView hitTest:[contentView convertPoint:startInWindow fromView:nil]];
            _armed = wasArmed;
        }
        if (below && below != self) {
            [below mouseDown:event];
        } else {
            [super mouseDown:event];
        }
        return;
    }

    NSPoint mouseInView = [self convertPoint:(nextEvent ? nextEvent.locationInWindow : startInWindow) fromView:nil];
    NSArray<NSDraggingItem *> *dragItems = [self buildDragItemsAtPoint:mouseInView];
    if ([dragItems count] == 0) {
        [super mouseDown:event];
        return;
    }

    [self beginDraggingSessionWithItems:dragItems event:(nextEvent ?: event) source:self];
}

#pragma mark - NSDraggingSource

- (NSDragOperation)draggingSession:(NSDraggingSession *)session
    sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
    // Files are dragged out as copies (DAWs/Finder import a copy reference).
    return NSDragOperationCopy;
}

- (void)draggingSession:(NSDraggingSession *)session
           endedAtPoint:(NSPoint)screenPoint
              operation:(NSDragOperation)operation {
    BOOL performed = (operation != NSDragOperationNone);
    // Always disarm after a drag ends so we don't keep intercepting the pointer.
    [self disarm];
    macosOnDragOutEnded(self.windowId, performed);
}

@end
