//go:build darwin

#include "macos_native.h"

#import <AppKit/AppKit.h>
#import <CoreGraphics/CoreGraphics.h>
#import <QuartzCore/CAMetalLayer.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define ONIX_MAC_EVENT_CAPACITY 256
#define ONIX_MAC_TITLEBAR_BUTTON_SIZE 18.0
#define ONIX_MAC_TITLEBAR_BUTTON_LEFT_INSET 20.0
#define ONIX_MAC_TITLEBAR_BUTTON_GAP 4.0

@class OnixMacView;
@class OnixMacWindowDelegate;

struct OnixMacWindow {
    NSWindow *window;
    OnixMacView *view;
    OnixMacWindowDelegate *delegate;
    CAMetalLayer *layer;
    int width;
    int height;
    double scale;
    int should_close;
    int mouse_captured;
    int cursor;
    CGPoint saved_cursor;
    int event_head;
    int event_tail;
    int input_region_enabled;
    int input_rect_count;
    int decorations;
    double title_bar_height;
    int resizable;
    int waiting;
    OnixMacInputRect *input_rects;
    OnixMacEvent events[ONIX_MAC_EVENT_CAPACITY];
};

// Keep these indices in sync with input.Cursor.
static NSCursor *onix_macos_cursor(int cursor) {
    switch (cursor) {
    case ONIX_CURSOR_TEXT: return [NSCursor IBeamCursor];
    case ONIX_CURSOR_POINTER: return [NSCursor pointingHandCursor];
    case ONIX_CURSOR_CROSSHAIR: case ONIX_CURSOR_CELL: case ONIX_CURSOR_ZOOM_IN: case ONIX_CURSOR_ZOOM_OUT: return [NSCursor crosshairCursor];
    case ONIX_CURSOR_MOVE: case ONIX_CURSOR_GRAB: case ONIX_CURSOR_ALL_SCROLL: return [NSCursor openHandCursor];
    case ONIX_CURSOR_EW_RESIZE: return [NSCursor resizeLeftRightCursor];
    case ONIX_CURSOR_NS_RESIZE: return [NSCursor resizeUpDownCursor];
    case ONIX_CURSOR_GRABBING: return [NSCursor closedHandCursor];
    case ONIX_CURSOR_NOT_ALLOWED: return [NSCursor operationNotAllowedCursor];
    case ONIX_CURSOR_NONE: {
        static NSCursor *hidden;
        if (hidden == nil) {
            NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(1, 1)];
            hidden = [[NSCursor alloc] initWithImage:image hotSpot:NSZeroPoint];
            [image release];
        }
        return hidden;
    }
    case ONIX_CURSOR_VERTICAL_TEXT: return [NSCursor IBeamCursorForVerticalLayout];
    case ONIX_CURSOR_CONTEXT_MENU: return [NSCursor contextualMenuCursor];
    case ONIX_CURSOR_COPY: return [NSCursor dragCopyCursor];
    case ONIX_CURSOR_ALIAS: return [NSCursor dragLinkCursor];
    default: return [NSCursor arrowCursor];
    }
}

static _Thread_local char onix_macos_error[512];

static void onix_macos_push(OnixMacWindow *window, int kind, int arg0, double x, double y, const char *text) {
    if (window == NULL) return;
    int was_empty = window->event_head == window->event_tail;
    if (kind == ONIX_MAC_EVENT_MOUSE_DELTA && window->event_head != window->event_tail) {
        OnixMacEvent *previous = &window->events[(window->event_tail + ONIX_MAC_EVENT_CAPACITY - 1) % ONIX_MAC_EVENT_CAPACITY];
        if (previous->kind == kind) {
            previous->x += x;
            previous->y += y;
            return;
        }
    }
    int next = (window->event_tail + 1) % ONIX_MAC_EVENT_CAPACITY;
    if (next == window->event_head) {
        onix_macos_release_event(&window->events[window->event_head]);
        window->event_head = (window->event_head + 1) % ONIX_MAC_EVENT_CAPACITY;
    }
    OnixMacEvent *event = &window->events[window->event_tail];
    memset(event, 0, sizeof(*event));
    event->kind = kind;
    event->arg0 = arg0;
    event->x = x;
    event->y = y;
    if (text != NULL) {
        size_t length = strlen(text);
        if (length >= sizeof(event->text)) {
            length = sizeof(event->text) - 1;
            while (length > 0 && ((unsigned char)text[length] & 0xC0) == 0x80) length--;
        }
        memcpy(event->text, text, length);
        event->text[length] = '\0';
    }
    window->event_tail = next;
    // AppKit may deliver delegate callbacks inside nextEventMatchingMask
    // without returning an NSEvent. Wake the wait when our queue gains work.
    if (was_empty && window->waiting) onix_macos_wake();
}

static void onix_macos_push_text_input(OnixMacWindow *window, const char *text) {
    if (window == NULL || text == NULL) return;
    size_t remaining = strlen(text);
    size_t capacity = sizeof(window->events[0].text) - 1;
    while (remaining > 0) {
        size_t length = remaining < capacity ? remaining : capacity;
        while (length > 0 && length < remaining && ((unsigned char)text[length] & 0xC0) == 0x80) length--;
        if (length == 0) return;
        char chunk[sizeof(window->events[0].text)];
        memcpy(chunk, text, length);
        chunk[length] = '\0';
        onix_macos_push(window, ONIX_MAC_EVENT_TEXT, 0, 0, 0, chunk);
        text += length;
        remaining -= length;
    }
}

