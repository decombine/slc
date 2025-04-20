package slc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"

	gogithub "github.com/google/go-github/v69/github"
	"github.com/open-policy-agent/opa/v1/rego"
)

type PolicyOptions struct {
	Variables []Variables
}

func WithVars(variables []Variables) PolicyOptions {
	return PolicyOptions{
		Variables: variables,
	}
}

// getPolicyDirectory downloads a directory of OPA Rego policy files from a GitHub repository.
//
//nolint:unused
func getPolicyDirectory(uri, branch, token string) (map[string][]byte, error) {
	ctx := context.Background()
	c := NewGitHubClient(token)

	owner, repo, err := parseGitHubURL(uri)
	if err != nil {
		return nil, err
	}
	opts := &gogithub.RepositoryContentGetOptions{
		Ref: branch,
	}
	_, directoryContent, resp, err := c.Repositories.GetContents(ctx, owner, repo, uri, opts)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	filesContent := make(map[string][]byte)
	for _, file := range directoryContent {
		if *file.Type == "file" {
			fileContent, _, _, err := c.Repositories.GetContents(ctx, owner, repo, *file.Path, opts)
			if err != nil {
				return nil, err
			}
			if fileContent != nil {
				decodedContent, err := fileContent.GetContent()
				if err != nil {
					return nil, err
				}
				filesContent[*file.Path] = []byte(decodedContent)
			}
		}
	}

	return filesContent, nil
}

// getPolicyFile downloads a single OPA Rego policy file from a GitHub repository.
func getPolicyFile(ctx context.Context, uri, branch, token, filePath string) ([]byte, error) {
	c := NewGitHubClient(token)

	owner, repo, err := parseGitHubURL(uri)
	if err != nil {
		return nil, err
	}
	opts := &gogithub.RepositoryContentGetOptions{
		Ref: branch,
	}
	fileContent, _, resp, err := c.Repositories.GetContents(ctx, owner, repo, filePath, opts)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	if fileContent != nil {
		decodedContent, err := fileContent.GetContent()
		if err != nil {
			return nil, err
		}
		return []byte(decodedContent), nil
	}

	return nil, errors.New("file content is empty")
}

// NewRegoPolicyFS prepares an OPA Rego policy for evaluation from the local file system that can be used within
// Contract State Condition. This is useful for testing and development.
func NewRegoPolicyFS(ctx context.Context, module, query, path string) (*rego.PreparedEvalQuery, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	r := rego.New(
		rego.Query(query),
		rego.Module(module, string(data)),
	)

	compiled, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}

	return &compiled, nil
}

// NewRegoPolicy prepares an OPA Rego policy for evaluation that can be used within Contract State Condition.
func NewRegoPolicy(ctx context.Context, module, query string, policyContent []byte, variables []Variables, logger *slog.Logger) (*rego.PreparedEvalQuery, error) {
	var modifiedPolicy []byte
	if len(variables) > 0 {
		for _, variable := range variables {
			if variable.Name == "" || variable.Type == "" {
				return nil, errors.New("variable name and type must be specified")
			}
			logger.Debug("Overriding policy variable: %s with value: %s", variable.Name, variable.Default)
			content, err := overridePolicyValue(policyContent, "$"+variable.Name, variable.Default, logger)
			if err != nil {
				return nil, err
			}
			if len(content) == 0 {
				//log.Printf("No changes made to policy for variable: %s", variable.Name)
			}
			modifiedPolicy = content
		}
	}
	if len(modifiedPolicy) != 0 {
		policyContent = modifiedPolicy
	}

	r := rego.New(
		rego.Query(query),
		rego.Module(module, string(policyContent)),
	)

	compiled, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}

	return &compiled, nil
}

func overridePolicyValue(policyContent []byte, key, newValue string, logger *slog.Logger) ([]byte, error) {
	logger.Debug("Overriding Policy with variables", "original", string(policyContent), "target", key, "newValue", newValue)

	// Replace the placeholder or specific value in the policy
	modifiedPolicy := bytes.ReplaceAll(policyContent, []byte(key), []byte(newValue))

	if !bytes.Contains(modifiedPolicy, []byte(newValue)) {
		logger.Debug("Replacement failed. Key not found", "key", key, "newValue", newValue)
	} else {
		logger.Debug("Replacement successful", "key", key, "newValue", newValue)
	}

	return modifiedPolicy, nil
}
