package mselect

import (
	"reflect"
	"unsafe"
)

// A runtimeSelect is a single case passed to reflect_rselect.
// It is a copy type of reflect.runtimeSelect.
type runtimeSelect struct {
	Dir reflect.SelectDir // SelectSend, SelectRecv or SelectDefault
	Typ unsafe.Pointer    // *rtype, channel type
	Ch  unsafe.Pointer    // channel
	Val unsafe.Pointer    // ptr to data (SendDir) or ptr to receive buffer (RecvDir)
}

// iface is the header of a non-empty interface value.
// It is a copy type of runtime.iface.
type iface struct {
	Tab  unsafe.Pointer // *itab
	Data unsafe.Pointer
}

func to_rtype(t reflect.Type) unsafe.Pointer {
	return (*iface)(unsafe.Pointer(&t)).Data
}

//go:noescape
//go:linkname reflect_rselect reflect.rselect
func reflect_rselect([]runtimeSelect) (chosen int, recvOK bool)
