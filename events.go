package slc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	CloudEventsVersion = "1.0"
	StrictHeaders      = false
)

// CreateEvent creates a new Event with the given type.
func (c *Contract) CreateEvent(eventType, source string) (cloudevents.Event, error) {
	if eventType == "" {
		return cloudevents.Event{}, errors.New("event type cannot be empty")
	}
	ev := cloudevents.NewEvent()
	ev.SetSpecVersion(CloudEventsVersion)
	ev.SetType(eventType)
	ev.SetID(uuid.New().String())
	ev.SetTime(time.Now())
	ev.SetSource(source)
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

// ConsumeEvent consumes an Event and initiates State Transition if the Event is relevant.
func (r *Reconciler) ConsumeEvent(ctx context.Context, event *cloudevents.Event, eligible []Transition) error {
	state, _ := r.getState(ctx)

	for _, t := range eligible {
		if t.On == "" {
			// No event to process
			continue
		}
		if t.On == event.Type() {
			log.Printf("Event %s triggers transition to %s", event.Type(), t.To)
			input := TransitionCtx{Input: string(event.Data())}
			tCtx := NewTransitionContext(ctx, &input)
			fire := r.FSM.FireCtx(tCtx, t.On, &input)
			if fire != nil {
				log.Printf("Transition failed with error: %v", fire.Error())
			} else {
				log.Printf("Transition successful")

				if r.Client != nil {
					err := r.checkForExitActions(ctx, state)
					if err != nil {
						log.Printf("Error checking for Exit Actions: %v", err)
					}
				}

				r.FSM.OnTransitioning()
			}
		}
	}
	return nil
}

func jetstreamToCloudEvent(m jetstream.Msg) (*cloudevents.Event, error) {
	ev := &event.Event{}
	// Attempt to unmarshal the data into a CloudEvent
	err := json.Unmarshal(m.Data(), ev)
	if err == nil {
		// Data is already a CloudEvent
		return ev, nil
	}
	if ev.Type() != "" {
		// If the type is set, we can assume it's a CloudEvent
		return ev, nil
	}

	// TODO: Review specification for options around encoding. Update handling appropriately.
	newEv := cloudevents.NewEvent()
	err = newEv.SetData(cloudevents.ApplicationJSON, m.Data())
	if err != nil {
		return nil, fmt.Errorf("failed to set cloudevent payload from JetStream message")
	}

	// TODO: Clarify time format in specification.
	t, err := time.Parse(time.RFC3339, m.Headers().Get("time"))
	if err != nil {
		if StrictHeaders {
			return nil, fmt.Errorf("failed to parse time header")
		}
		t = time.Now()
	}
	newEv.SetTime(t)
	newEv.SetID(m.Headers().Get("id"))

	if newEv.ID() == "" {
		if StrictHeaders {
			return nil, fmt.Errorf("failed to parse id header")
		}
		id := uuid.New().String()
		newEv.SetID(id)
	}

	newEv.SetSpecVersion(CloudEventsVersion)
	newEv.SetDataContentType(cloudevents.ApplicationJSON)

	return &newEv, nil
}
