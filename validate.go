package slc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	gogithub "github.com/google/go-github/v69/github"
)

var validate *validator.Validate

var (
	ErrCannotUnmarshalJSON = errors.New("cannot unmarshal contract json")
	ErrCannotUnmarshalYAML = errors.New("cannot unmarshal contract yaml")
	ErrCannotUnmarshalTOML = errors.New("cannot unmarshal contract toml")
)

// ValidateJSONPayload validates a JSON payload input against the Contract struct.
func ValidateJSONPayload(in []byte) (*Contract, error) {
	var c Contract
	err := json.Unmarshal(in, &c)
	if err != nil {
		return nil, ErrCannotUnmarshalJSON
	}
	validate = validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ValidateYAMLPayload validates a YAML payload input against the Contract struct.
func ValidateYAMLPayload(in []byte) (*Contract, error) {
	var c Contract
	err := yaml.Unmarshal(in, &c)
	if err != nil {
		fmt.Printf("error: %v", err)
		return nil, ErrCannotUnmarshalYAML
	}
	validate = validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ValidateTOMLPayload validates a TOML payload input against the Contract struct.
func ValidateTOMLPayload(in []byte) (*Contract, error) {
	var c Contract
	err := toml.Unmarshal(in, &c)
	if err != nil {
		return nil, ErrCannotUnmarshalTOML
	}

	validate = validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ValidateRepository accepts a GitSource and validates the target
// repository exists, is accessible, and at minimum a contract.json.
func ValidateRepository(ctx context.Context, token, uri, branch, path string) (string, error) {

	c := NewGitHubClient(token)
	owner, repo, err := parseGitHubURL(uri)
	if err != nil {
		return "", err
	}
	opts := &gogithub.RepositoryContentGetOptions{
		Ref: branch,
	}
	content, _, resp, err := c.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", err
	}

	if content == nil {
		return "", err
	}

	con, err := content.GetContent()
	if err != nil {
		return "", err
	}

	return con, nil
}
