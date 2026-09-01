//go:build darwin

#import <Cocoa/Cocoa.h>

static NSMenuItem *gonakoMenuItem(NSString *title, SEL action, NSString *key) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title
                                                  action:action
                                           keyEquivalent:key];
    // nil sends the action through the responder chain to the focused editor.
    item.target = nil;
    return item;
}

void gonakoInstallEditMenu(void) {
    NSApplication *app = [NSApplication sharedApplication];
    NSMenu *mainMenu = app.mainMenu;
    if (mainMenu == nil) {
        mainMenu = [[NSMenu alloc] initWithTitle:@""];
        app.mainMenu = mainMenu;
    }

    for (NSMenuItem *item in mainMenu.itemArray) {
        if ([item.title isEqualToString:@"編集"] || [item.title isEqualToString:@"Edit"]) {
            return;
        }
    }

    NSMenu *editMenu = [[NSMenu alloc] initWithTitle:@"編集"];
    [editMenu addItem:gonakoMenuItem(@"取り消す", @selector(undo:), @"z")];
    [editMenu addItem:gonakoMenuItem(@"やり直す", @selector(redo:), @"Z")];
    [editMenu addItem:[NSMenuItem separatorItem]];
    [editMenu addItem:gonakoMenuItem(@"切り取り", @selector(cut:), @"x")];
    [editMenu addItem:gonakoMenuItem(@"コピー", @selector(copy:), @"c")];
    [editMenu addItem:gonakoMenuItem(@"貼り付け", @selector(paste:), @"v")];
    [editMenu addItem:gonakoMenuItem(@"すべて選択", @selector(selectAll:), @"a")];

    NSMenuItem *editMenuItem = [[NSMenuItem alloc] initWithTitle:@"編集"
                                                          action:nil
                                                   keyEquivalent:@""];
    editMenuItem.submenu = editMenu;
    [mainMenu addItem:editMenuItem];
}
