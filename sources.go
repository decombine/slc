package slc

import (
	"errors"
	"os"
	"strings"
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
