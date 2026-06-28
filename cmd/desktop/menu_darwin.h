#ifndef ENK_MENU_H
#define ENK_MENU_H

// installAppMenu bygger den native macOS-menylinjen (app-, rediger-, foretak-
// og språkmenyer). Kalles etter at webview er opprettet.
void installAppMenu(void);

// copyToClipboard legger UTF-8-teksten på den generelle utklippstavlen.
void copyToClipboard(const char *text);

#endif
