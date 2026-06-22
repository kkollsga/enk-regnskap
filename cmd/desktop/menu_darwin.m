#import <Cocoa/Cocoa.h>
#include "menu_darwin.h"
#include "_cgo_export.h"

// Mål for menyhandlinger som ringer tilbake til Go.
@interface ENKMenuTarget : NSObject
@end

@implementation ENKMenuTarget
- (void)switchCompany:(id)sender { (void)sender; goMenuSwitchCompany(); }
- (void)generateDemo:(id)sender { (void)sender; goMenuGenerateDemo(); }
- (void)langNB:(id)sender { (void)sender; goMenuLang((char *)"nb"); }
- (void)langPT:(id)sender { (void)sender; goMenuLang((char *)"pt"); }
- (void)langEN:(id)sender { (void)sender; goMenuLang((char *)"en"); }
@end

static ENKMenuTarget *gTarget = nil;

static void addItem(NSMenu *menu, NSString *title, SEL action, NSString *key, id target) {
  NSMenuItem *it = [[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key];
  if (target) {
    [it setTarget:target];
  }
  [menu addItem:it];
}

void installAppMenu(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    gTarget = [[ENKMenuTarget alloc] init];
    NSMenu *mainMenu = [[NSMenu alloc] init];

    // App-meny (skjul / avslutt)
    NSMenuItem *appItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:appItem];
    NSMenu *appMenu = [[NSMenu alloc] init];
    [appItem setSubmenu:appMenu];
    addItem(appMenu, @"Skjul ENK Regnskap", @selector(hide:), @"h", NSApp);
    [appMenu addItem:[NSMenuItem separatorItem]];
    addItem(appMenu, @"Avslutt ENK Regnskap", @selector(terminate:), @"q", NSApp);

    // Rediger-meny (klipp ut / kopier / lim inn i webview)
    NSMenuItem *editItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:editItem];
    NSMenu *editMenu = [[NSMenu alloc] initWithTitle:@"Rediger"];
    [editItem setSubmenu:editMenu];
    addItem(editMenu, @"Angre", @selector(undo:), @"z", nil);
    addItem(editMenu, @"Gjør om", @selector(redo:), @"Z", nil);
    [editMenu addItem:[NSMenuItem separatorItem]];
    addItem(editMenu, @"Klipp ut", @selector(cut:), @"x", nil);
    addItem(editMenu, @"Kopier", @selector(copy:), @"c", nil);
    addItem(editMenu, @"Lim inn", @selector(paste:), @"v", nil);
    addItem(editMenu, @"Merk alt", @selector(selectAll:), @"a", nil);

    // Foretak-meny
    NSMenuItem *foretakItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:foretakItem];
    NSMenu *foretakMenu = [[NSMenu alloc] initWithTitle:@"Foretak"];
    [foretakItem setSubmenu:foretakMenu];
    addItem(foretakMenu, @"Bytt foretak…", @selector(switchCompany:), @"o", gTarget);
    addItem(foretakMenu, @"Generer testdata", @selector(generateDemo:), @"", gTarget);

    // Språk-meny
    NSMenuItem *langItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:langItem];
    NSMenu *langMenu = [[NSMenu alloc] initWithTitle:@"Språk"];
    [langItem setSubmenu:langMenu];
    addItem(langMenu, @"Norsk bokmål", @selector(langNB:), @"", gTarget);
    addItem(langMenu, @"Português", @selector(langPT:), @"", gTarget);
    addItem(langMenu, @"English", @selector(langEN:), @"", gTarget);

    [NSApp setMainMenu:mainMenu];
  });
}
