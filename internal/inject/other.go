//go:build !linux && !windows && !darwin

package inject

// Open reports that this system has no backend.
func Open() (Device, error) { return nil, errUnsupported }
