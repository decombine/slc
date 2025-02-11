package slc

import (
	"context"
	"errors"
	"strings"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/qmuntal/stateless"
)

// NewStateMachine initializes a Finite State Machine (FSM) for a given Contract. The FSM
// is constructed based on the StateConfiguration of the Contract. The FSM is set to the current State
// passed as an argument.
func NewStateMachine(current string, c *Contract) (*stateless.StateMachine, error) {
	var queue []string
	var initialExists, currentExists bool = false, false
	tree := stateless.NewStateMachine(current)

	// breadth-first search: iterate through each state, for each state, add it to the
	// state machine and include transitions. validate state and
	// transitions all have valid relations.
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
					if strings.HasPrefix("rego", states[t].Transitions[i].Conditions[j].Name) {
						guards = append(guards, func(_ context.Context, _ ...any) bool {
							return includeConditions(context.Background(), states[t].Transitions[i].Conditions[j])
						})
					}
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

func includeConditions(ctx context.Context, conditions ...Condition) bool {
	for _, condition := range conditions {
		if strings.HasPrefix("rego", condition.Name) {
			return regoCondition(ctx, condition)
		}
	}
	return false
}

func regoCondition(ctx context.Context, condition Condition) bool {
	policy := rego.New(
		rego.Query(condition.Name),
		rego.Input(condition.Value),
	)
	res, err := policy.Eval(ctx)
	if err != nil {
		panic(err)
	}
	return res.Allowed()
}

// StateTransitionValidator evaluates a State Machine and a possible transition
// to determine if the transition is valid or not.
func StateTransitionValidator(current string, ctr *Contract, tx Transition) (*stateless.StateMachine, error) {
	sm, err := NewStateMachine(current, ctr)
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
