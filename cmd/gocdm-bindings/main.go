package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/arran4/gocdm/bindings"
)

//export GoCDMDefaultConfigJSON
func GoCDMDefaultConfigJSON() *C.char {
	return C.CString(bindings.DefaultConfigJSON())
}

//export GoCDMLoadConfigJSON
func GoCDMLoadConfigJSON(path *C.char) *C.char {
	return C.CString(bindings.LoadConfigJSON(C.GoString(path)))
}

//export GoCDMDiscoverSessionsJSON
func GoCDMDiscoverSessionsJSON(homeDir *C.char) *C.char {
	return C.CString(bindings.DiscoverSessionsJSON(C.GoString(homeDir)))
}

//export GoCDMFreeCString
func GoCDMFreeCString(str *C.char) {
	C.free(unsafe.Pointer(str))
}

func main() {}
