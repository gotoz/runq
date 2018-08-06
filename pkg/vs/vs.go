// Package vs defines constants that are shared between
// runq-exec and init/vsockd.
package vs

const Port uint32 = 1

const (
	_ byte = iota
	ConnControl
	ConnExecute
	Done
)

const (
	ConfTTY byte = 1 << iota
	ConfStdin
)