static void onix_macos_update_size(OnixMacWindow *window) {
    if (window == NULL || window->view == nil) return;
    NSSize points = [(NSView *)window->view bounds].size;
    NSScreen *screen = window->window.screen ?: NSScreen.mainScreen;
    window->scale = screen.backingScaleFactor;
    if (window->scale <= 0) window->scale = 1;
    window->width = (int)(points.width * window->scale + 0.5);
    window->height = (int)(points.height * window->scale + 0.5);
#if !defined(ONIX_MAC_OPENGL)
    window->layer.contentsScale = window->scale;
    window->layer.drawableSize = CGSizeMake(window->width, window->height);
#endif
}

static void onix_macos_update_input_passthrough(OnixMacWindow *state) {
    if (state == NULL || state->window == nil || state->view == nil) return;
    if (!state->input_region_enabled) {
        state->window.ignoresMouseEvents = NO;
        return;
    }

    NSPoint window_point = [state->window convertPointFromScreen:[NSEvent mouseLocation]];
    NSPoint point = [(NSView *)state->view convertPoint:window_point fromView:nil];
    BOOL accepts_mouse = NO;
    for (int i = 0; i < state->input_rect_count; i++) {
        OnixMacInputRect rect = state->input_rects[i];
        if (point.x >= rect.x && point.x < rect.x + rect.width &&
            point.y >= rect.y && point.y < rect.y + rect.height) {
            accepts_mouse = YES;
            break;
        }
    }
    state->window.ignoresMouseEvents = !accepts_mouse;
}

static int onix_macos_key(NSEvent *event) {
    // Use physical key codes so layout and modifier state do not change the
    // canonical key sent to the remote desktop.
    switch (event.keyCode) {
    case 0: return 65; case 1: return 83; case 2: return 68; case 3: return 70; case 4: return 72;
    case 5: return 71; case 6: return 90; case 7: return 88; case 8: return 67; case 9: return 86;
    case 11: return 66; case 12: return 81; case 13: return 87; case 14: return 69; case 15: return 82;
    case 16: return 89; case 17: return 84; case 18: return 49; case 19: return 50; case 20: return 51;
    case 21: return 52; case 22: return 54; case 23: return 53; case 24: return 61; case 25: return 57;
    case 26: return 55; case 27: return 45; case 28: return 56; case 29: return 48; case 30: return 93;
    case 31: return 79; case 32: return 85; case 33: return 91; case 34: return 73; case 35: return 80;
    case 37: return 76; case 38: return 74; case 39: return 39; case 40: return 75; case 41: return 59;
    case 42: return 92; case 43: return 44; case 44: return 47; case 45: return 78; case 46: return 77;
    case 47: return 46; case 49: return 32; case 50: return 96;
    case 53: return 256;
    case 36: return 257;
    case 48: return 258;
    case 51: return 259;
    case 117: return 261;
    case 124: return 262;
    case 123: return 263;
    case 125: return 264;
    case 126: return 265;
    case 116: return 266;
    case 121: return 267;
    case 115: return 268;
    case 119: return 269;
    case 122: return 290; case 120: return 291; case 99: return 292; case 118: return 293;
    case 96: return 294; case 97: return 295; case 98: return 296; case 100: return 297;
    case 101: return 298; case 109: return 299; case 103: return 300; case 111: return 301;
    case 105: return 302; case 107: return 303; case 113: return 304; case 106: return 305;
    case 64: return 306; case 79: return 307;
    default: break;
    }
    NSString *characters = event.charactersIgnoringModifiers.uppercaseString;
    if (characters.length == 1) {
        unichar character = [characters characterAtIndex:0];
        if (character >= 32 && character <= 126) return character;
    }
    return 0;
}

@interface OnixMacView : NSView <NSTextInputClient, NSDraggingDestination> {
    NSMutableAttributedString *_markedText;
    NSRange _markedRange;
    NSRange _selectedRange;
    NSRect _caretRect;
    uint64_t _textInputSession;
    BOOL _textInputActive;
}
@property(nonatomic, assign) OnixMacWindow *state;
- (void)clearMarkedText:(BOOL)notify;
- (void)configureTextInputSession:(uint64_t)session
                           active:(BOOL)active
                        selection:(NSRange)selection
                            caret:(NSRect)caret;
@end

