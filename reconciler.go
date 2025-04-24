package slc

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/qmuntal/stateless"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcilerConfig struct {
	// LogLevel is the log level for the Reconciler.
	LogLevel slog.Level
	// UseCloudEventReceiver indicates if the Reconciler should start a CloudEvents receiver.
	// Can be combined with Reconciler.CloudEventOpts to configure the receiver.
	UseCloudEventReceiver bool
	// PublishSubject adds a Subject to Publish events to on the Stream.
	PublishSubject string
	// Workers is the number of workers to spawn for processing messages.
	Workers int
	// MaxMassages is the maximum number of messages to process at once.
	MaxMassages int
}

type Reconciler struct {
	Config         ReconcilerConfig
	EventChannel   chan *cloudevents.Event
	Consumer       jetstream.Consumer
	Stream         jetstream.JetStream
	Contract       *Contract
	FSM            *stateless.StateMachine
	Client         client.Client
	CloudEventOpts []http.Option
	Logger         *slog.Logger
}

type ReconcilerOptions struct {
	client         client.Client
	logger         *slog.Logger
	cloudEventOpts []http.Option
}

func WithKubernetesClient(client client.Client) ReconcilerOptions {
	return ReconcilerOptions{
		client: client,
	}
}

func WithReconcilerLogger(logger *slog.Logger) ReconcilerOptions {
	return ReconcilerOptions{
		logger: logger,
	}
}

func WithCloudEventReceiverOpts(ceOpts ...http.Option) ReconcilerOptions {
	return ReconcilerOptions{
		cloudEventOpts: ceOpts,
	}
}

func NewReconciler(c *Contract, fsm *stateless.StateMachine, consumer jetstream.Consumer, stream jetstream.JetStream,
	config ReconcilerConfig, options ...ReconcilerOptions) *Reconciler {

	var r Reconciler

	for _, o := range options {
		if o.client != nil {
			r.Client = o.client
		}
		if o.cloudEventOpts != nil {
			r.Config.UseCloudEventReceiver = true
			r.CloudEventOpts = o.cloudEventOpts
		}
		if o.logger != nil {
			r.Logger = o.logger
		} else {
			level := slog.LevelInfo
			if config.LogLevel != 0 {
				level = config.LogLevel
			}
			r.Logger = slog.New(slog.NewTextHandler(log.Writer(), &slog.HandlerOptions{
				Level: level,
			}))
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
	r.Logger.Info("Starting Decombine Smart Legal Contract Reconciler...")
	state, err := r.getState(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	r.Logger.Info("Contract", "Contract", r.Contract.Name, "State", state.Name)

	eligible, err := r.eligibleTransitions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get eligible transitions: %w", err)
	}

	if r.Client != nil {
		r.Logger.Info("Smart Legal Contract connected to Kubernetes API. Synchronizing any Entry workloads.")
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
			r.Logger.Error("Error creating transitioning event", "error", err)
			return
		}
		payload, _ := evt.MarshalJSON()
		if r.Config.PublishSubject != "" {
			publish, err := r.Stream.Publish(ctx, r.Config.PublishSubject, payload)
			if err != nil {
				r.Logger.Error("Error publishing transitioning event", "error", err)
				return
			}
			r.Logger.Info("Published transitioning event", "publish", publish)
		} else {
			r.Logger.Info("No publish subject configured. Skipping publishing transitioning event.")
		}
	})

	// Start a goroutine to spawn workers for incoming message processing
	go func() {
		err := r.run()
		if err != nil {
			r.Logger.Error("Error running reconciler", "error", err)
		}
	}()
	for {
		select {
		case event := <-r.EventChannel:
			r.Logger.Info("Received event", "type", event.Type(), "source", event.Source(), "id", event.ID())
			err := r.ConsumeEvent(ctx, event, eligible)
			if err != nil {
				r.Logger.Error("Error processing event", "error", err)
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
	ctx := context.Background()

	go func() {
		if r.Config.UseCloudEventReceiver {
			// Support flexible implementation if SLC operators prefer to customize their CloudEvent Receiver.
			var (
				cec cloudevents.Client
				err error
			)
			if r.CloudEventOpts != nil {
				cec, err = cloudevents.NewClientHTTP(r.CloudEventOpts...)
			} else {
				cec, err = cloudevents.NewClientHTTP()
			}
			if err != nil {
				log.Fatalf("failed to create client, %v", err)
			}

			r.Logger.Debug("Cloud Event receiver being activated")

			if err = cec.StartReceiver(ctx, r.receiveHttp); err != nil {
				log.Fatalf("failed to start receiver: %v", err)
			}
		}
	}()

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
// managed by the kustomization-controller.
func (r *Reconciler) reconcileKustomization(ctx context.Context, kustomization *kustomizev1.Kustomization, namespace string) error {
	existing := &kustomizev1.Kustomization{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: kustomization.Name, Namespace: namespace}, existing)
	if err != nil && apierrors.IsNotFound(err) {
		// Create the Kustomization
		r.Logger.Info("Creating Kustomization", "Kustomization", kustomization.Name, "Namespace", kustomization.Namespace)
		return r.Client.Create(ctx, kustomization)
	} else if err != nil {
		return err
	}

	return nil

	// No need to update the kustomization. The kustomize-controller will take care of that.
}

// receiveHttp is a callback function that receives CloudEvents from the CloudEvents Receiver.
func (r *Reconciler) receiveHttp(ctx context.Context, ce cloudevents.Event) {
	go func() {
		r.EventChannel <- &ce
	}()
}

// receiveJetStream is a callback function that receives messages from the JetStream Consumer.
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
				r.Logger.Info("Exiting State", "State", state.Name, "Kustomization", k.Name, "Namespace", k.Namespace)
				if err := r.reconcileKustomization(ctx, k, ka.Namespace); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
