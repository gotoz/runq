// Package vs defines constants that are shared between
// runq-exec and init/vsockd.
package vs

const Port = 1

const (
	_ byte = iota
	ConnControl
	ConnExecute
	Done
)

const (
	ConfDefault byte = 1 << iota
	ConfTTY
	ConfStdin
)
