package slc

import (
	"context"
	"testing"
	"time"
)

type fsmtest struct {
	name      string
	contract  string
	state     string
	events    []fsmEvent
	options   []FSMOption
	shouldErr bool
}

type fsmEvent struct {
	name      string
	payload   any
	shouldErr bool
}

func TestNewStateMachine(t *testing.T) {
	tests := []fsmtest{
		{
			name:      "Test NewStateMachine",
			contract:  "./tests/minimal_ok.yaml",
			state:     "Draft",
			options:   []FSMOption{},
			shouldErr: false,
		},
		{
			name:      "Test NewStateMachine with invalid initial state",
			contract:  "./tests/invalid_initial.yaml",
			state:     "Nonexistent",
			options:   []FSMOption{},
			shouldErr: true,
		},
		{
			name:     "Test NewStateMachine with option",
			contract: "./tests/minimal_ok.yaml",
			state:    "Draft",
			options: []FSMOption{
				WithFSPolicyFiles("./tests/policies"),
			},
			shouldErr: false,
		},
	}

	t.Parallel()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			c, err := GetFSContract(test.contract)
			if err != nil {
				t.Fatal(err)
			}

			sm, err := NewStateMachine(ctx, test.state, c, test.options...)
			if err != nil && !test.shouldErr {
				t.Fatalf("unexpected error: %s", err)
			}
			if err == nil && test.shouldErr {
				t.Fatal("expected error, got nil")
			}
			if err == nil && !test.shouldErr {
				if sm == nil {
					t.Fatal("expected state machine, got nil")
				}
			}
		})
	}
}

func TestStateMachineTransitions(t *testing.T) {
	tests := []fsmtest{
		{
			name:     "Test NewStateMachine with state transitions",
			contract: "./tests/minimal_ok.yaml",
			state:    "Draft",
			options: []FSMOption{
				WithFSPolicyFiles("./tests/policies/"),
			},
			events: []fsmEvent{
				{
					name:      "com.decombine.signature.sign",
					payload:   map[string]interface{}{"user": "bob"},
					shouldErr: true,
				},
				{
					name:      "com.decombine.signature.sign",
					payload:   nil,
					shouldErr: true,
				},
				{
					name:      "com.decombine.signature.sign",
					payload:   map[string]interface{}{"user": "admin"},
					shouldErr: false,
				},
			},
			shouldErr: false,
		},
	}

	t.Parallel()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			c, err := GetFSContract(test.contract)
			if err != nil {
				t.Fatal(err)
			}

			sm, err := NewStateMachine(ctx, test.state, c, test.options...)
			if err != nil && !test.shouldErr {
				t.Fatalf("unexpected error: %s", err)
			}

			for _, event := range test.events {
				input := TransitionCtx{Input: event.payload}
				ctx = NewTransitionContext(ctx, &input)
				err = sm.FireCtx(ctx, event.name)
				if err != nil && !event.shouldErr {
					t.Fatalf("unexpected error: %s", err)
				}
				if err == nil && event.shouldErr {
					t.Fatal("expected error, got nil")
				}
			}

			if err == nil && test.shouldErr {
				t.Fatal("expected error, got nil")
			}
			if err == nil && !test.shouldErr {
				if sm == nil {
					t.Fatal("expected state machine, got nil")
				}
			}
		})
	}
}