@implementation OnixMacView
- (instancetype)initWithFrame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (self != nil) {
        _markedRange = NSMakeRange(NSNotFound, 0);
        _selectedRange = NSMakeRange(0, 0);
    }
    return self;
}
- (void)dealloc {
    [_markedText release];
    [super dealloc];
}
- (void)resetCursorRects {
    [super resetCursorRects];
    [self addCursorRect:self.bounds cursor:onix_macos_cursor(self.state->cursor)];
}
- (BOOL)acceptsFirstResponder { return YES; }
- (BOOL)isFlipped { return YES; }
// Client drags belong to the engine; window dragging is explicitly requested.
- (BOOL)mouseDownCanMoveWindow { return NO; }
- (void)updateTrackingAreas {
    [super updateTrackingAreas];
    for (NSTrackingArea *area in self.trackingAreas) [self removeTrackingArea:area];
    NSTrackingAreaOptions options = NSTrackingMouseMoved | NSTrackingActiveInKeyWindow | NSTrackingInVisibleRect;
    [self addTrackingArea:[[[NSTrackingArea alloc] initWithRect:NSZeroRect options:options owner:self userInfo:nil] autorelease]];
}
- (void)keyDown:(NSEvent *)event {
    int key = onix_macos_key(event);
    if (key != 0) onix_macos_push(self.state, ONIX_MAC_EVENT_KEY_DOWN, key, 0, 0, NULL);
    if (_textInputActive) [self interpretKeyEvents:@[event]];
}
- (void)keyUp:(NSEvent *)event {
    int key = onix_macos_key(event);
    if (key != 0) onix_macos_push(self.state, ONIX_MAC_EVENT_KEY_UP, key, 0, 0, NULL);
}
- (void)flagsChanged:(NSEvent *)event {
    int key = 0;
    NSEventModifierFlags modifier = event.modifierFlags;
    NSEventModifierFlags mask = 0;
    switch (event.keyCode) {
    case 56: key = 340; mask = NSEventModifierFlagShift; break;
    case 60: key = 344; mask = NSEventModifierFlagShift; break;
    case 59: key = 341; mask = NSEventModifierFlagControl; break;
    case 62: key = 345; mask = NSEventModifierFlagControl; break;
    case 58: key = 342; mask = NSEventModifierFlagOption; break;
    case 61: key = 346; mask = NSEventModifierFlagOption; break;
    case 55: key = 343; mask = NSEventModifierFlagCommand; break;
    case 54: key = 347; mask = NSEventModifierFlagCommand; break;
    case 57: key = 280; mask = NSEventModifierFlagCapsLock; break;
    default: break;
    }
    if (key != 0) {
        int kind = (modifier & mask) != 0 ? ONIX_MAC_EVENT_KEY_DOWN : ONIX_MAC_EVENT_KEY_UP;
        onix_macos_push(self.state, kind, key, 0, 0, NULL);
    }
}
- (void)mouseDown:(NSEvent *)event { [self pushMouse:event kind:ONIX_MAC_EVENT_MOUSE_DOWN button:0]; }
- (void)rightMouseDown:(NSEvent *)event { [self pushMouse:event kind:ONIX_MAC_EVENT_MOUSE_DOWN button:1]; }
- (void)otherMouseDown:(NSEvent *)event { [self pushMouse:event kind:ONIX_MAC_EVENT_MOUSE_DOWN button:2]; }
- (void)mouseUp:(NSEvent *)event { [self pushMouse:event kind:ONIX_MAC_EVENT_MOUSE_UP button:0]; }
- (void)rightMouseUp:(NSEvent *)event { [self pushMouse:event kind:ONIX_MAC_EVENT_MOUSE_UP button:1]; }
- (void)otherMouseUp:(NSEvent *)event { [self pushMouse:event kind:ONIX_MAC_EVENT_MOUSE_UP button:2]; }
- (void)mouseMoved:(NSEvent *)event {
    if (self.state->mouse_captured) {
        onix_macos_push(self.state, ONIX_MAC_EVENT_MOUSE_DELTA, 0, event.deltaX, event.deltaY, NULL);
    } else {
        [self pushMouse:event kind:ONIX_MAC_EVENT_MOUSE_MOVE button:0];
    }
}
- (void)mouseDragged:(NSEvent *)event { [self mouseMoved:event]; }
- (void)rightMouseDragged:(NSEvent *)event { [self mouseMoved:event]; }
- (void)otherMouseDragged:(NSEvent *)event { [self mouseMoved:event]; }
- (void)scrollWheel:(NSEvent *)event { onix_macos_push(self.state, ONIX_MAC_EVENT_MOUSE_WHEEL, 0, event.scrollingDeltaX, event.scrollingDeltaY, NULL); }

- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {
    NSPasteboard *pasteboard = sender.draggingPasteboard;
    NSDictionary *options = @{NSPasteboardURLReadingFileURLsOnlyKey: @YES};
    return [pasteboard canReadObjectForClasses:@[[NSURL class]] options:options] ? NSDragOperationCopy : NSDragOperationNone;
}
- (NSDragOperation)draggingUpdated:(id<NSDraggingInfo>)sender {
    return [self draggingEntered:sender];
}
- (void)draggingExited:(id<NSDraggingInfo>)sender { (void)sender; }
- (BOOL)prepareForDragOperation:(id<NSDraggingInfo>)sender { return [self draggingEntered:sender] != NSDragOperationNone; }
- (BOOL)performDragOperation:(id<NSDraggingInfo>)sender {
    NSPasteboard *pasteboard = sender.draggingPasteboard;
    NSDictionary *options = @{NSPasteboardURLReadingFileURLsOnlyKey: @YES};
    NSArray<NSURL *> *urls = [pasteboard readObjectsForClasses:@[[NSURL class]] options:options];
    NSPoint point = sender.draggingLocation;
    for (NSURL *url in urls) {
        const char *path = url.path.fileSystemRepresentation;
        if (path != NULL) onix_macos_push(self.state, ONIX_MAC_EVENT_FILES_DROPPED, 0, point.x, point.y, path);
    }
    return urls.count > 0;
}
- (void)pushMouse:(NSEvent *)event kind:(int)kind button:(int)button {
    NSPoint point = [self convertPoint:event.locationInWindow fromView:nil];
    onix_macos_push(self.state, kind, button, point.x, point.y, NULL);
    if (kind == ONIX_MAC_EVENT_MOUSE_DOWN && button == 0) {
        // Go dispatches after AppKit has drained its queue and autorelease pool.
        // Keep the original press with its queued event, not NSApp.currentEvent.
        int index = (self.state->event_tail + ONIX_MAC_EVENT_CAPACITY - 1) % ONIX_MAC_EVENT_CAPACITY;
        self.state->events[index].mouse_down = [event retain];
    }
}

- (BOOL)hasMarkedText {
    return _markedRange.location != NSNotFound && _markedRange.length > 0;
}

- (NSRange)markedRange {
    return _markedRange;
}

- (NSRange)selectedRange {
    return _selectedRange;
}

