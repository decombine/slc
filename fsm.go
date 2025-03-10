package slc

import (
	"context"
	"errors"
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

// NewStateMachine initializes a Finite State Machine (FSM) for a given Smart Legal Contract. The FSM
// is constructed based on the StateConfiguration of the Contract. The FSM is set to the current State
// passed as an argument.
func NewStateMachine(ctx context.Context, current string, c *Contract, opts ...FSMOption) (*stateless.StateMachine, error) {
	options := &FSMOptions{}
	for _, opt := range opts {
		opt(options)
	}

	var queue []string
	var initialExists, currentExists bool = false, false
	tree := stateless.NewStateMachine(current)

	// Queue the states and validate the initial and current states exist.
	for i := 0; i < len(c.State.States); i++ {
		queue = append(queue, c.State.States[i].Name)
		if c.State.States[i].Name == c.State.Initial {
			initialExists = true
		}
		if c.State.States[i].Name == current {
			currentExists = true
		}
	}

	if !initialExists || !currentExists {
		return nil, errors.New("state configuration is invalid: initial or current state not found")
	}

	if len(queue) == 0 {
		return nil, errors.New("state configuration is invalid: no states found")
	}

	visited := make(map[string]bool)

	for len(queue) > 0 {
		currentState := queue[0]
		queue = queue[1:]
		if visited[currentState] {
			continue
		}
		visited[currentState] = true
		states := c.State.States
		for t := range states {

			for i := 0; i < len(states[t].Transitions); i++ {
				if states[t].Name != currentState {
					continue
				}

				var guards []stateless.GuardFunc
				for j := 0; j < len(states[t].Transitions[i].Conditions); j++ {

					// If there are no conditions, the transition is always valid.
					// Otherwise, we construct Guard Conditions for the FSM.
					guards = append(guards, func(ctx context.Context, _ ...any) bool {
						inner, ok := FromContext(ctx)
						if !ok {
							inner = &TransitionCtx{Input: ""}
						}
						var policyContent []byte

						// Retrieve any policy files that will be required for the FSM to evaluate transitions.
						// For now, these are loaded into memory. This is also being done in a loop, which is not ideal,
						// but it simplifies the scenario for now. This should be refactored to load the policy files
						// earlier in the function, but that will then require a mapping between policy files and conditions.
						// TODO: This may have a large memory footprint; profiling should be done in various scenarios when feasible.
						// TODO: Refactor this to a cleaner implementation.
						if options.FilesystemPath != "" {
							policyContent, _ = os.ReadFile(options.FilesystemPath + states[t].Transitions[i].Conditions[j].Path)
						} else if options.GitHubPAT != "" {
							policyContent, _ = getPolicyFile(ctx, c.Policy.URL, c.Policy.Branch, options.GitHubPAT, states[t].Transitions[i].Conditions[j].Path)
						} else {
							policyContent, _ = getPolicyFile(ctx, c.Policy.URL, c.Policy.Branch, "", states[t].Transitions[i].Conditions[j].Path)
						}

						return regoCondition(*inner, states[t].Transitions[i].Conditions[j], policyContent)
					})
				}

				tree.Configure(currentState).Permit(states[t].Transitions[i].On, states[t].Transitions[i].To, guards...)
				if !visited[states[t].Name] {
					queue = append(queue, states[t].Name)
				}
			}

		}
	}

	return tree, nil
}

func regoCondition(tCtx TransitionCtx, condition Condition, policyContent []byte) bool {
	ctx := context.Background()

	policy, err := NewRegoPolicy(ctx, condition.Name, condition.Value, policyContent)
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
