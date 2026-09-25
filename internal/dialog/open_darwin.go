//go:build darwin

package dialog

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit -framework Foundation
#include <stdlib.h>
#import <AppKit/AppKit.h>

static void nspCopyGoString(char **dst, NSString *value) {
	if (dst == NULL) {
		return;
	}
	if (value == nil) {
		*dst = NULL;
		return;
	}
	const char *utf8 = [value UTF8String];
	if (utf8 == NULL) {
		*dst = NULL;
		return;
	}
	*dst = strdup(utf8);
}

static char **nspOpenFolders(const char *title, int *count) {
	__block char **paths = NULL;
	__block int selected = 0;
	void (^showPanel)(void) = ^{
		NSOpenPanel *panel = [NSOpenPanel openPanel];
		if (title != NULL) {
			NSString *folderTitle = [NSString stringWithUTF8String:title];
			[panel setTitle:folderTitle];
			[panel setMessage:folderTitle];
		}
		[panel setCanChooseFiles:NO];
		[panel setCanChooseDirectories:YES];
		[panel setAllowsMultipleSelection:YES];
		[panel setCanCreateDirectories:NO];
		[panel setResolvesAliases:YES];
		[panel setTreatsFilePackagesAsDirectories:NO];

		NSWindow *host = [NSApp keyWindow];
		if (host == nil) {
			host = [NSApp mainWindow];
		}

		NSModalResponse response;
		if (host != nil) {
			dispatch_semaphore_t done = dispatch_semaphore_create(0);
			__block NSModalResponse sheetResponse = NSModalResponseCancel;
			[panel beginSheetModalForWindow:host completionHandler:^(NSModalResponse result) {
				sheetResponse = result;
				dispatch_semaphore_signal(done);
			}];
			while (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 50 * NSEC_PER_MSEC)) != 0) {
				[[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.05]];
			}
			response = sheetResponse;
		} else {
			response = [panel runModal];
		}
		if (response != NSModalResponseOK) {
			return;
		}

		NSArray<NSURL *> *urls = [panel URLs];
		selected = (int)[urls count];
		if (selected == 0) {
			return;
		}
		paths = (char **)calloc((size_t)selected, sizeof(char *));
		if (paths == NULL) {
			selected = 0;
			return;
		}
		for (int index = 0; index < selected; index++) {
			nspCopyGoString(&paths[index], urls[(NSUInteger)index].path);
		}
	};

	if ([NSThread isMainThread]) {
		showPanel();
	} else {
		dispatch_sync(dispatch_get_main_queue(), showPanel);
	}
	if (count != NULL) {
		*count = selected;
	}
	return paths;
}

static void nspFreeFolderPaths(char **paths, int count) {
	if (paths == NULL) {
		return;
	}
	for (int index = 0; index < count; index++) {
		free(paths[index]);
	}
	free(paths);
}
*/
import "C"

import (
	"unsafe"
)

func OpenFolders() ([]string, error) {
	var count C.int
	title := C.CString(FolderDialogTitle)
	defer C.free(unsafe.Pointer(title))
	cPaths := C.nspOpenFolders(title, &count)
	if cPaths == nil || count == 0 {
		return nil, nil
	}
	defer C.nspFreeFolderPaths(cPaths, count)

	length := int(count)
	selected := make([]string, 0, length)
	slice := unsafe.Slice(cPaths, length)
	for _, path := range slice {
		if path == nil {
			continue
		}
		selected = append(selected, C.GoString(path))
	}
	return normalizeSelection(selected), nil
}
