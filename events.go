package slc

import (
	"errors"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/qmuntal/stateless"
)

const (
	CloudEventsVersion = "1.0"
)

// CreateEvent creates a new Event with the given type.
func (c *Contract) CreateEvent(eventType string) (cloudevents.Event, error) {
	if eventType == "" {
		return cloudevents.Event{}, errors.New("event type cannot be empty")
	}
	ev := cloudevents.NewEvent()
	ev.SetSpecVersion(CloudEventsVersion)
	ev.SetType(eventType)
	return ev, nil
}

// GetEvents returns a list of all events that the Contract StateConfiguration has registered.
func (c *Contract) GetEvents() []string {
	var evt []string
	for _, s := range c.State.States {
		for _, t := range s.Transitions {
			// Register each event
			evt = append(evt, t.On)
		}
	}
	return evt
}

// IsEventRegistered determines if the Event is registered in a Transition.
func (c *Contract) IsEventRegistered(event cloudevents.Event) bool {
	name := event.Type()
	events := c.GetEvents()
	for _, e := range events {
		if e == name {
			return true
		}
	}
	return false
}

// ConsumeEvent consumes an Event and transitions the Contract FSM to the next State.
func (c *Contract) ConsumeEvent(event cloudevents.Event, fsm *stateless.StateMachine) error {
	if !c.IsEventRegistered(event) {
		return errors.New("event not registered in a contract transition")
	}
	// Find the Transition
	for _, s := range c.State.States {
		for _, t := range s.Transitions {
			if t.On == event.Type() {
				permitted, err := fsm.PermittedTriggers()
				if err != nil {
					return err
				}
				for _, p := range permitted {
					if p == t.On {
						// Transition to the next state
						err = fsm.Fire(t.On)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}
