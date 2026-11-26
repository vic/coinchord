//go:build windows

package inject

import (
	"testing"
	"unsafe"
)

// INPUT is 40 bytes on 64-bit Windows; SendInput rejects other sizes.
func TestInputSize(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) == 8 && (unsafe.Sizeof(keyInput{}) != 40 || unsafe.Sizeof(ptrInput{}) != 40) {
		t.Fatalf("key %d ptr %d", unsafe.Sizeof(keyInput{}), unsafe.Sizeof(ptrInput{}))
	}
}
