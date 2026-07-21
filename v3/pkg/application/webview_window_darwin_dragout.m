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

    // If the left mouse button is already down when we arm (the common
    // case: the frontend arms from a pointermove during the very gesture
    // that should become the drag), start the drag session immediately
    // rather than waiting for a *subsequent* mouseDown. Waiting is what
    // made drag-out only work on the second gesture. beginDraggingSession
    // needs a mouse event; reuse the app's current event when it is a
    // mouse event, otherwise the armed mouseDown: path below still works.
    if (_armed && ([NSEvent pressedMouseButtons] & 1) != 0) {
        [self startDragFromCurrentEvent];
    }
}

// startDragFromCurrentEvent begins an NSDraggingSession using the app's
// current mouse event, centred at the current pointer location. No-op if
// there is no usable mouse event (the overlay then falls back to arming
// and starting on the next mouseDown:). Must run on the main thread.
- (void)startDragFromCurrentEvent {
    if (!_armed || _files == nil || [_files count] == 0) {
        return;
    }
    NSEvent *cur = [NSApp currentEvent];
    if (cur == nil) {
        return;
    }
    NSEventType t = [cur type];
    if (t != NSEventTypeLeftMouseDragged && t != NSEventTypeLeftMouseDown) {
        return;
    }
    NSPoint mouseInView = [self convertPoint:[cur locationInWindow] fromView:nil];
    NSArray<NSDraggingItem *> *dragItems = [self buildDragItemsAtPoint:mouseInView];
    if ([dragItems count] == 0) {
        return;
    }
    [self beginDraggingSessionWithItems:dragItems event:cur source:self];
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

// imageByAddingLabel composites a filename label into a rounded dark pill
// beneath the given (waveform / icon) image, returning a new image sized
// to fit both. This makes the drag preview show the sample's name as well
// as its waveform. Falls back to the original image when label is empty.
- (NSImage *)imageByAddingLabel:(NSString *)label toImage:(NSImage *)base {
    if (label == nil || [label length] == 0) {
        return base;
    }

    NSFont *font = [NSFont systemFontOfSize:11 weight:NSFontWeightMedium];
    NSMutableParagraphStyle *para = [[[NSMutableParagraphStyle alloc] init] autorelease];
    para.lineBreakMode = NSLineBreakByTruncatingMiddle;
    para.alignment = NSTextAlignmentCenter;
    NSDictionary *attrs = @{
        NSFontAttributeName: font,
        NSForegroundColorAttributeName: [NSColor colorWithSRGBRed:0.93 green:0.94 blue:0.96 alpha:1.0],
        NSParagraphStyleAttributeName: para,
    };

    NSSize baseSize = base.size;
    if (baseSize.width <= 0 || baseSize.height <= 0) {
        baseSize = NSMakeSize(64, 64);
    }

    const CGFloat padX = 8.0;   // pill horizontal padding
    const CGFloat padY = 3.0;   // pill vertical padding
    const CGFloat gap = 4.0;    // gap between waveform and pill
    const CGFloat maxLabelW = 220.0;

    NSSize textSize = [label sizeWithAttributes:attrs];
    CGFloat labelW = textSize.width;
    if (labelW > maxLabelW) {
        labelW = maxLabelW;
    }
    CGFloat pillW = labelW + padX * 2.0;
    CGFloat pillH = textSize.height + padY * 2.0;

    CGFloat outW = MAX(baseSize.width, pillW);
    CGFloat outH = baseSize.height + gap + pillH;

    NSImage *out = [[[NSImage alloc] initWithSize:NSMakeSize(outW, outH)] autorelease];
    [out lockFocus];

    // Draw the base image centred along the top.
    NSRect baseRect = NSMakeRect((outW - baseSize.width) / 2.0, pillH + gap, baseSize.width, baseSize.height);
    [base drawInRect:baseRect fromRect:NSZeroRect operation:NSCompositingOperationSourceOver fraction:1.0];

    // Draw the rounded pill background.
    NSRect pillRect = NSMakeRect((outW - pillW) / 2.0, 0, pillW, pillH);
    NSBezierPath *pill = [NSBezierPath bezierPathWithRoundedRect:pillRect xRadius:pillH / 2.0 yRadius:pillH / 2.0];
    [[NSColor colorWithSRGBRed:0.09 green:0.09 blue:0.11 alpha:0.94] setFill];
    [pill fill];

    // Draw the label text, clipped to the pill's inner width.
    NSRect textRect = NSMakeRect(pillRect.origin.x + padX, padY, pillW - padX * 2.0, textSize.height);
    [label drawInRect:textRect withAttributes:attrs];

    [out unlockFocus];
    return out;
}

// buildDragItems builds the NSDraggingItems for the currently armed files,
// centred on the given point (in this view's coordinates).
- (NSArray<NSDraggingItem *> *)buildDragItemsAtPoint:(NSPoint)mouseInView {
    NSMutableArray<NSDraggingItem *> *dragItems = [NSMutableArray arrayWithCapacity:[_files count]];

    NSImage *base = _dragImage;
    if (base == nil) {
        // Fall back to the generic document icon.
        base = [[NSWorkspace sharedWorkspace] iconForFileType:NSFileTypeForHFSTypeCode(kGenericDocumentIcon)];
    }

    // Build the drag label: the first file's name, plus a "+N" suffix
    // when dragging multiple files so the count is legible.
    NSString *label = [[_files firstObject] lastPathComponent];
    if (label == nil) {
        label = @"";
    }
    if ([_files count] > 1) {
        label = [label stringByAppendingFormat:@"  +%lu", (unsigned long)([_files count] - 1)];
    }

    NSImage *image = [self imageByAddingLabel:label toImage:base];
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
