package slc

import (
	"errors"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
)

const (
	Version = "0.1.0"
)

// Contract is the definition of a Decombine SLC.
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
	// The Network of the SLC
	Network Network `json:"network,omitempty" yaml:"network,omitempty" toml:"network,omitempty"`
	// Status of the SLC. Typically used by the runtime operating the SLC.
	Status Status `json:"status,omitempty" yaml:"status,omitempty" toml:"status,omitempty"`
}

// Network provides a reference for remote authentication, authorization, and state management.
type Network struct {
	// The Name of the Network. E.g., "decombine"
	Name string `json:"name" yaml:"name" toml:"name" `
	// The API hostname address of the Network. E.g., "api.decombine.com"
	API string `json:"api" yaml:"api" toml:"api" `
	// The URL of the Network for informational purposes. E.g., "https://decombine.com"
	URL string `json:"url" yaml:"url" toml:"url"`
	// EventURL is the URL of the Event Stream.
	EventURL string `json:"eventUrl" yaml:"eventUrl" toml:"eventUrl"`
	// The ClientID of the Network used for OIDC.
	ClientID string `json:"clientId" yaml:"clientId" tom:"clientId" `
	// The Relying Party (RP) Issuer used for OIDC.
	Issuer string `json:"issuer" yaml:"issuer" toml:"issuer"`
	// The DiscoveryEndpoint used for OIDC.
	DiscoveryEndpoint string `json:"discoveryEndpoint" yaml:"discoveryEndpoint" toml:"discoveryEndpoint"`
}

type ContractText struct {
	// Text URL of the Smart Legal Contract
	URL string `json:"url" yaml:"url" toml:"url" validate:"required,url"`
}

type TextSource struct {
	// Name of the TextSource. E.g., "agreement-markdown, services-contract.pdf, com.decombine.decision-slc"
	Name string `json:"name" yaml:"name" toml:"name"`
	// Kind of the TextSource is a string value representing the REST resource of the object. E.g., "concerto, markdown, pdf"
	Kind string `json:"kind" yaml:"kind" toml:"kind"`
	// URL of the TextSource is a string value representing the URL/URI to the given resource.
	URL string `json:"url" yaml:"url" toml:"url"`
}

// Condition is used to apply a Policy to a Smart Legal Contract State Transition.
// A Policy may include Open Policy Agent (OPA) Rego logic.
type Condition struct {
	// Name of the Condition.
	Name string `json:"name" yaml:"name" toml:"name"`
	// Value of the Condition. This may be used to represent a specific policy query.
	// E.g., "data.policy.allow"
	Value string `json:"value" yaml:"value" toml:"value"`
	// Path to the Condition logic. E.g., "./service/condition.rego"
	// Path is relative to the PolicySource.Directory.
	Path string `json:"path" yaml:"path" toml:"path"`
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

// A State is a configured Status for a Decombine Smart Legal Contract based on UML State Machine.
type State struct {
	// The name of the State
	Name string `json:"name" yaml:"name" toml:"name"`
	// The actions that are executed when the State is entered
	Entry Action `json:"entry" yaml:"entry" toml:"entry"`
	// The actions that are executed when the State is exited
	Exit Action `json:"exit" yaml:"exit" toml:"exit"`
	// The variables associated with the State
	Variables []Variables `json:"variables" yaml:"variables" toml:"variables"`
	// The transitions that are possible from this State
	Transitions []Transition `json:"transitions" yaml:"transitions" toml:"transitions" validate:"required,gte=0,dive"`
}

type Variables struct {
	// Name of the Variable
	Name string `json:"name" yaml:"name" toml:"name"`
	// The Type of the Variable (e.g., "string", "int", "bool")
	Type string `json:"type" yaml:"type" toml:"type"`
	// Default value of the Variable
	Default string `json:"default" yaml:"default" toml:"default"`
	// Ref is the reference to a specific source to populate the Variable
	Ref string `json:"ref" yaml:"ref" toml:"ref"`
	// Kind is a string value representing the REST resource of the object
	Kind string `json:"kind" yaml:"kind" toml:"kind"`
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

// TODO: Convert State Configuration errors to a custom Error type.

var (
	ErrStateNotFound = errors.New("state not found")
)

func (c *Contract) GetState(name string) (State, error) {
	for _, s := range c.State.States {
		if s.Name == name {
			return s, nil
		}
	}
	return State{}, ErrStateNotFound
}

func (c *Contract) InsertState(s State) {
	c.State.States = append(c.State.States, s)
}

func New() Contract {
	return Contract{
		Version: Version,
	}
}

// GetVariables returns the Variables for a given State. Variables can be
// used to store values associated with State Configuration.
func (c *Contract) GetVariables(state string) ([]Variables, error) {
	for _, s := range c.State.States {
		if s.Name == state {
			return s.Variables, nil
		}
	}
	return nil, ErrStateNotFound
}

// ParseConcertoPayload processes JSON dynamically to extract objects with a "$class" field.
func ParseConcertoPayload(data interface{}) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	switch v := data.(type) {
	case map[string]interface{}:
		// Check if the map contains a "$class" field
		if _, ok := v["$class"]; ok {
			results = append(results, v)
		}

		// Recursively check nested objects
		for _, value := range v {
			nestedResults, err := ParseConcertoPayload(value)
			if err != nil {
				return nil, err
			}
			results = append(results, nestedResults...)
		}

	case []interface{}:
		// Iterate through array elements
		for _, item := range v {
			nestedResults, err := ParseConcertoPayload(item)
			if err != nil {
				return nil, err
			}
			results = append(results, nestedResults...)
		}
	}

	return results, nil
}