- (void)setMarkedText:(id)value selectedRange:(NSRange)selectedRange replacementRange:(NSRange)replacementRange {
    if (!_textInputActive) return;
    NSString *text = [value isKindOfClass:[NSAttributedString class]] ? [(NSAttributedString *)value string] : (NSString *)value;
    if (text.length == 0) {
        [self clearMarkedText:YES];
        return;
    }

    NSUInteger location = [self hasMarkedText] ? _markedRange.location : _selectedRange.location;
    if (replacementRange.location != NSNotFound) location = replacementRange.location;
    [_markedText release];
    _markedText = [[NSMutableAttributedString alloc] initWithString:text];
    _markedRange = NSMakeRange(location, text.length);
    _selectedRange = NSMakeRange(location + selectedRange.location, selectedRange.length);
    onix_macos_push(self.state, ONIX_MAC_EVENT_TEXT_COMPOSITION, 1, 0, 0, text.UTF8String);
}

- (void)unmarkText {
    [self clearMarkedText:YES];
}

- (void)insertText:(id)value replacementRange:(NSRange)replacementRange {
    if (!_textInputActive) return;
    NSString *text = [value isKindOfClass:[NSAttributedString class]] ? [(NSAttributedString *)value string] : (NSString *)value;
    NSUInteger location = [self hasMarkedText] ? _markedRange.location : _selectedRange.location;
    if (replacementRange.location != NSNotFound) location = replacementRange.location;
    [self clearMarkedText:YES];
    _selectedRange = NSMakeRange(location + text.length, 0);
    if (text.length == 0) return;

    NSCharacterSet *lineBreaks = [NSCharacterSet characterSetWithCharactersInString:@"\r\n"];
    if ([text rangeOfCharacterFromSet:[lineBreaks invertedSet]].location == NSNotFound) {
        onix_macos_push(self.state, ONIX_MAC_EVENT_TEXT_COMMAND, ONIX_MAC_TEXT_COMMAND_ENTER, 0, 0, NULL);
        return;
    }
    onix_macos_push_text_input(self.state, text.UTF8String);
}

- (void)doCommandBySelector:(SEL)selector {
    if (!_textInputActive) return;
    if (selector == @selector(deleteBackward:) || selector == @selector(deleteBackwardByDecomposingPreviousCharacter:)) {
        onix_macos_push(self.state, ONIX_MAC_EVENT_TEXT_COMMAND, ONIX_MAC_TEXT_COMMAND_BACKSPACE, 0, 0, NULL);
        return;
    }
    if (selector == @selector(insertNewline:) || selector == @selector(insertLineBreak:) || selector == @selector(insertNewlineIgnoringFieldEditor:)) {
        onix_macos_push(self.state, ONIX_MAC_EVENT_TEXT_COMMAND, ONIX_MAC_TEXT_COMMAND_ENTER, 0, 0, NULL);
    }
}

- (NSArray *)validAttributesForMarkedText {
    return @[];
}

- (NSAttributedString *)attributedSubstringForProposedRange:(NSRange)range actualRange:(NSRangePointer)actualRange {
    if (actualRange != NULL) *actualRange = NSMakeRange(NSNotFound, 0);
    return nil;
}

- (NSUInteger)characterIndexForPoint:(NSPoint)point {
    return _selectedRange.location;
}

- (NSRect)firstRectForCharacterRange:(NSRange)range actualRange:(NSRangePointer)actualRange {
    if (actualRange != NULL) *actualRange = [self hasMarkedText] ? _markedRange : _selectedRange;
    NSRect rect = _caretRect;
    if (rect.size.width <= 0) rect.size.width = 1;
    if (rect.size.height <= 0) rect.size.height = 1;
    NSRect windowRect = [self convertRect:rect toView:nil];
    return self.window != nil ? [self.window convertRectToScreen:windowRect] : NSZeroRect;
}

- (void)configureTextInputSession:(uint64_t)session
                           active:(BOOL)active
                        selection:(NSRange)selection
                            caret:(NSRect)caret {
    BOOL reset = _textInputSession != session || _textInputActive != active;
    NSTextInputContext *inputContext = nil;
    _caretRect = caret;
    if (!reset) {
        if (![self hasMarkedText]) _selectedRange = selection;
        [[self inputContext] invalidateCharacterCoordinates];
        return;
    }

    inputContext = [self inputContext];
    _textInputActive = NO;
    [inputContext discardMarkedText];
    [self clearMarkedText:YES];
    _textInputSession = session;
    _textInputActive = active;
    _selectedRange = selection;
    if (active) {
        [self.window makeFirstResponder:self];
        [inputContext activate];
    } else {
        [inputContext deactivate];
    }
    [inputContext invalidateCharacterCoordinates];
}

- (void)clearMarkedText:(BOOL)notify {
    BOOL hadMarkedText = [self hasMarkedText];
    [_markedText release];
    _markedText = nil;
    _markedRange = NSMakeRange(NSNotFound, 0);
    if (notify && hadMarkedText) onix_macos_push(self.state, ONIX_MAC_EVENT_TEXT_COMPOSITION, 0, 0, 0, NULL);
}
@end

@interface OnixMacWindowDelegate : NSObject <NSWindowDelegate>
@property(nonatomic, assign) OnixMacWindow *state;
@end

