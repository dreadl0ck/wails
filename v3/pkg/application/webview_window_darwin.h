//go:build darwin && !ios

#ifndef WebviewWindowDelegate_h
#define WebviewWindowDelegate_h

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

@protocol WailsWebviewWindow <NSObject>
@property (assign) WKWebView* webView;
@property (assign) NSView* dragOutView;
@property BOOL disableEscapeExitsFullscreen;
@end

@interface WebviewWindow : NSWindow <WailsWebviewWindow>
- (BOOL) canBecomeKeyWindow;
- (BOOL) canBecomeMainWindow;
- (BOOL) acceptsFirstResponder;
- (BOOL) becomeFirstResponder;
- (BOOL) resignFirstResponder;
- (WebviewWindow*) initWithContentRect:(NSRect)contentRect styleMask:(NSUInteger)windowStyle backing:(NSBackingStoreType)bufferingType defer:(BOOL)deferCreation;

@property (assign) WKWebView* webView; // We already retain WKWebView since it's part of the Window.
@property BOOL disableEscapeExitsFullscreen;
@property (assign) NSView* dragOutView; // Overlay view enabling native file drag-out (NSDraggingSource).

@end

@interface WebviewWindowDelegate : NSObject <NSWindowDelegate, WKScriptMessageHandler, WKNavigationDelegate, WKURLSchemeHandler, NSDraggingDestination, WKUIDelegate>

@property unsigned int windowId;
@property (retain) NSEvent* leftMouseEvent;
@property unsigned int invisibleTitleBarHeight;
@property BOOL showToolbarWhenFullscreen;
@property NSWindowStyleMask previousStyleMask; // Used to restore the window style mask when using frameless

- (void)handleLeftMouseUp:(NSWindow *)window;
- (void)handleLeftMouseDown:(NSEvent*)event;
- (void)startDrag:(NSWindow*)window;

@end


// Glass effect style constants. These match the Go MacLiquidGlassStyle
// constants and are shared with the build-guarded implementations in
// mac_private_api_darwin.go and mac_public_api_darwin.go.
typedef NS_ENUM(NSInteger, MacLiquidGlassStyle) {
    LiquidGlassStyleAutomatic = 0,
    LiquidGlassStyleLight = 1,
    LiquidGlassStyleDark = 2,
    LiquidGlassStyleVibrant = 3
};

NSString* acceleratorStringFromKeyEvent(NSEvent* event);
BOOL dispatchKeyEquivalent(NSEvent* event, NSWindow* window);

void windowSetScreen(void* window, void* screen, int yOffset);

// File drag-out (NSDraggingSource) support.
// armFileDragOut arms the drag-out overlay with the given NUL-separated file
// paths and an optional drag image loaded from imagePath (may be NULL/empty).
void armFileDragOut(void* window, char** paths, int pathCount, char* imagePath);
// disarmFileDragOut disables the drag-out overlay (pointer events pass through).
void disarmFileDragOut(void* window);

// Liquid Glass support functions
bool isLiquidGlassSupported();
void windowSetLiquidGlass(void* window, int style, int material, double cornerRadius, 
                          int r, int g, int b, int a, 
                          const char* groupID, double groupSpacing);
void windowRemoveVisualEffects(void* window);
void configureWebViewForLiquidGlass(void* window);

#endif /* WebviewWindowDelegate_h */
