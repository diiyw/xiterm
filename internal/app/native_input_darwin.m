#import <Cocoa/Cocoa.h>
#include <stdint.h>

static id xitermKeyMonitor;
static NSMutableArray<NSArray<NSNumber *> *> *xitermModifierSnapshots;

// Match keys forwarded by the pinned Onix macOS backend. Unknown multimedia
// keys must not add a snapshot without a corresponding OnKeydown callback.
static int xitermCanonicalKey(NSEvent *event) {
    // Keep this mapping aligned with window/macos_native.m in the pinned
    // Onix dependency so each Go key callback consumes its own flag snapshot.
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

void xiterm_install_key_modifiers(uintptr_t handle) {
    NSWindow *window = (NSWindow *)handle;
    xitermModifierSnapshots = [[NSMutableArray alloc] init];
    // Capture flags while NSEvent is being dispatched. Querying global flags
    // later would lose fast shortcuts already released in the same event batch.
    xitermKeyMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown handler:^NSEvent *(NSEvent *event) {
        int key = xitermCanonicalKey(event);
        if (event.window == window && key != 0) {
            NSEventModifierFlags f = event.modifierFlags;
            int flags = ((f & NSEventModifierFlagShift) ? 1 : 0)
                      | ((f & NSEventModifierFlagControl) ? 2 : 0)
                      | ((f & NSEventModifierFlagOption) ? 4 : 0)
                      | ((f & NSEventModifierFlagCommand) ? 8 : 0);
            [xitermModifierSnapshots addObject:@[@(key), @(flags)]];
        }
        return event;
    }];
}

int xiterm_next_key_modifiers(int key) {
    while (xitermModifierSnapshots.count > 0) {
        NSArray<NSNumber *> *snapshot = xitermModifierSnapshots[0];
        int code = snapshot[0].intValue;
        int flags = snapshot[1].intValue;
        [xitermModifierSnapshots removeObjectAtIndex:0];
        // Cocoa may consume a key equivalent without forwarding keyDown.
        if (code == key) return flags;
    }
    return -1;
}

void xiterm_remove_key_modifiers(void) {
    if (xitermKeyMonitor != nil) [NSEvent removeMonitor:xitermKeyMonitor];
    xitermKeyMonitor = nil;
    [xitermModifierSnapshots release];
    xitermModifierSnapshots = nil;
}
