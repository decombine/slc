package slc

import (
	"errors"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
)

const (
	Version = "0.1.0"
)

// Contract is the definition of for a Decombine SLC.
type Contract struct {
	// The unique identifier (UUID) of the SLC. Typically created by the Network managing the SLC.
	ID string `json:"id,omitempty" yaml:"id,omitempty" toml:"id,omitempty"`
	// The friendly Name of the SLC
	Name string `json:"name" yaml:"name" toml:"name" validate:"required"`
	// The Version of the SLC schema
	Version string `json:"version" yaml:"version" toml:"version" validate:"required,semver"`
	// Text of the SLC
	Text ContractText `json:"text" yaml:"text" toml:"text"`
	// The Source of the SLC
	Source GitSource `json:"source" yaml:"source" toml:"source"`
	// The Policy included in the SLC
	Policy PolicySource `json:"policy" yaml:"policy" toml:"policy"`
	// The StateConfiguration of the SLC used to dictate a State Machine.
	State StateConfiguration `json:"state" yaml:"state" toml:"state" validate:"required"`
	// Status of the SLC. Typically used by the runtime operating the SLC.
	Status Status `json:"status,omitempty" yaml:"status,omitempty" toml:"status,omitempty"`
}

// Network provides a reference for remote authentication, authorization, and state management.
type Network struct {
	// The Name of the Network. E.g., "decombine"
	Name string `json:"name" yaml:"name" toml:"name" validate:"required"`
	// The API hostname address of the Network. E.g., "api.decombine.com"
	API string `json:"api" yaml:"api" toml:"api" validate:"required"`
	// The URL of the Network for informational purposes. E.g., "https://decombine.com"
	URL string `json:"url" yaml:"url" toml:"url" validate:"required"`
	// The ClientID of the Network used for OIDC.
	ClientID string `json:"clientId" yaml:"clientId" tom:"clientId" validate:"required"`
	// The Relying Party (RP) Issuer used for OIDC.
	Issuer string `json:"issuer" yaml:"issuer" toml:"issuer" validate:"required"`
	// The DiscoveryEndpoint used for OIDC.
	DiscoveryEndpoint string `json:"discoveryEndpoint" yaml:"discoveryEndpoint" toml:"discoveryEndpoint" validate:"required"`
}

type ContractText struct {
	// Text URL of the Smart Legal Contract
	URL string `json:"url" yaml:"url" toml:"url" validate:"required,url"`
}

// Condition is a value that must be satisfied (true) in
// order for a transition or action to occur.
type Condition struct {
	// The condition that must be satisfied for the guard to be true
	Name string `json:"name" yaml:"name" toml:"name"`
	// The value that the condition must be in order for the guard to be true
	Value string `json:"value" yaml:"value" toml:"value"`
}

// A GitSource is a Git repository source for Smart Legal Contracts.
type GitSource struct {
	// The type of the source
	Type string `json:"type,omitempty" yaml:"type,omitempty" toml:"type,omitempty"`
	// The URL of the Git repository
	URL string `json:"url" yaml:"url" toml:"url" validate:"required,url"`
	// The branch of the Git repository
	Branch string `json:"branch" yaml:"branch" toml:"branch"`
	// The path to the Smart Legal Contract Definition file
	Path string `json:"path" yaml:"path" toml:"path"`
}

// PolicySource for the Open Policy Agent (OPA) policies.
type PolicySource struct {
	// The branch of the Git repository
	Branch string `json:"branch" yaml:"branch" toml:"branch"`
	// The directory containing the OPA policies
	Directory string `json:"directory" yaml:"directory" toml:"directory"`
	// The URL of the Git repository
	URL string `json:"url" yaml:"url" toml:"url" validate:"required,url"`
}

// A State is a condition of being. It represents a snapshot of the current
// condition of a smart legal contract.
type State struct {
	// The name of the State
	Name string `json:"name" yaml:"name" toml:"name"`
	// The actions that are executed when the State is entered
	Entry Action `json:"entry" yaml:"entry" toml:"entry"`
	// The actions that are executed when the State is exited
	Exit Action `json:"exit" yaml:"exit" toml:"exit"`
	// The variables associated with the State
	Variables map[string]any `json:"variables" yaml:"variables" toml:"variables"`
	// The transitions that are possible from this State
	Transitions []Transition `json:"transitions" yaml:"transitions" toml:"transitions" validate:"required,gte=0,dive"`
}

// A StateConfiguration is a collection of States that define the State
// Machine of a Smart Legal Contract.
type StateConfiguration struct {
	// The Initial State of the SLC
	Initial string `json:"initial" yaml:"initial" toml:"initial" validate:"required"`
	// The URL of the StateConfiguration
	URL string `json:"url" yaml:"url" toml:"url" validate:"required,url"`
	// The States that comprise the SLC
	States []State `json:"states" yaml:"states" toml:"states" validate:"required,gte=1,dive"`
}

type Action struct {
	// The type of the action
	ActionType        string             `json:"actionType,omitempty" yaml:"actionType" toml:"actionType"`
	KubernetesActions []KubernetesAction `json:"kubernetesAction,omitempty" yaml:"kubernetesAction" toml:"kubernetesAction"`
}

type KubernetesAction struct {
	Name              string                         `json:"name,omitempty"`
	Namespace         string                         `json:"namespace,omitempty"`
	KustomizationSpec *kustomizev1.KustomizationSpec `json:"kustomizationSpec,omitempty" yaml:"kustomizationSpec" toml:"kustomizationSpec"`
}

// Transition is a change from one State to another.
type Transition struct {
	// The Name of the Transition
	Name string `json:"name" yaml:"name" toml:"name" validate:"required"`
	// The State To which the Transition leads
	To string `json:"to" yaml:"to" toml:"to" validate:"required"`
	// The Event that Triggers the Transition
	On string `json:"on" yaml:"on" toml:"on" validate:"required"`
	// The Guard Conditions that must be satisfied for the Transition to occur
	Conditions []Condition `json:"conditions" yaml:"conditions" toml:"conditions"`
}

type Status struct {
	// The current state of the smart legal contract
	CurrentState string `json:"currentState,omitempty" yaml:"currentState,omitempty" toml:"currentState,omitempty"`
	// The source state of the smart legal contract
	SourceState string `json:"sourceState,omitempty" yaml:"sourceState,omitempty" toml:"sourceState,omitempty"`
	// The policy state of the smart legal contract
	PolicyState string `json:"policyState,omitempty" yaml:"policyState,omitempty" toml:"policyState,omitempty"`
	// The workload state of the smart legal contract
	WorkloadState string `json:"workloadState,omitempty" yaml:"workloadState,omitempty" toml:"workloadState,omitempty"`
}

func (c *Contract) GetState(name string) (State, error) {
	for _, s := range c.State.States {
		if s.Name == name {
			return s, nil
		}
	}
	return State{}, errors.New("state not found")
}

func (c *Contract) InsertState(s State) {
	c.State.States = append(c.State.States, s)
}

func New() Contract {
	return Contract{
		Version: Version,
	}
}
