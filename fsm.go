package slc

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/qmuntal/stateless"
)

// TransitionCtx is used to pass input data to FSM Guard Functions for State Transition evaluation using
// Open Policy Agent (OPA) Rego policies.
type TransitionCtx struct {
	Input interface{} `json:"input" yaml:"input" toml:"input"`
}

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key int

// transitionKey is the key for transitionCtx values in Contexts. It is
// unexported; clients use transition.NewContext and transition.FromContext
// instead of using this key directly.
var transitionKey key

// NewTransitionContext returns a new Context that carries value TransitionCtx.
func NewTransitionContext(ctx context.Context, u *TransitionCtx) context.Context {
	return context.WithValue(ctx, transitionKey, u)
}

// FromContext returns the transition value stored in ctx, if any.
func FromContext(ctx context.Context) (*TransitionCtx, bool) {
	u, ok := ctx.Value(transitionKey).(*TransitionCtx)
	return u, ok
}

type FSMOption func(*FSMOptions)

// FSMOptions is a struct that holds options for configuring the behavior of the FSM.
type FSMOptions struct {
	GitHubPAT      string
	FilesystemPath string
	Logger         *slog.Logger
}

// WithGitHubToken is an FSMOption that changes the default behavior of the FSM to use a GitHub Personal Access Token
// when retrieving Policy files from a remote Git repository.
func WithGitHubToken(token string) FSMOption {
	return func(opts *FSMOptions) {
		opts.GitHubPAT = token
	}
}

// WithFSPolicyFiles is an FSMOption that changes the default behavior of the FSM to use Policy files
// from the file system instead of a remote Git repository. This is useful for testing and development.
// The path argument is relative to the current working directory.
func WithFSPolicyFiles(path string) FSMOption {
	return func(opts *FSMOptions) {
		opts.FilesystemPath = path
	}
}

// WithLogger is an FSMOption that allows the user to set a custom logger for the FSM.
func WithLogger(logger *slog.Logger) FSMOption {
	return func(opts *FSMOptions) {
		opts.Logger = logger
	}
}

// NewStateMachine initializes a Finite State Machine (FSM) for a given Smart Legal Contract. The FSM
// is constructed based on the StateConfiguration of the Contract. The FSM is set to the current State
// passed as an argument.
func NewStateMachine(ctx context.Context, current string, c *Contract, opts ...FSMOption) (*stateless.StateMachine, error) {
	options := &FSMOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Use default logger if none provided
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	var queue []string
	var initialExists, currentExists bool = false, false
	tree := stateless.NewStateMachine(current)

	// Queue the states and validate the initial and current states exist.
	queueStates(c.State.States, &queue, c.State.Initial, current, &initialExists, &currentExists)

	if !initialExists || !currentExists {
		// TODO: Convert to custom error type.
		return nil, errors.New("state configuration is invalid: initial or current state not found")
	}

	if len(queue) == 0 {
		// TODO: Convert to custom error type.
		return nil, errors.New("state configuration is invalid: no states found")
	}

	visited := make(map[string]bool)
	// Create a map to store all states (including substates)
	stateMap := make(map[string]State)
	// Create a map to store parent states for substates
	parentMap := make(map[string]string)
	buildStateMap(c.State.States, stateMap, parentMap)

	for len(queue) > 0 {
		currentState := queue[0]
		queue = queue[1:]
		if visited[currentState] {
			continue
		}
		visited[currentState] = true

		// Find the state in the map
		if state, exists := stateMap[currentState]; exists {
			// Process transitions for this state
			processStateTransitions(ctx, tree, state, stateMap, visited, &queue, options, c, logger, parentMap)
		}
	}

	return tree, nil
}

// queueStates adds all states and their substates to the queue
// and checks if the initial and current states exist
func queueStates(states []State, queue *[]string, initial string, current string, initialExists *bool, currentExists *bool) {
	for _, state := range states {
		*queue = append(*queue, state.Name)
		if state.Name == initial {
			*initialExists = true
		}
		if state.Name == current {
			*currentExists = true
		}

		// Recursively process substates
		if len(state.Substates) > 0 {
			queueStates(state.Substates, queue, initial, current, initialExists, currentExists)
		}
	}
}

// buildStateMap creates maps of all states (including substates) for lookup.
func buildStateMap(states []State, stateMap map[string]State, parentMap map[string]string) {
	for _, state := range states {
		stateMap[state.Name] = state

		// Recursively process substates and set parent
		if len(state.Substates) > 0 {
			for _, substate := range state.Substates {
				parentMap[substate.Name] = state.Name
			}
			buildStateMap(state.Substates, stateMap, parentMap)
		}
	}
}