@implementation OnixMacWindowDelegate
- (void)windowDidBecomeKey:(NSNotification *)notification {
    onix_macos_push(self.state, ONIX_MAC_EVENT_REDRAW, 0, 0, 0, NULL);
}
- (void)windowDidExpose:(NSNotification *)notification {
    onix_macos_push(self.state, ONIX_MAC_EVENT_REDRAW, 0, 0, 0, NULL);
}
- (void)windowDidDeminiaturize:(NSNotification *)notification {
    onix_macos_push(self.state, ONIX_MAC_EVENT_REDRAW, 0, 0, 0, NULL);
}
- (void)windowDidChangeOcclusionState:(NSNotification *)notification {
    if (self.state->window.occlusionState & NSWindowOcclusionStateVisible) {
        onix_macos_push(self.state, ONIX_MAC_EVENT_REDRAW, 0, 0, 0, NULL);
    }
}
- (void)windowDidResignKey:(NSNotification *)notification {
    onix_macos_set_mouse_captured(self.state, 0);
}
- (BOOL)windowShouldClose:(NSWindow *)sender {
    self.state->should_close = 1;
    onix_macos_push(self.state, ONIX_MAC_EVENT_CLOSE, 0, 0, 0, NULL);
	return NO;
}
- (void)windowDidResize:(NSNotification *)notification {
    onix_macos_update_size(self.state);
    onix_macos_push(self.state, ONIX_MAC_EVENT_RESIZE, 0, self.state->width, self.state->height, NULL);
}
- (void)windowDidChangeBackingProperties:(NSNotification *)notification {
    [self windowDidResize:notification];
}
@end

@interface OnixMacApplicationDelegate : NSObject <NSApplicationDelegate>
@end

@implementation OnixMacApplicationDelegate
- (void)applicationDidBecomeActive:(NSNotification *)notification {
    NSApplication *app = notification.object;
    for (NSWindow *window in app.windows) {
        if (![window.contentView isKindOfClass:[OnixMacView class]]) continue;
        onix_macos_push(((OnixMacView *)window.contentView).state, ONIX_MAC_EVENT_REDRAW, 0, 0, 0, NULL);
    }
}
- (BOOL)applicationShouldHandleReopen:(NSApplication *)sender hasVisibleWindows:(BOOL)visible {
    NSWindow *target = nil;
    for (NSWindow *window in sender.windows) {
        if (![window.contentView isKindOfClass:[OnixMacView class]]) continue;
        OnixMacWindow *state = ((OnixMacView *)window.contentView).state;
        if (state == NULL || state->should_close) continue;
        if (target == nil || window.isMainWindow) target = window;
        if (window.isKeyWindow) { target = window; break; }
    }
    if (target == nil) return YES;
    [sender unhide:nil];
    // AppKit counts miniaturized windows as visible for this callback.
    if (target.isMiniaturized) [target deminiaturize:nil];
    [target makeKeyAndOrderFront:nil];
    onix_macos_push(((OnixMacView *)target.contentView).state, ONIX_MAC_EVENT_REDRAW, 0, 0, 0, NULL);
    return NO;
}
@end

// NSApplication does not retain its delegate; keep it for the app's lifetime.
static OnixMacApplicationDelegate *onix_macos_app_delegate;

const char *onix_macos_last_error(void) { return onix_macos_error; }

static NSWindowStyleMask onix_macos_style(int decorations, double title_bar_height, int resizable) {
    NSWindowStyleMask style = decorations
        ? NSWindowStyleMaskBorderless
        : NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable;
    if (decorations && title_bar_height > 0) {
        style = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskFullSizeContentView;
    }
    if (resizable) style |= NSWindowStyleMaskResizable;
    return style;
}

static void onix_macos_layout_window_buttons(OnixMacWindow *state) {
    if (state == NULL || state->window == nil || state->view == nil || !state->decorations || state->title_bar_height <= 0) return;
    NSButton *close = [state->window standardWindowButton:NSWindowCloseButton];
    if (close == nil || close.superview == nil || close.superview.superview == nil) return;
    NSView *titlebar = close.superview;
    NSView *container = titlebar.superview;
    if (container.superview == nil) return;
    NSRect titlebarRect = state->view.bounds;
    titlebarRect.size.height = state->title_bar_height;
    NSRect frame = [state->view convertRect:titlebarRect toView:container.superview];
    if (!NSEqualRects(container.frame, frame)) container.frame = frame;
    frame = container.bounds;
    if (!NSEqualRects(titlebar.frame, frame)) titlebar.frame = frame;

    // Use the flipped content view for both axes; AppKit's nested titlebar
    // views may use a different origin and must also cover the full bar height.
    NSButton *buttons[] = {
        close,
        [state->window standardWindowButton:NSWindowMiniaturizeButton],
        [state->window standardWindowButton:NSWindowZoomButton],
    };
    for (size_t index = 0; index < sizeof(buttons) / sizeof(buttons[0]); index++) {
        NSButton *button = buttons[index];
        if (button == nil || button.superview == nil) continue;
        NSPoint center = NSMakePoint(NSMinX(titlebarRect) + ONIX_MAC_TITLEBAR_BUTTON_LEFT_INSET +
                                    index * (ONIX_MAC_TITLEBAR_BUTTON_SIZE + ONIX_MAC_TITLEBAR_BUTTON_GAP) +
                                    ONIX_MAC_TITLEBAR_BUTTON_SIZE * 0.5, NSMidY(titlebarRect));
        center = [button.superview convertPoint:center fromView:state->view];
        NSRect buttonFrame = NSMakeRect(center.x - ONIX_MAC_TITLEBAR_BUTTON_SIZE * 0.5,
                                       center.y - ONIX_MAC_TITLEBAR_BUTTON_SIZE * 0.5,
                                       ONIX_MAC_TITLEBAR_BUTTON_SIZE, ONIX_MAC_TITLEBAR_BUTTON_SIZE);
        if (!NSEqualRects(button.frame, buttonFrame)) button.frame = buttonFrame;
        button.imageScaling = NSImageScaleProportionallyUpOrDown;
    }
}

