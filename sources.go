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
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
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
	MediaTypeConcertoDataV2             = "application/vnd.concerto.data.v2+json"
	MediaTypeDecombineTemplateSlcV2JSON = "application/vnd.decombine.template.slc.v1+json"
)

type RepoCredential struct {
	Username     string
	Password     string
	RefreshToken string
	AccessToken  string
}

type OCITarget struct {
	Registry string
	Repo     string
	Tag      string
}

type ClientOpts struct {
	// OCI is the target to pull the OCI artifact from.
	OCI OCITarget
	// OCICreds are the credentials to use for the OCI registry.
	OCICreds RepoCredential
	// OCIPullPath is the target to pull the OCI artifact to.
	OCIPullPath string
}

func WithOCI(registry, repo, tag string) ClientOpts {
	return ClientOpts{
		OCI: OCITarget{
			Registry: registry,
			Repo:     repo,
			Tag:      tag,
		},
	}
}

func WithOCICreds(credential RepoCredential) ClientOpts {
	return ClientOpts{
		OCICreds: credential,
	}
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

//func GetContract(opts ...ClientOpts) (*Contract, error) {
//	ctx := context.Background()
//
//	var options ClientOpts
//	for _, opt := range opts {
//		if opt.OCI.Registry != "" {
//			options.OCI = opt.OCI
//		}
//		if opt.OCICreds.Username != "" {
//			options.OCICreds = opt.OCICreds
//		}
//	}
//	return repo, nil
//}

func ociRepo(registry, repo string, opts ...ClientOpts) (*remote.Repository, error) {
	var options ClientOpts
	for _, opt := range opts {
		if opt.OCICreds.Username != "" {
			options.OCICreds = opt.OCICreds
		}
	}

	r, err := remote.NewRepository(registry + "/" + repo)
	if err != nil {
		return nil, err
	}

	if options.OCICreds.Username != "" {
		creds := reflectCreds(options.OCICreds)

		r.Client = &auth.Client{
			Client:     retry.DefaultClient,
			Cache:      auth.NewCache(),
			Credential: auth.StaticCredential(registry, creds),
		}
	}
	return r, nil
}

// GetArtifact retrieves an artifact from a remote repository. GetArtifact is not yet implemented while
// the OCI design is being finalized. The function is a placeholder for future use.
//
//nolint:unused
func GetArtifact(ctx context.Context, target OCITarget, opts ...ClientOpts) (v1.Descriptor, error) {
	path := "./oci"
	for _, opt := range opts {
		if opt.OCIPullPath != "" {
			path = opt.OCIPullPath
		}
	}

	fs, err := file.New(path)
	if err != nil {
		return v1.Descriptor{}, err
	}
	defer fs.Close()
	repo, err := ociRepo(target.Registry, target.Repo, opts...)
	if err != nil {
		return v1.Descriptor{}, err
	}
	manifestDescriptor, err := oras.Copy(ctx, repo, target.Tag, fs, target.Tag, oras.DefaultCopyOptions)
	if err != nil {
		return v1.Descriptor{}, err
	}
	return manifestDescriptor, nil

}

// getBlob retrieves a blob from a remote repository. See GetArtifact for more details.
//
//nolint:unused
func getBlob(repo *remote.Repository, artifactType, artifactURL string) ([]byte, error) {
	return nil, nil
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

// Reflects credentials to the OCI client authentication struct.
// This avoids client libraries needing to directly use the ORAS creds.
func reflectCreds(creds RepoCredential) auth.Credential {
	return auth.Credential{
		Username:     creds.Username,
		Password:     creds.Password,
		RefreshToken: creds.RefreshToken,
		AccessToken:  creds.AccessToken,
	}
}
