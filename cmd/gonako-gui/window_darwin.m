//go:build darwin

#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    int width;
    int height;
    int x;
    int y;
    int state;
    int resizable;
    char *title;
} gonakoWindowInfo;

static NSScreen *gonakoWindowScreen(NSWindow *window) {
    NSScreen *screen = window.screen;
    if (screen == nil) {
        screen = [NSScreen mainScreen];
    }
    return screen;
}

static void gonakoRestoreWindow(NSWindow *window) {
    if (window.isMiniaturized) {
        [window deminiaturize:nil];
    }
    if ((window.styleMask & NSWindowStyleMaskFullScreen) != 0) {
        [window toggleFullScreen:nil];
    }
    if (window.isZoomed) {
        [window zoom:nil];
    }
}

void gonakoApplyWindowSettings(void *windowPtr, int hasPosition, int center, int x, int y,
                               int hasState, int state, int hasResizable, int resizable) {
    NSWindow *window = (__bridge NSWindow *)windowPtr;
    if (window == nil) {
        return;
    }

    // 通常状態への復元を先に行い、その後で座標や属性を変更する。
    if (hasState && state == 0) {
        gonakoRestoreWindow(window);
    }

    if (hasResizable) {
        NSWindowStyleMask mask = window.styleMask;
        if (resizable) {
            mask |= NSWindowStyleMaskResizable;
        } else {
            mask &= ~NSWindowStyleMaskResizable;
        }
        window.styleMask = mask;
    }

    if (hasPosition) {
        if (center) {
            [window center];
        } else {
            NSScreen *screen = [NSScreen mainScreen];
            NSRect visible = screen.visibleFrame;
            NSPoint topLeft = NSMakePoint(x, NSMaxY(visible) - y);
            [window setFrameTopLeftPoint:topLeft];
        }
    }

    if (!hasState || state == 0) {
        return;
    }
    if (state == 1) {
        if (window.isMiniaturized) {
            [window deminiaturize:nil];
        }
        if ((window.styleMask & NSWindowStyleMaskFullScreen) != 0) {
            [window toggleFullScreen:nil];
        }
        if (!window.isZoomed) {
            [window zoom:nil];
        }
    } else if (state == 2) {
        [window miniaturize:nil];
    } else if (state == 3) {
        if (window.isMiniaturized) {
            [window deminiaturize:nil];
        }
        if ((window.styleMask & NSWindowStyleMaskFullScreen) == 0) {
            [window toggleFullScreen:nil];
        }
    }
}

// theme: 0=自動（OSに従う）、1=ライト、2=ダーク。
// WKWebViewのprefers-color-schemeもウィンドウの外観に追従する。
void gonakoApplyWindowTheme(void *windowPtr, int theme) {
    NSWindow *window = (__bridge NSWindow *)windowPtr;
    if (window == nil) {
        return;
    }
    if (theme == 1) {
        window.appearance = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
    } else if (theme == 2) {
        window.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    } else {
        window.appearance = nil;
    }
}

gonakoWindowInfo gonakoGetWindowInfo(void *windowPtr) {
    NSWindow *window = (__bridge NSWindow *)windowPtr;
    gonakoWindowInfo result = {0};
    if (window == nil) {
        return result;
    }

    NSRect frame = window.frame;
    NSScreen *screen = [NSScreen mainScreen];
    NSRect visible = screen.visibleFrame;
    result.width = (int)llround(frame.size.width);
    result.height = (int)llround(frame.size.height);
    result.x = (int)llround(NSMinX(frame));
    result.y = (int)llround(NSMaxY(visible) - NSMaxY(frame));
    result.resizable = (window.styleMask & NSWindowStyleMaskResizable) != 0;
    if (window.isMiniaturized) {
        result.state = 2;
    } else if ((window.styleMask & NSWindowStyleMaskFullScreen) != 0) {
        result.state = 3;
    } else if (window.isZoomed) {
        result.state = 1;
    } else {
        result.state = 0;
    }
    const char *title = window.title.UTF8String;
    result.title = strdup(title == NULL ? "" : title);
    return result;
}

void gonakoFreeWindowTitle(char *title) {
    free(title);
}