static void onix_macos_apply_style(OnixMacWindow *state) {
    if (state == NULL || state->window == nil) return;
    state->window.styleMask = onix_macos_style(state->decorations, state->title_bar_height, state->resizable);
    state->window.titlebarAppearsTransparent = state->decorations ? YES : NO;
    state->window.titleVisibility = state->decorations ? NSWindowTitleHidden : NSWindowTitleVisible;
    state->window.movableByWindowBackground = NO;
    onix_macos_layout_window_buttons(state);
}

OnixMacWindow *onix_macos_create(int width, int height, const char *title, int transparent, int decorations, double title_bar_height, int shadow, int resizable) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        if (onix_macos_app_delegate == nil) {
            onix_macos_app_delegate = [[OnixMacApplicationDelegate alloc] init];
            NSApp.delegate = onix_macos_app_delegate;
        }
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        [NSApp finishLaunching];
        OnixMacWindow *state = calloc(1, sizeof(OnixMacWindow));
        if (state == NULL) {
            snprintf(onix_macos_error, sizeof(onix_macos_error), "failed to allocate macOS window state");
            return NULL;
        }
        NSRect rect = NSMakeRect(0, 0, width, height);
        state->decorations = decorations != 0;
        state->title_bar_height = title_bar_height;
        state->resizable = resizable != 0;
        NSWindowStyleMask style = onix_macos_style(state->decorations, state->title_bar_height, state->resizable);
        state->window = [[NSWindow alloc] initWithContentRect:rect styleMask:style backing:NSBackingStoreBuffered defer:NO];
        state->window.releasedWhenClosed = NO;
        state->window.opaque = transparent ? NO : YES;
        state->window.backgroundColor = transparent ? NSColor.clearColor : NSColor.windowBackgroundColor;
        state->window.hasShadow = shadow ? YES : NO;
        onix_macos_apply_style(state);
        state->view = [[OnixMacView alloc] initWithFrame:rect];
        state->view.state = state;
#if !defined(ONIX_MAC_OPENGL)
        state->view.wantsLayer = YES;
        state->layer = [CAMetalLayer layer];
        state->layer.opaque = transparent ? NO : YES;
        state->view.layer = state->layer;
#else
        state->view.wantsBestResolutionOpenGLSurface = YES;
#endif
        state->delegate = [[OnixMacWindowDelegate alloc] init];
        state->delegate.state = state;
        state->window.delegate = state->delegate;
        state->window.contentView = state->view;
        [state->view registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
        state->window.title = title != NULL ? [NSString stringWithUTF8String:title] : @"Onix";
        [state->window center];
        [state->window makeKeyAndOrderFront:nil];
        [state->window makeFirstResponder:state->view];
        [NSApp activateIgnoringOtherApps:YES];
        onix_macos_layout_window_buttons(state);
        onix_macos_update_size(state);
        return state;
    }
}

int onix_macos_file_dialog(
    OnixMacWindow *state,
    int mode,
    const char *title,
    const char *default_path,
    const char *extensions,
    int extension_bytes,
    char ***paths
) {
    if (paths == NULL || mode < 0 || mode > 2) return -1;
    *paths = NULL;
    @autoreleasepool {
        NSOpenPanel *panel = [NSOpenPanel openPanel];
        panel.title = title != NULL ? [NSString stringWithUTF8String:title] : @"Select a file";
        panel.canChooseFiles = mode != 1;
        panel.canChooseDirectories = mode == 1;
        panel.allowsMultipleSelection = mode == 2;
        panel.allowsOtherFileTypes = YES;
        if (default_path != NULL && default_path[0] != '\0') {
            NSString *path = [NSString stringWithUTF8String:default_path];
            BOOL isDirectory = NO;
            if ([[NSFileManager defaultManager] fileExistsAtPath:path isDirectory:&isDirectory]) {
                if (!isDirectory) path = [path stringByDeletingLastPathComponent];
                panel.directoryURL = [NSURL fileURLWithPath:path isDirectory:YES];
            }
        }
        if (extensions != NULL && extension_bytes > 0 && mode != 1) {
            NSMutableArray<UTType *> *allowed = [NSMutableArray array];
            int offset = 0;
            while (offset < extension_bytes) {
                const char *extension = extensions + offset;
                size_t length = strnlen(extension, (size_t)(extension_bytes - offset));
                if (length == 0) {
                    offset++;
                    continue;
                }
                NSString *value = [NSString stringWithUTF8String:extension];
                UTType *type = value != nil ? [UTType typeWithFilenameExtension:value] : nil;
                if (type != nil) [allowed addObject:type];
                offset += (int)length + 1;
            }
            if (allowed.count > 0) {
                panel.allowedContentTypes = allowed;
                panel.allowsOtherFileTypes = NO;
            }
        }
        [NSApp activateIgnoringOtherApps:YES];
        BOOL attached = state != NULL && state->window != nil;
        // Position before attaching: an unpositioned child panel starts at the
        // screen's lower-left origin rather than receiving modal placement.
        NSScreen *screen = (attached ? state->window.screen : nil) ?: NSScreen.mainScreen;
        NSRect visibleFrame = screen.visibleFrame;
        NSRect panelFrame = panel.frame;
        [panel setFrameOrigin:NSMakePoint(NSMidX(visibleFrame) - NSWidth(panelFrame) * 0.5,
                                         NSMidY(visibleFrame) - NSHeight(panelFrame) * 0.5)];
        if (attached) [state->window addChildWindow:panel ordered:NSWindowAbove];
        NSModalResponse response = [panel runModal];
        if (attached) [state->window removeChildWindow:panel];
        if (response != NSModalResponseOK) return 0;
        NSArray<NSURL *> *urls = panel.URLs;
        if (urls.count == 0) return 0;
        char **selected = calloc(urls.count, sizeof(char *));
        if (selected == NULL) {
            snprintf(onix_macos_error, sizeof(onix_macos_error), "failed to allocate file dialog result");
            return -1;
        }
        for (NSUInteger index = 0; index < urls.count; index++) {
            const char *value = urls[index].path.UTF8String;
            selected[index] = value != NULL ? strdup(value) : NULL;
            if (value != NULL && selected[index] == NULL) {
                onix_macos_file_dialog_free(selected, (int)urls.count);
                snprintf(onix_macos_error, sizeof(onix_macos_error), "failed to allocate file dialog path");
                return -1;
            }
        }
        *paths = selected;
        return (int)urls.count;
    }
}

