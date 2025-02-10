package slc

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
)

type tt struct {
	name     string
	input    []byte
	expected []string
}

type TestCase struct {
	Name      string                 `yaml:"name"`
	ShouldErr bool                   `yaml:"err"`
	Contract  map[string]interface{} `yaml:"contract"`
}

type TOMLTestCase struct {
	Name      string         `yaml:"name"`
	ShouldErr bool           `yaml:"err"`
	Contract  map[string]any `yaml:"contract"`
}

// TODO: Add support for loading multiple cases.
//func loadCases(t *testing.T, path string) []TestCase {
//	file, err := os.Open(path)
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer file.Close()
//
//	var tcs []TestCase
//	if err := yaml.NewDecoder(file).Decode(&tcs); err != nil {
//		t.Fatal(err)
//	}
//	return tcs
//}

func loadCase(t *testing.T, path string) TestCase {
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var tc TestCase
	if err = yaml.NewDecoder(file).Decode(&tc.Contract); err != nil {
		t.Fatal(err)
	}

	return tc
}

func loadTOMLCase(t *testing.T, path string) TOMLTestCase {
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var tc TOMLTestCase
	if _, err = toml.NewDecoder(file).Decode(&tc.Contract); err != nil {
		t.Fatal(err)
	}

	return tc
}

func TestValidateJSONPayload(t *testing.T) {
	tests := []tt{
		{
			name:  "Invalid State and Policy",
			input: []byte(`{"name":"test","version":"0.0.1","text":{"url":"https://example.com"},"source":{"url":"https://example.com"},"policy":{"url":"https://example.com"}}`),
			expected: []string{
				`Key: 'Contract.State' Error:Field validation for 'State' failed on the 'required' tag`,
				//`Key: 'Contract.State.URL' Error:Field validation for 'URL' failed on the 'required' tag`,
			},
		},
		{
			name:  "invalid contract",
			input: []byte(`{"name":"test","version":"0.0.1","text":{"url":"https://example.com"},"source":{"url":"https://example.com"}}`),
			expected: []string{
				`Key: 'Contract.Policy.URL' Error:Field validation for 'URL' failed on the 'required' tag`,
				`Key: 'Contract.State' Error:Field validation for 'State' failed on the 'required' tag`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ValidateJSONPayload(test.input)
			var actual []string
			if err != nil {
				for _, err := range err.(validator.ValidationErrors) {
					actual = append(actual, err.Error())
				}
			}
			if !equalSlices(actual, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}

func TestValidateYAMLPayload(t *testing.T) {
	type tests struct {
		name string
		path string
		err  bool
	}

	testCases := []tests{
		{
			name: "Minimal Successful",
			path: "tests/minimal_ok.yaml",
			err:  false,
		},
		{
			name: "Version Missing",
			path: "tests/version_missing.yaml",
			err:  true,
		},
		{
			name: "Kustomization Ok",
			path: "tests/kustomization_ok.yaml",
			err:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := loadCase(t, tc.path)
			cBytes := new(bytes.Buffer)
			err := yaml.NewEncoder(cBytes).Encode(c.Contract)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ValidateYAMLPayload(cBytes.Bytes())
			if err == nil && tc.err {
				t.Fatal("expected error, got nil")
			}
			if err != nil && !tc.err {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}

}

func TestValidateTOMLPayload(t *testing.T) {
	type tests struct {
		name string
		path string
		err  bool
	}

	testCases := []tests{
		{
			name: "Minimal Successful",
			path: "tests/minimal_ok.toml",
			err:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := loadTOMLCase(t, tc.path)
			cBytes := new(bytes.Buffer)
			err := toml.NewEncoder(cBytes).Encode(c.Contract)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ValidateTOMLPayload(cBytes.Bytes())
			if err == nil && tc.err {
				t.Fatal("expected error, got nil")
			}
			if err != nil && !tc.err {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
}

func TestValidateRepository(t *testing.T) {
	type tests struct {
		shouldErr bool
		token     string
		uri       string
		branch    string
		path      string
	}

	testCases := []tests{
		{
			shouldErr: true,
			token:     "",
			uri:       "https://github.com/decombine/notaccesible",
			branch:    "main",
			path:      "contract.json",
		},
		// TODO: Add test case after adding new public SLC repository.
	}

	for _, tc := range testCases {
		ctx := context.Background()
		_, err := ValidateRepository(ctx, tc.token, tc.uri, tc.branch, tc.path)
		if err != nil {
			if tc.shouldErr {
				continue
			}
			t.Fatal(err)
		}
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
