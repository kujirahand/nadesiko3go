// Package host defines the boundary between the language runtime and its
// execution environment.
package host

import "time"

type Handle uint64
type TimerID uint64
type CallbackID uint32

type OpenFlag uint8

const (
	OpenRead OpenFlag = iota
	OpenWrite
	OpenAppend
)

type Host interface {
	Print(s string)
	Now() time.Time
	Env() Env
	Timer() Timer
}

// Env uses integer handles so Go pointers and maps do not cross the host
// boundary. Network and process operations will be added with nodelib.
type Env interface {
	Open(path string, flag OpenFlag) (Handle, error)
	Read(handle Handle, maxBytes int) ([]byte, error)
	Write(handle Handle, data []byte) (int, error)
	Close(handle Handle) error
}

// Timer queues opaque callback IDs. The VM owns the corresponding functions
// and controls their deterministic execution order.
type Timer interface {
	// Post schedules a callback to run once.
	Post(at time.Time, callback CallbackID) TimerID
	// PostEvery schedules a callback to run repeatedly, one interval apart.
	PostEvery(at time.Time, interval time.Duration, callback CallbackID) TimerID
	Cancel(id TimerID) bool
}