void onix_macos_file_dialog_free(char **paths, int count) {
    if (paths == NULL) return;
    for (int index = 0; index < count; index++) free(paths[index]);
    free(paths);
}

void onix_macos_set_cursor(OnixMacWindow *state, int cursor) {
    if (state == NULL || cursor < 0 || cursor >= ONIX_CURSOR_COUNT) return;
    state->cursor = cursor;
    [state->window invalidateCursorRectsForView:state->view];
    if (!state->mouse_captured) [onix_macos_cursor(cursor) set];
}

int onix_macos_set_mouse_captured(OnixMacWindow *state, int captured) {
    if (state == NULL) return 0;
    captured = captured != 0;
    if (state->mouse_captured == captured) return 1;
    if (captured) {
        if (!state->window.isKeyWindow) return 0;
        CGEventRef event = CGEventCreate(NULL);
        if (event == NULL) return 0;
        state->saved_cursor = CGEventGetLocation(event);
        CFRelease(event);
        if (CGAssociateMouseAndMouseCursorPosition(false) != kCGErrorSuccess) return 0;
        NSPoint center = NSMakePoint(NSMidX(state->view.bounds), NSMidY(state->view.bounds));
        center = [state->view convertPoint:center toView:nil];
        center = [state->window convertPointToScreen:center];
        CGWarpMouseCursorPosition(CGPointMake(center.x, CGDisplayBounds(CGMainDisplayID()).size.height - center.y));
        [NSCursor hide];
    } else {
        CGAssociateMouseAndMouseCursorPosition(true);
        CGWarpMouseCursorPosition(state->saved_cursor);
        [NSCursor unhide];
    }
    state->mouse_captured = captured;
    onix_macos_push(state, ONIX_MAC_EVENT_MOUSE_CAPTURE, captured, 0, 0, NULL);
    return 1;
}

void onix_macos_destroy(OnixMacWindow *state) {
    if (state == NULL) return;
    onix_macos_set_mouse_captured(state, 0);
    state->window.delegate = nil;
    [state->window orderOut:nil];
    [state->window close];
    state->window.contentView = nil;
    [state->delegate release];
    [state->view release];
    [state->window release];
    while (state->event_head != state->event_tail) {
        onix_macos_release_event(&state->events[state->event_head]);
        state->event_head = (state->event_head + 1) % ONIX_MAC_EVENT_CAPACITY;
    }
    free(state->input_rects);
    free(state);
}

static void onix_macos_send_event(NSEvent *event) {
    NSWindow *window = event.window ?: NSApp.keyWindow;
    // NSApplication drops key-up events while Command is held. Dispatch them
    // to our window so shortcut keys do not remain held in the input state.
    if (event.type == NSEventTypeKeyUp && (event.modifierFlags & NSEventModifierFlagCommand) &&
        [window.contentView isKindOfClass:[OnixMacView class]]) {
        [window sendEvent:event];
    } else {
        [NSApp sendEvent:event];
    }
}

void onix_macos_poll(OnixMacWindow *state) {
    if (state == NULL) return;
    @autoreleasepool {
        onix_macos_update_input_passthrough(state);
        NSEvent *event;
        while ((event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:[NSDate distantPast] inMode:NSDefaultRunLoopMode dequeue:YES]) != nil) {
            onix_macos_send_event(event);
        }
        [NSApp updateWindows];
        onix_macos_layout_window_buttons(state);
    }
}

void onix_macos_wait(OnixMacWindow *state, int timeout_ms) {
    if (state == NULL || state->should_close || state->event_head != state->event_tail) return;
    @autoreleasepool {
        NSDate *deadline = timeout_ms < 0 ? [NSDate distantFuture] : [NSDate dateWithTimeIntervalSinceNow:(double)timeout_ms / 1000.0];
        state->waiting = 1;
        // Peeking with dequeue:NO can leave AppKit's idle wait stuck. Consume
        // and dispatch the event here, before the normal poll drains the rest.
        NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:deadline inMode:NSDefaultRunLoopMode dequeue:YES];
        state->waiting = 0;
        if (event != nil) onix_macos_send_event(event);
    }
}

void onix_macos_wake(void) {
    @autoreleasepool {
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
        [NSApp postEvent:event atStart:NO];
    }
}

int onix_macos_pop_event(OnixMacWindow *state, OnixMacEvent *event) {
    if (state == NULL || event == NULL || state->event_head == state->event_tail) return 0;
    *event = state->events[state->event_head];
    state->events[state->event_head].mouse_down = NULL;
    state->event_head = (state->event_head + 1) % ONIX_MAC_EVENT_CAPACITY;
    return 1;
}

void onix_macos_release_event(OnixMacEvent *event) {
    if (event == NULL) return;
    [(NSEvent *)event->mouse_down release];
    event->mouse_down = NULL;
}

