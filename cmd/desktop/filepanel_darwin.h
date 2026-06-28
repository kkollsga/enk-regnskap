#ifndef FILEPANEL_DARWIN_H
#define FILEPANEL_DARWIN_H
// enableFileOpenPanel setter en WKUIDelegate på WKWebView-en i vinduet, slik at
// HTML <input type="file"> åpner en NSOpenPanel (inkl. iPhone-import via
// Continuity). webview/webview_go setter ingen UIDelegate, så uten dette skjer
// ingenting når man klikker «velg fil».
void enableFileOpenPanel(void *nsWindow);
#endif
