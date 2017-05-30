package main

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"
)

func UnmarshalMetadata(b []byte) (Metadata, error) {
	var m metadata
	if err := yaml.Unmarshal(b, &m); err != nil {
		return Metadata{}, err
	}
	var deps [][]string
	for _, dep := range m.Dependencies {
		if dep.Type != ":runtime" {
			continue
		}

		deps = append(deps, []string{
			dep.Name,
			fmt.Sprintf("%s %s",
				dep.Requirement.Requirements[0][0],
				dep.Requirement.Requirements[0][1].(map[interface{}]interface{})["version"]),
		})
	}
	return Metadata{
		Name:         m.Name,
		Number:       m.Version.Version,
		Platform:     m.Platform,
		Dependencies: deps,
	}, nil
}

type Metadata struct {
	Name         string
	Number       string
	Platform     string
	Dependencies [][]string
}

type metadata struct {
	Name    string
	Version struct {
		Version string
	}
	Platform     string
	Dependencies []struct {
		Name        string
		Type        string
		Requirement struct {
			Requirements [][]interface{}
		}
	}
}
