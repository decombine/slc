package slc

import "testing"

type fsmtest struct {
	name      string
	contract  string
	state     string
	shouldErr bool
}

func TestNewStateMachine(t *testing.T) {
	tests := []fsmtest{
		{
			name:      "Test NewStateMachine",
			contract:  "./tests/minimal_ok.yaml",
			state:     "Draft",
			shouldErr: false,
		},
		{
			name:      "Test NewStateMachine with invalid initial state",
			contract:  "./tests/invalid_initial.yaml",
			state:     "Nonexistent",
			shouldErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := GetFSContract(test.contract)
			if err != nil {
				t.Fatal(err)
			}

			sm, err := NewStateMachine(test.state, c)
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
