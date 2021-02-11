package utils

import "unsafe"

// unsafe, but lightning fast []byte to string conversion
// no allocation
// only english strings which are passed as []byte
func ToString(data []byte) string {
	return *(*string)(unsafe.Pointer(&data))
}
