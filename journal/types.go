// journal package plus alerting, fsjournal, and mockjournal packages are
// copied from github.com/filecoin-project/lotus/journal with minor modifications
// to make them not lotus specific. Also moved over to avoid being dependent on
// a particular version of lotus.

package journal

import (
	"fmt"
	"strings"
	"time"
)

var (
	// DefaultDisabledEvents lists the journal events disabled by
	// default, usually because they are considered noisy.
	DefaultDisabledEvents = DisabledEvents{}
)

// DisabledEvents is the set of event types whose journaling is suppressed.
type DisabledEvents []EventType

// ParseDisabledEvents parses a string of the form: "system1:event1,system1:event2[,...]"
// into a DisabledEvents object, returning an error if the string failed to parse.
//
// It sanitizes strings via strings.TrimSpace.
func ParseDisabledEvents(s string) (DisabledEvents, error) {
	s = strings.TrimSpace(s) // sanitize
	evts := strings.Split(s, ",")
	ret := make(DisabledEvents, 0, len(evts))
	for _, evt := range evts {
		evt = strings.TrimSpace(evt) // sanitize
		s := strings.Split(evt, ":")
		if len(s) != 2 {
			return nil, fmt.Errorf("invalid event type: %s", s)
		}
		ret = append(ret, EventType{System: s[0], Event: s[1]})
	}
	return ret, nil
}

// EventType represents the signature of an event.
type EventType struct {
	System string
	Event  string

	// enabled stores whether this event type is enabled.
	enabled bool

	// safe is a sentinel marker that's set to true if this EventType was
	// constructed correctly (via Journal#RegisterEventType).
	safe bool
}

func (et EventType) String() string {
	return et.System + ":" + et.Event
}

// Enabled returns whether this event type is enabled in the journaling
// subsystem. Users are advised to check this before actually attempting to
// add a journal entry, as it helps bypass object construction for events that
// would be discarded anyway.
//
// All event types are enabled by default, and specific event types can only
// be disabled at Journal construction time.
func (et EventType) Enabled() bool {
	return et.safe && et.enabled
}

//go:generate go run github.com/golang/mock/mockgen -destination=mockjournal/journal.go -package=mockjournal . Journal

// Journal represents an audit trail of system actions.
//
// Every entry is tagged with a timestamp, a system name, and an event name.
// The supplied data can be any type, as long as it is JSON serializable,
// including structs, map[string]interface{}, or primitive types.
//
// For cleanliness and type safety, we recommend to use typed events. See the
// *Evt struct types in this package for more info.
type Journal interface {
	EventTypeRegistry

	// RecordEvent records this event to the journal, if and only if the
	// EventType is enabled. If so, it calls the supplier function to obtain
	// the payload to record.
	//
	// Implementations MUST recover from panics raised by the supplier function.
	RecordEvent(evtType EventType, supplier func() interface{})

	ReadEvents() ([]Event, error)

	// Close closes this journal for further writing.
	Close() error
}

// Event represents a journal entry.
//
// See godocs on Journal for more information.
type Event struct {
	EventType

	Timestamp time.Time   `json:"timestamp,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

func (e Event) String() string {
	d, ok := e.Data.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%s: %s", e.Timestamp.String(), e.EventType.String())
	}
	data := []string{}
	for k, v := range d {
		data = append(data, fmt.Sprintf("%s: %v", k, v))
	}
	return fmt.Sprintf("%s:\t%s\t%s", e.Timestamp.String(), e.EventType.String(), strings.Join(data, "\t"))
}
