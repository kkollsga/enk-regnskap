#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

// ENKUIDelegate presenterer en NSOpenPanel når en HTML-fil-input klikkes.
@interface ENKUIDelegate : NSObject <WKUIDelegate>
@end

@implementation ENKUIDelegate
- (void)webView:(WKWebView *)webView
    runOpenPanelWithParameters:(WKOpenPanelParameters *)parameters
              initiatedByFrame:(WKFrameInfo *)frame
             completionHandler:(void (^)(NSArray<NSURL *> *))completionHandler {
    NSOpenPanel *panel = [NSOpenPanel openPanel];
    panel.canChooseFiles = YES;
    panel.canChooseDirectories = NO;
    panel.allowsMultipleSelection = parameters.allowsMultipleSelection;
    // WKOpenPanelParameters har ingen accept/MIME-info, så vi kan ikke speile
    // HTML-ens accept="". Vi lar panelet tillate alle filer; serveren avviser
    // uansett ugyldige typer. NSOpenPanel tilbyr automatisk «Importer fra
    // iPhone» (Continuity) på moderne macOS.
    [panel beginSheetModalForWindow:webView.window
                  completionHandler:^(NSModalResponse result) {
        completionHandler(result == NSModalResponseOK ? panel.URLs : nil);
    }];
}
@end

// UIDelegate er en svak (assign) referanse – delegaten må beholdes for appens
// levetid, ellers blir den frigjort og klikk gjør ingenting igjen.
static ENKUIDelegate *gUIDelegate = nil;

// findWebView leter rekursivt etter WKWebView-en i view-hierarkiet (robust selv
// om den ikke ligger rett på contentView).
static WKWebView *findWebView(NSView *v) {
    if ([v isKindOfClass:[WKWebView class]]) return (WKWebView *)v;
    for (NSView *sub in v.subviews) {
        WKWebView *found = findWebView(sub);
        if (found) return found;
    }
    return nil;
}

void enableFileOpenPanel(void *nsWindow) {
    NSWindow *window = (NSWindow *)nsWindow;
    WKWebView *web = findWebView(window.contentView);
    if (!web) return;
    gUIDelegate = [[ENKUIDelegate alloc] init];
    web.UIDelegate = gUIDelegate;
}
