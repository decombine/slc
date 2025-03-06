package slc

import (
	"context"
	"errors"
	"net/http"
	"os"

	gogithub "github.com/google/go-github/v69/github"
	"github.com/open-policy-agent/opa/v1/rego"
)

// getPolicyDirectory downloads a directory of OPA Rego policy files from a GitHub repository.
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
func NewRegoPolicy(ctx context.Context, module, query string, policyContent []byte) (*rego.PreparedEvalQuery, error) {
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