int onix_macos_should_close(OnixMacWindow *state) { return state == NULL || state->should_close; }
void onix_macos_cancel_close(OnixMacWindow *state) { if (state != NULL) state->should_close = 0; }
int onix_macos_width(OnixMacWindow *state) { return state != NULL ? state->width : 0; }
int onix_macos_height(OnixMacWindow *state) { return state != NULL ? state->height : 0; }
double onix_macos_scale(OnixMacWindow *state) { return state != NULL ? state->scale : 1; }
void *onix_macos_layer(OnixMacWindow *state) { return state != NULL ? state->layer : NULL; }
void *onix_macos_native_window(OnixMacWindow *state) { return state != NULL ? state->window : NULL; }

void onix_macos_set_title(OnixMacWindow *state, const char *title) {
    if (state != NULL && title != NULL) state->window.title = [NSString stringWithUTF8String:title];
}

void onix_macos_set_decorations(OnixMacWindow *state, int mode) {
	if (state == NULL) return;
	state->decorations = mode != 0;
	onix_macos_apply_style(state);
}

void onix_macos_set_title_bar_height(OnixMacWindow *state, double height) {
    if (state == NULL) return;
    state->title_bar_height = height > 0 ? height : 0;
    onix_macos_apply_style(state);
}

void onix_macos_set_shadow_visible(OnixMacWindow *state, int visible) {
    if (state != NULL) state->window.hasShadow = visible ? YES : NO;
}

void onix_macos_set_resizable(OnixMacWindow *state, int enabled) {
    if (state == NULL) return;
    state->resizable = enabled != 0;
    onix_macos_apply_style(state);
}

void onix_macos_set_taskbar_visible(OnixMacWindow *state, int visible) {
    if (state == NULL) return;
    [NSApp setActivationPolicy:visible ? NSApplicationActivationPolicyRegular : NSApplicationActivationPolicyAccessory];
    if (visible) [NSApp activateIgnoringOtherApps:YES];
}

void onix_macos_set_always_on_top(OnixMacWindow *state, int enabled) {
    if (state == NULL) return;
    NSWindowCollectionBehavior floating_behavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary;
    state->window.level = enabled ? NSFloatingWindowLevel : NSNormalWindowLevel;
    if (enabled) {
        state->window.collectionBehavior |= floating_behavior;
    } else {
        state->window.collectionBehavior &= ~floating_behavior;
    }
}

void onix_macos_set_input_region(OnixMacWindow *state, int enabled, const OnixMacInputRect *rects, int count) {
    if (state == NULL || count < 0 || (count > 0 && rects == NULL)) return;
    OnixMacInputRect *replacement = NULL;
    if (count > 0) {
        replacement = malloc((size_t)count * sizeof(OnixMacInputRect));
        if (replacement == NULL) return;
        memcpy(replacement, rects, (size_t)count * sizeof(OnixMacInputRect));
    }
    free(state->input_rects);
    state->input_rects = replacement;
    state->input_rect_count = count;
    state->input_region_enabled = enabled != 0;
    onix_macos_update_input_passthrough(state);
}

void onix_macos_minimize(OnixMacWindow *state) {
    if (state != NULL) [state->window miniaturize:nil];
}

void onix_macos_toggle_maximize(OnixMacWindow *state) {
    if (state != NULL) [state->window zoom:nil];
}

void onix_macos_toggle_fullscreen(OnixMacWindow *state) {
    if (state != NULL) [state->window toggleFullScreen:nil];
}

void onix_macos_start_drag(OnixMacWindow *state, const OnixMacEvent *event) {
    if (state == NULL || event == NULL || state->mouse_captured) return;
    NSEvent *press = (NSEvent *)event->mouse_down;
    // A quick click may already have been released while Go was processing.
    if (press.type != NSEventTypeLeftMouseDown || press.window != state->window ||
        ([NSEvent pressedMouseButtons] & 1) == 0) return;
    [state->window performWindowDragWithEvent:press];
}

void onix_macos_set_text_input_config(
    OnixMacWindow *state,
    uint64_t session,
    int active,
    int selection_start,
    int selection_end,
    double caret_x,
    double caret_y,
    double caret_width,
    double caret_height
) {
    if (state == NULL || state->view == nil) return;
    @autoreleasepool {
        int start = selection_start < selection_end ? selection_start : selection_end;
        int end = selection_start < selection_end ? selection_end : selection_start;
        NSRange selection = NSMakeRange((NSUInteger)(start < 0 ? 0 : start), (NSUInteger)(end < 0 ? 0 : end) - (NSUInteger)(start < 0 ? 0 : start));
        NSRect caret = NSMakeRect(caret_x, caret_y, caret_width, caret_height);
        [state->view configureTextInputSession:session active:active != 0 selection:selection caret:caret];
    }
}

int onix_macos_set_clipboard_text(const char *text) {
    if (text == NULL) return 0;
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    return [pasteboard setString:[NSString stringWithUTF8String:text] forType:NSPasteboardTypeString] ? 1 : 0;
}

int onix_macos_get_clipboard_text(char *buffer, int capacity) {
    NSString *value = [[NSPasteboard generalPasteboard] stringForType:NSPasteboardTypeString];
    if (value == nil) return -1;
    const char *text = value.UTF8String;
    int length = (int)strlen(text);
    if (buffer != NULL && capacity > 0) {
        int copied = length < capacity - 1 ? length : capacity - 1;
        memcpy(buffer, text, (size_t)copied);
        buffer[copied] = '\0';
    }
    return length;
}

void *onix_macos_view(OnixMacWindow *state) { return state ? state->view : NULL; }
