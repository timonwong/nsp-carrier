//go:build windows

package dialog

import (
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

var (
	fileOpenDialogCLSID = ole.NewGUID("{DC1C5A9C-E88A-4dde-A5A1-60F82A20AEF7}")
	fileOpenDialogIID   = ole.NewGUID("{d57c7288-d4ad-4768-be02-9d969532d960}")
	shellItemIID        = ole.NewGUID("{43826d1e-e718-42ee-bc55-a1e261c37bfe}")
)

const (
	fosPickFolders      = 0x20
	fosForceFilesystem  = 0x40
	fosAllowMultiselect = 0x200
	fosPathMustExist    = 0x800
	fosFileMustExist    = 0x1000
	sigdnFileSysPath    = 0x80058000
	errorCancelled      = 0x800704C7
)

type iUnknownVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

type iModalWindowVtbl struct {
	iUnknownVtbl
	Show uintptr
}

type iFileDialogVtbl struct {
	iModalWindowVtbl
	SetFileTypes        uintptr
	SetFileTypeIndex    uintptr
	GetFileTypeIndex    uintptr
	Advise              uintptr
	Unadvise            uintptr
	SetOptions          uintptr
	GetOptions          uintptr
	SetDefaultFolder    uintptr
	SetFolder           uintptr
	GetFolder           uintptr
	GetCurrentSelection uintptr
	SetFileName         uintptr
	GetFileName         uintptr
	SetTitle            uintptr
	SetOkButtonLabel    uintptr
	SetFileNameLabel    uintptr
	GetResult           uintptr
	AddPlace            uintptr
	SetDefaultExtension uintptr
	Close               uintptr
	SetClientGuid       uintptr
	ClearClientData     uintptr
	SetFilter           uintptr
}

type iFileOpenDialogVtbl struct {
	iFileDialogVtbl
	GetResults       uintptr
	GetSelectedItems uintptr
}

type iFileOpenDialog struct {
	vtbl *iFileOpenDialogVtbl
}

type iUnknown struct {
	vtbl *iUnknownVtbl
}

type iShellItem struct {
	vtbl *iShellItemVtbl
}

type iShellItemVtbl struct {
	iUnknownVtbl
	BindToHandler  uintptr
	GetParent      uintptr
	GetDisplayName uintptr
	GetAttributes  uintptr
	Compare        uintptr
}

type iShellItemArray struct {
	vtbl *iShellItemArrayVtbl
}

type iShellItemArrayVtbl struct {
	iUnknownVtbl
	BindToHandler              uintptr
	GetPropertyStore           uintptr
	GetPropertyDescriptionList uintptr
	GetAttributes              uintptr
	GetCount                   uintptr
	GetItemAt                  uintptr
	EnumItems                  uintptr
}

func OpenFolders() ([]string, error) {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		oleError, ok := err.(*ole.OleError)
		if !ok || (oleError.Code() != ole.S_OK && oleError.Code() != 0x00000001) {
			return nil, err
		}
	} else {
		defer ole.CoUninitialize()
	}

	unknown, err := ole.CreateInstance(fileOpenDialogCLSID, fileOpenDialogIID)
	if err != nil {
		return nil, err
	}
	dialog := (*iFileOpenDialog)(unsafe.Pointer(unknown))
	defer dialog.release()

	title, err := syscall.UTF16PtrFromString(FolderDialogTitle)
	if err != nil {
		return nil, err
	}
	if err := dialog.setTitle(title); err != nil {
		return nil, err
	}

	options, err := dialog.getOptions()
	if err != nil {
		return nil, err
	}
	if err := dialog.setOptions(folderPickerOptions(options)); err != nil {
		return nil, err
	}

	if err := dialog.show(currentWindowHandle()); err != nil {
		if isCancelled(err) {
			return nil, nil
		}
		return nil, err
	}

	results, err := dialog.results()
	if err != nil {
		if isCancelled(err) {
			return nil, nil
		}
		return nil, err
	}
	return normalizeSelection(results), nil
}

func (d *iFileOpenDialog) setTitle(title *uint16) error {
	ret, _, _ := syscall.SyscallN(d.vtbl.SetTitle, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(title)))
	return hresultErr(ret)
}

func (d *iFileOpenDialog) getOptions() (uint32, error) {
	var options uint32
	ret, _, _ := syscall.SyscallN(d.vtbl.GetOptions, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&options)))
	return options, hresultErr(ret)
}

func (d *iFileOpenDialog) setOptions(options uint32) error {
	ret, _, _ := syscall.SyscallN(d.vtbl.SetOptions, uintptr(unsafe.Pointer(d)), uintptr(options))
	return hresultErr(ret)
}

func (d *iFileOpenDialog) show(hwnd uintptr) error {
	ret, _, _ := syscall.SyscallN(d.vtbl.Show, uintptr(unsafe.Pointer(d)), hwnd)
	return hresultErr(ret)
}

func (d *iFileOpenDialog) results() ([]string, error) {
	var items *iShellItemArray
	ret, _, _ := syscall.SyscallN(d.vtbl.GetResults, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&items)))
	if err := hresultErr(ret); err != nil {
		return nil, err
	}
	if items == nil {
		return nil, nil
	}
	defer items.release()

	var count uint32
	ret, _, _ = syscall.SyscallN(items.vtbl.GetCount, uintptr(unsafe.Pointer(items)), uintptr(unsafe.Pointer(&count)))
	if err := hresultErr(ret); err != nil {
		return nil, err
	}

	paths := make([]string, 0, count)
	for index := uint32(0); index < count; index++ {
		var item *iShellItem
		ret, _, _ = syscall.SyscallN(items.vtbl.GetItemAt, uintptr(unsafe.Pointer(items)), uintptr(index), uintptr(unsafe.Pointer(&item)))
		if err := hresultErr(ret); err != nil {
			return nil, err
		}
		if item == nil {
			continue
		}
		path, err := item.displayName()
		item.release()
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func (d *iFileOpenDialog) release() {
	syscall.SyscallN(d.vtbl.Release, uintptr(unsafe.Pointer(d)))
}

func (items *iShellItemArray) release() {
	syscall.SyscallN(items.vtbl.Release, uintptr(unsafe.Pointer(items)))
}

func (item *iShellItem) displayName() (string, error) {
	var ptr *uint16
	ret, _, _ := syscall.SyscallN(item.vtbl.GetDisplayName, uintptr(unsafe.Pointer(item)), uintptr(sigdnFileSysPath), uintptr(unsafe.Pointer(&ptr)))
	if err := hresultErr(ret); err != nil {
		return "", err
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(ptr)))
	return ole.LpOleStrToString(ptr), nil
}

func (item *iShellItem) release() {
	syscall.SyscallN(item.vtbl.Release, uintptr(unsafe.Pointer(item)))
}

func currentWindowHandle() uintptr {
	foreground := windows.GetForegroundWindow()
	if foreground == 0 {
		return 0
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(foreground, &pid); err != nil {
		return 0
	}
	if pid != windows.GetCurrentProcessId() {
		return 0
	}
	return uintptr(foreground)
}

func hresultErr(hr uintptr) error {
	if hr == 0 {
		return nil
	}
	return ole.NewError(hr)
}

func isCancelled(err error) bool {
	oleError, ok := err.(*ole.OleError)
	if !ok {
		return false
	}
	return uint32(oleError.Code()) == uint32(errorCancelled)
}