// processStateTransitions processes transitions for a state and adds them to the FSM.
func processStateTransitions(ctx context.Context, tree *stateless.StateMachine, state State, stateMap map[string]State,
	visited map[string]bool, queue *[]string, options *FSMOptions, c *Contract, logger *slog.Logger, parentMap map[string]string) {

	// Process transitions for this state
	for i := 0; i < len(state.Transitions); i++ {
		var guards []stateless.GuardFunc
		transition := state.Transitions[i] // Create a local copy to avoid closure issues

		for j := 0; j < len(transition.Conditions); j++ {
			// Create local copies of loop variables to avoid closure issues
			condition := transition.Conditions[j]

			// If there are no conditions, the transition is always valid.
			// Otherwise, we construct Guard Conditions for the FSM.
			guards = append(guards, func(ctx context.Context, _ ...any) bool {
				inner, ok := FromContext(ctx)
				if !ok {
					inner = &TransitionCtx{Input: ""}
				}
				var policyContent []byte

				// Retrieve any policy files that will be required for the FSM to evaluate transitions.
				if options.FilesystemPath != "" {
					// Ensure the path ends with a trailing slash to avoid policy files not being found.
					if options.FilesystemPath[len(options.FilesystemPath)-1:] != "/" {
						options.FilesystemPath += "/"
					}

					policyContent, _ = os.ReadFile(options.FilesystemPath + condition.Path)
				} else if options.GitHubPAT != "" {
					policyContent, _ = getPolicyFile(ctx, c.Policy.URL, c.Policy.Branch, options.GitHubPAT, condition.Path)
				} else {
					policyContent, _ = getPolicyFile(ctx, c.Policy.URL, c.Policy.Branch, "", condition.Path)
				}

				var vars []Variables
				if state.Variables != nil {
					vars = append(vars, state.Variables...)
				}

				logger.Debug("Policy file content", "path", condition.Path, "content", string(policyContent))

				return regoCondition(*inner, condition, policyContent, vars, logger)
			})
		}

		// Check if the state is a substate
		if parentState, isSubstate := parentMap[state.Name]; isSubstate {
			// If it's a substate, use SubstateOf
			tree.Configure(state.Name).SubstateOf(parentState).Permit(transition.On, transition.To, guards...)
		} else {
			// Otherwise, configure normally
			tree.Configure(state.Name).Permit(transition.On, transition.To, guards...)
		}

		// Add destination state to queue if not visited
		if !visited[transition.To] {
			*queue = append(*queue, transition.To)
		}
	}

	// Process substates
	for _, substate := range state.Substates {
		if !visited[substate.Name] {
			*queue = append(*queue, substate.Name)
		}
	}
}

func regoCondition(tCtx TransitionCtx, condition Condition, policyContent []byte, variables []Variables, logger *slog.Logger) bool {
	ctx := context.Background()

	policy, err := NewRegoPolicy(ctx, condition.Name, condition.Value, policyContent, variables, logger)
	if err != nil {
		panic(err)
	}
	res, err := policy.Eval(ctx, rego.EvalInput(tCtx.Input))
	if err != nil {
		panic(err)
	}
	return res.Allowed()
}

// StateTransitionValidator evaluates a State Machine and a possible transition
// to determine if the transition is valid or not.
func StateTransitionValidator(ctx context.Context, current string, ctr *Contract, tx Transition) (*stateless.StateMachine, error) {
	sm, err := NewStateMachine(ctx, current, ctr)
	if err != nil {
		return nil, err
	}

	if current == ctr.State.Initial {
		permitted, err := sm.PermittedTriggers()
		if err != nil {
			return nil, err
		}
		// Check the Contract object to determine what transitions are possible. Since there
		// are no events in the stream, we can only rely on the Contract object to determine
		// if the transition is valid.
		for _, s := range ctr.State.States {
			if s.Name == current {
				for _, st := range s.Transitions {
					if st.On == tx.On && st.To == tx.To {
						// For the sake of completeness, also validate that the trigger is permitted by FSM.
						for _, e := range permitted {
							if e == tx.On {
								if err = sm.Fire(tx.On); err != nil {
									return nil, err
								}
								return sm, nil
							}
						}

					}
				}
			}
		}
	}

	// If there are no more events in the queue, we can validate against the proposed transition tx.
	permitted, err := sm.PermittedTriggers()
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(permitted); i++ {
		if permitted[i] == tx.On {
			for _, s := range ctr.State.States {
				if s.Name == current {
					for _, st := range s.Transitions {

						if st.On == tx.On && st.To == tx.To {
							if err = sm.Fire(tx.On); err != nil {
								return nil, err
							}
							return sm, nil
						}
					}
				}
			}
		}
	}

	return sm, errors.New("transition not valid")

}
