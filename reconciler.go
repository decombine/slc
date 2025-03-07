package slc

import (
	"context"
	"fmt"
	"log"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/qmuntal/stateless"
)

type ReconcilerConfig struct {
	Workers     int
	MaxMassages int
}

type Reconciler struct {
	Config       ReconcilerConfig
	EventChannel chan *cloudevents.Event
	Consumer     jetstream.Consumer
	Stream       jetstream.JetStream
	Contract     *Contract
	FSM          *stateless.StateMachine
}

func NewReconciler(c *Contract, fsm *stateless.StateMachine, consumer jetstream.Consumer, stream jetstream.JetStream, config ReconcilerConfig) *Reconciler {
	return &Reconciler{
		Config:   config,
		Consumer: consumer,
		Stream:   stream,
		Contract: c,
		FSM:      fsm,
	}
}

func (r *Reconciler) Start(ctx context.Context) error {
	r.EventChannel = make(chan *cloudevents.Event)
	log.Printf("Starting Decombine Smart Legal Contract Reconciler...\n")
	state, err := r.getState(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	log.Printf("Current Contract State: %s\n", state.Name)

	eligible, err := r.eligibleTransitions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get eligible transitions: %w", err)
	}

	// Register a cloudevent to be published to the Stream when transitioning
	// so that other services can listen for state changes.
	r.FSM.OnTransitioning(func(context.Context, stateless.Transition) {
		evt, err := r.Contract.CreateEvent("com.decombine.slc.transitioning", "decombine")
		if err != nil {
			log.Printf("Error creating transitioning event: %v", err)
			return
		}
		payload, _ := evt.MarshalJSON()
		publish, err := r.Stream.Publish(ctx, "default", payload)
		if err != nil {
			log.Printf("Error publishing transitioning event: %v", err)
			return
		}
		log.Printf("SLC published transitioning CloudEvent: %v", publish)
	})

	// Start a goroutine to spawn workers for incoming message processing
	go func() {
		err := r.run()
		if err != nil {
			log.Printf("Error running reconciler: %v", err)
		}
	}()
	for {
		select {
		case event := <-r.EventChannel:
			log.Printf("Received event: %v at %s with Type %s", event.ID(), event.Time(), event.Type())
			err := r.ConsumeEvent(ctx, event, eligible)
			if err != nil {
				log.Printf("Error consuming event: %v", err)
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (r *Reconciler) Publish(context.Context, stateless.Transition) {

}

// run is a blocking function that listens for incoming messages from the JetStream Consumer.
func (r *Reconciler) run() error {
	iter, _ := r.Consumer.Messages(jetstream.PullMaxMessages(r.Config.MaxMassages))
	numWorkers := r.Config.Workers
	sem := make(chan struct{}, numWorkers)
	for {
		sem <- struct{}{}
		go func() {
			defer func() {
				<-sem
			}()
			msg, err := iter.Next()
			if err != nil {
				_ = fmt.Errorf("error processing message: %v", err)
				return
			}
			r.receiveJetStream(msg)
			_ = msg.Ack()
		}()
	}
}

func (r *Reconciler) receiveJetStream(msg jetstream.Msg) {
	go func() {
		ce, err := jetstreamToCloudEvent(msg)
		if err != nil {
			fmt.Println(err)
			return
		}
		r.EventChannel <- ce
	}()
}

// eligibleTransitions returns the Transition that can be triggered from the current State.
func (r *Reconciler) eligibleTransitions(ctx context.Context) ([]Transition, error) {
	s, err := r.getState(ctx)
	if err != nil {
		return nil, err
	}

	return s.Transitions, nil
}

// getState returns the current State of the Smart Legal Contract.
func (r *Reconciler) getState(ctx context.Context) (State, error) {
	fsmState, err := r.FSM.State(ctx)
	if err != nil {
		return State{}, fmt.Errorf("failed to get current state: %w", err)
	}
	for _, s := range r.Contract.State.States {
		if s.Name == fsmState {
			return s, nil
		}
	}
	return State{}, fmt.Errorf("state not found: %s", fsmState)
}
