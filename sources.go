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
	"oras.land/oras-go/v2/registry/remote"
)

const (
	// JSON is the JSON file format.
	JSON = "json"
	// YAML is the YAML file format.
	YAML = "yaml"
	// TOML is the TOML file format.
	TOML = "toml"
)

// MediaTypes

const (
	MediaTypeConcertoDataV2 = "application/vnd.concerto.data.v2+json"
)

func GetArtifact(repo *remote.Repository, url, artifactType string) ([]byte, error) {

	switch artifactType {
	case MediaTypeConcertoDataV2:
		return getBlob(repo, MediaTypeConcertoDataV2, url)
	default:
		return nil, fmt.Errorf("unsupported artifact type: %s", artifactType)
	}

}

func getBlob(repo *remote.Repository, artifactType, artifactURL string) ([]byte, error) {
	return nil, nil
}

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
	case JSON:
		return ValidateJSONPayload(input)
	case YAML:
		return ValidateYAMLPayload(input)
	case TOML:
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
	case JSON:
		return ValidateJSONPayload(input)
	case YAML:
		return ValidateYAMLPayload(input)
	case TOML:
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
		return JSON
	} else if strings.HasSuffix(path, ".yaml") {
		return YAML
	} else if strings.HasSuffix(path, ".toml") {
		return TOML
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
