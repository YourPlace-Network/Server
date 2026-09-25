#import <Cocoa/Cocoa.h>
#include <libproc.h>
#include <unistd.h>

static BOOL isDockPinned(void);

@interface LauncherDelegate : NSObject <NSApplicationDelegate>
@property(nonatomic) BOOL backgroundLaunch;
@property(nonatomic) BOOL quitting;
@property(nonatomic) pid_t serverPID;
@property(nonatomic, strong) NSTimer *serverTimer;
@property(nonatomic, strong) NSTask *task;
- (void)openYourPlace;
- (void)refreshServerStatus:(NSTimer *)timer;
@end

@implementation LauncherDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    self.serverTimer = [NSTimer scheduledTimerWithTimeInterval:1.0 target:self selector:@selector(refreshServerStatus:) userInfo:nil repeats:YES];
    if (self.backgroundLaunch) {
        [self refreshServerStatus:nil];
    } else {
        [self openYourPlace];
    }
}
- (BOOL)applicationShouldHandleReopen:(NSApplication *)application hasVisibleWindows:(BOOL)hasVisibleWindows {
    [self openYourPlace];
    return NO;
}
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)application {
    self.quitting = YES;
    [self.serverTimer invalidate];
    if (self.task.running) {
        [self.task terminate];
    }
    return NSTerminateNow;
}
- (void)openYourPlace {
    NSString *executable = [NSBundle.mainBundle pathForAuxiliaryExecutable:@"YourPlaceHelper"];
    NSTask *task;
    if (self.task != nil || self.quitting) {
        return;
    }
    task = [[NSTask alloc] init];
    self.task = task;
    task.arguments = @[@"-open-ui"];
    task.terminationHandler = ^(NSTask *finished) {
        dispatch_async(dispatch_get_main_queue(), ^{
            if (self.quitting) {
                return;
            }
            if (finished.terminationStatus != 0) {
                NSAlert *alert = [[NSAlert alloc] init];
                alert.messageText = @"YourPlace could not open";
                alert.informativeText = @"The background server could not be started or did not become ready. Please try again.";
                [NSApp activateIgnoringOtherApps:YES];
                [alert runModal];
            }
            self.task = nil;
            [self refreshServerStatus:nil];
        });
    };
    @try {
        task.launchPath = executable;
        [task launch];
    } @catch (NSException *exception) {
        NSAlert *alert = [[NSAlert alloc] init];
        alert.messageText = @"YourPlace could not open";
        alert.informativeText = @"The installed helper executable could not be launched.";
        [NSApp activateIgnoringOtherApps:YES];
        [alert runModal];
        [NSApp terminate:nil];
    }
}
- (void)refreshServerStatus:(NSTimer *)timer {
    NSString *serverPath = [NSBundle.mainBundle.bundlePath stringByAppendingPathComponent:@"Contents/Helpers/YourPlaceServer.app/Contents/MacOS/YourPlace"];
    char path[PROC_PIDPATHINFO_MAXSIZE];
    int size;
    int count;
    NSMutableData *buffer;
    pid_t *pids;
    if (self.task != nil || self.quitting) {
        return;
    }
    if (self.serverPID > 0 && proc_pidpath(self.serverPID, path, sizeof(path)) > 0 &&
        strcmp(path, serverPath.fileSystemRepresentation) == 0) {
        return;
    }
    self.serverPID = 0;
    size = proc_listpids(PROC_UID_ONLY, getuid(), NULL, 0);
    if (size <= 0) {
        return;
    }
    buffer = [NSMutableData dataWithLength:(NSUInteger)size + 32 * sizeof(pid_t)];
    pids = buffer.mutableBytes;
    count = proc_listpids(PROC_UID_ONLY, getuid(), pids, (int)buffer.length);
    if (count < 0) {
        return;
    }
    for (int index = 0; index < count / (int)sizeof(pid_t); index++) {
        if (pids[index] > 0 && proc_pidpath(pids[index], path, sizeof(path)) > 0 &&
            strcmp(path, serverPath.fileSystemRepresentation) == 0) {
            self.serverPID = pids[index];
            return;
        }
    }
    [NSApp terminate:nil];
}

@end

static BOOL isDockPinned(void) {
    id items;
    NSString *appPath = NSBundle.mainBundle.bundleURL.URLByStandardizingPath.path;
    CFPreferencesAppSynchronize(CFSTR("com.apple.dock"));
    items = CFBridgingRelease(CFPreferencesCopyAppValue(CFSTR("persistent-apps"), CFSTR("com.apple.dock")));
    if (![items isKindOfClass:NSArray.class]) {
        return NO;
    }
    for (id item in items) {
        id data;
        id identifier;
        id file;
        id path;
        id type;
        NSURL *url;
        if (![item isKindOfClass:NSDictionary.class]) {
            continue;
        }
        data = item[@"tile-data"];
        if (![data isKindOfClass:NSDictionary.class]) {
            continue;
        }
        identifier = data[@"bundle-identifier"];
        if ([identifier isKindOfClass:NSString.class] && [identifier isEqualToString:NSBundle.mainBundle.bundleIdentifier]) {
            return YES;
        }
        file = data[@"file-data"];
        if (![file isKindOfClass:NSDictionary.class]) {
            continue;
        }
        path = file[@"_CFURLString"];
        type = file[@"_CFURLStringType"];
        if (![path isKindOfClass:NSString.class] || ![type isKindOfClass:NSNumber.class]) {
            continue;
        }
        url = [type integerValue] == 0 ? [NSURL fileURLWithPath:path] : [NSURL URLWithString:path];
        if (url.isFileURL && [url.URLByStandardizingPath.path isEqualToString:appPath]) {
            return YES;
        }
    }
    return NO;
}

