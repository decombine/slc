package slc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	gogithub "github.com/google/go-github/v69/github"
)

func GetFSContract(path string) (*Contract, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	n := f.Name()
	var input []byte
	input, err = os.ReadFile(n)
	if err != nil {
		return nil, err
	}
	f.Close()
	switch getFileType(n) {
	case "json":
		return ValidateJSONPayload(input)
	case "yaml":
		return ValidateYAMLPayload(input)
	case "toml":
		return ValidateTOMLPayload(input)
	}
	return nil, errors.New("unknown or unsupported file format")
}

// GetGitHubContract retrieves a Contract from a remote GitHub repository.
// A Personal Access Token (PAT) token may be provided for private repositories.
func GetGitHubContract(token, uri, branch, path string) (*Contract, error) {
	ctx := context.Background()
	c := NewGitHubClient(token)
	owner, repo, err := parseGitHubURL(uri)
	if err != nil {
		return nil, err
	}
	opts := &gogithub.RepositoryContentGetOptions{
		Ref: branch,
	}
	content, _, resp, err := c.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	if content == nil {
		return nil, err
	}

	con, err := content.GetContent()
	if err != nil {
		return nil, err
	}

	input := []byte(con)
	switch getFileType(path) {
	case "json":
		return ValidateJSONPayload(input)
	case "yaml":
		return ValidateYAMLPayload(input)
	case "toml":
		return ValidateTOMLPayload(input)
	}
	return nil, errors.New("unknown or unsupported file format")
}

func NewGitHubClient(token string) *gogithub.Client {

	if token == "" {
		c := gogithub.NewClient(&http.Client{
			// A default timeout so the client doesn't hang indefinitely
			Timeout: 10 * time.Second,
		})
		return c
	}

	c := gogithub.NewClient(&http.Client{
		Timeout: 10 * time.Second,
	}).WithAuthToken(token)
	return c
}

func getFileType(path string) string {
	if strings.HasSuffix(path, ".json") {
		return "json"
	} else if strings.HasSuffix(path, ".yaml") {
		return "yaml"
	} else if strings.HasSuffix(path, ".toml") {
		return "toml"
	}
	return ""
}

func parseGitHubURL(gitHubURL string) (string, string, error) {
	parsedURL, err := url.Parse(gitHubURL)
	if err != nil {
		return "", "", err
	}

	// Split the path to get the owner and repo
	parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub URL: %s", gitHubURL)
	}

	return parts[0], parts[1], nil
}
