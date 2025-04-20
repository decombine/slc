package slc

import (
	"context"
	"fmt"
	"log"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/qmuntal/stateless"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	Client       client.Client
	//Logger       *slog.Logger
}

type ReconcilerOptions struct {
	client client.Client
	//logger *slog.Logger
}

func WithKubernetesClient(client client.Client) ReconcilerOptions {
	return ReconcilerOptions{
		client: client,
	}
}

func NewReconciler(c *Contract, fsm *stateless.StateMachine, consumer jetstream.Consumer, stream jetstream.JetStream,
	config ReconcilerConfig, options ...ReconcilerOptions) *Reconciler {

	var r Reconciler

	for _, o := range options {
		if o.client != nil {
			r.Client = o.client
		}
	}

	r.Contract = c
	r.FSM = fsm
	r.Consumer = consumer
	r.Stream = stream
	r.Config = config

	return &r
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

	if r.Client != nil {
		log.Printf("Smart Legal Contract connected to Kubernetes API. Synchronizing any Entry workloads.")
		err := r.reconcileAction(ctx, state.Entry)
		if err != nil {
			return fmt.Errorf("failed to reconcile entry actions: %w", err)
		}
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

// runAction executes an Action triggered by a State Entry or Exit.
func (r *Reconciler) reconcileAction(ctx context.Context, action Action) error {
	if action.KubernetesActions != nil {
		for _, ka := range action.KubernetesActions {
			if ka.KustomizationSpec != nil {
				k := &kustomizev1.Kustomization{
					ObjectMeta: ctrl.ObjectMeta{
						Name:      ka.Name,
						Namespace: ka.Namespace,
					},
					Spec: *ka.KustomizationSpec,
				}
				if err := r.reconcileKustomization(ctx, k, ka.Namespace); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// reconcileKustomization reconciles a Kustomization resource. The Kustomization resource is an external resource
// that is managed by the kustomization-controller.
func (r *Reconciler) reconcileKustomization(ctx context.Context, kustomization *kustomizev1.Kustomization, namespace string) error {
	existing := &kustomizev1.Kustomization{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: kustomization.Name, Namespace: namespace}, existing)
	if err != nil && apierrors.IsNotFound(err) {
		// Create the Kustomization
		log.Printf("Creating Kustomization %s/%s", kustomization.Namespace, kustomization.Name)
		return r.Client.Create(ctx, kustomization)
	} else if err != nil {
		return err
	}

	return nil

	// No need to update the kustomization. The kustomize-controller will take care of that.
	// return r.Update(ctx, kustomization)
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

// checkForExitActions checks if the transitioning State has any Exit Actions to reconcile.
func (r *Reconciler) checkForExitActions(ctx context.Context, state State) error {
	if state.Exit.KubernetesActions != nil {
		for _, ka := range state.Exit.KubernetesActions {
			if ka.KustomizationSpec != nil {
				k := &kustomizev1.Kustomization{
					ObjectMeta: ctrl.ObjectMeta{
						Name:      ka.Name,
						Namespace: ka.Namespace,
					},
					Spec: *ka.KustomizationSpec,
				}
				log.Printf("Exiting State %s. Reconciling Kustomization %s/%s", state.Name, k.Namespace, k.Name)
				if err := r.reconcileKustomization(ctx, k, ka.Namespace); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