static int installDock(void) {
    id complete;
    id savedItems;
    NSMutableArray *items;
    NSURL *appURL = NSBundle.mainBundle.bundleURL.URLByStandardizingPath;
    BOOL found = NO;
    if (getuid() < 501 || geteuid() != getuid()) {
        return 1;
    }
    if (!CFPreferencesAppSynchronize(CFSTR("com.yourplace.launcher"))) {
        return 1;
    }
    complete = CFBridgingRelease(CFPreferencesCopyAppValue(CFSTR("dockSetupComplete"), CFSTR("com.yourplace.launcher")));
    if (complete != nil) {
        if (CFGetTypeID((__bridge CFTypeRef)complete) != CFBooleanGetTypeID()) {
            return 1;
        }
        if ([complete boolValue]) {
            return 0;
        }
    }
    if (!CFPreferencesAppSynchronize(CFSTR("com.apple.dock"))) {
        return 1;
    }
    savedItems = CFBridgingRelease(CFPreferencesCopyAppValue(CFSTR("persistent-apps"), CFSTR("com.apple.dock")));
    if (savedItems != nil && ![savedItems isKindOfClass:NSArray.class]) {
        return 1;
    }
    items = savedItems != nil ? [savedItems mutableCopy] : [NSMutableArray array];
    for (id item in items) {
        id data;
        id identifier;
        id file;
        id path;
        id type;
        NSURL *url;
        if (![item isKindOfClass:NSDictionary.class]) {
            return 1;
        }
        data = item[@"tile-data"];
        if (data == nil) {
            continue;
        }
        if (![data isKindOfClass:NSDictionary.class]) {
            return 1;
        }
        identifier = data[@"bundle-identifier"];
        file = data[@"file-data"];
        if ((identifier != nil && ![identifier isKindOfClass:NSString.class]) ||
            (file != nil && ![file isKindOfClass:NSDictionary.class])) {
            return 1;
        }
        path = file[@"_CFURLString"];
        type = file[@"_CFURLStringType"];
        if ((path != nil && ![path isKindOfClass:NSString.class]) ||
            (type != nil && ![type isKindOfClass:NSNumber.class])) {
            return 1;
        }
        url = nil;
        if (path != nil) {
            url = [type integerValue] == 0 ? [NSURL fileURLWithPath:path] : [NSURL URLWithString:path];
        }
        if ([identifier isEqualToString:@"com.yourplace.server"] ||
            (url.isFileURL && [url.URLByStandardizingPath.path isEqualToString:appURL.path])) {
            found = YES;
        }
    }
    if (!found) {
        [items addObject:@{
            @"tile-type": @"file-tile",
            @"tile-data": @{
                @"bundle-identifier": @"com.yourplace.server",
                @"file-label": @"YourPlace",
                @"file-type": @41,
                @"file-data": @{
                    @"_CFURLString": appURL.absoluteString,
                    @"_CFURLStringType": @15
                }
            }
        }];
        CFPreferencesSetAppValue(CFSTR("persistent-apps"), (__bridge CFArrayRef)items, CFSTR("com.apple.dock"));
        if (!CFPreferencesAppSynchronize(CFSTR("com.apple.dock"))) {
            return 1;
        }
    }
    CFPreferencesSetAppValue(CFSTR("dockSetupComplete"), kCFBooleanTrue, CFSTR("com.yourplace.launcher"));
    if (!CFPreferencesAppSynchronize(CFSTR("com.yourplace.launcher"))) {
        return 1;
    }
    if (!found) {
        NSTask *refresh = [[NSTask alloc] init];
        refresh.launchPath = @"/usr/bin/killall";
        refresh.arguments = @[@"-u", NSUserName(), @"Dock"];
        @try {
            [refresh launch];
            [refresh waitUntilExit];
        } @catch (NSException *exception) {
            return 1;
        }
    }
    return 0;
}

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        __attribute__((objc_precise_lifetime)) LauncherDelegate *delegate;
        NSMenu *menu;
        NSMenu *applicationMenu;
        NSMenuItem *applicationItem;
        BOOL backgroundLaunch = argc == 2 && strcmp(argv[1], "--background") == 0;
        if (argc == 2 && strcmp(argv[1], "--install-dock") == 0) {
            return installDock();
        }
        if (backgroundLaunch) {
            if (!isDockPinned()) {
                return 0;
            }
            for (NSRunningApplication *application in [NSRunningApplication runningApplicationsWithBundleIdentifier:NSBundle.mainBundle.bundleIdentifier]) {
                if (application.processIdentifier != getpid()) {
                    return 0;
                }
            }
        }
        [NSApplication sharedApplication];
        delegate = [[LauncherDelegate alloc] init];
        delegate.backgroundLaunch = backgroundLaunch;
        menu = [[NSMenu alloc] init];
        applicationMenu = [[NSMenu alloc] initWithTitle:@"YourPlace"];
        applicationItem = [[NSMenuItem alloc] init];
        [applicationMenu addItemWithTitle:@"Quit YourPlace" action:@selector(terminate:) keyEquivalent:@"q"];
        applicationItem.submenu = applicationMenu;
        [menu addItem:applicationItem];
        NSApp.mainMenu = menu;
        NSApp.delegate = delegate;
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        [NSApp run];
    }
    return 0;
}
