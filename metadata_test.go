package main

import (
	"io/ioutil"
	"testing"
)

func TestDecodeMetadata(t *testing.T) {
	expected := struct {
		Name    string
		Version struct {
			Version string
		}
		Platform     string
		Dependencies []struct {
			Name        string
			Type        string
			Requirement map[string]interface{}
		}
	}{
		Name:     "sinatra",
		Platform: "ruby",
		Version: struct{ Version string }{
			Version: "2.0.0",
		},
		Dependencies: []struct {
			Name        string
			Type        string
			Requirement map[string]interface{}
		}{
			{Name: "rack", Type: ":runtime"},
			{Name: "tilt", Type: ":runtime"},
			{Name: "rack-protection", Type: ":runtime"},
			{Name: "mustermann", Type: ":runtime"},
		},
	}
	buf, _ := ioutil.ReadFile("testdata/sinatra-metadata.yaml")
	v, err := UnmarshalMetadata(buf)
	if err != nil {
		t.Error(err)
	}

	if v.Name != expected.Name {
		t.Error("invalid name")
	}
	if v.Platform != expected.Platform {
		t.Error("invalid platform")
	}
	if v.Number != expected.Version.Version {
		t.Error("invalid version")
	}
	if len(v.Dependencies) != len(expected.Dependencies) {
		t.Error("invalid version")
	}

	for i, dep := range v.Dependencies {
		if dep[0] != expected.Dependencies[i].Name {
			t.Errorf("invalid dep name; got: %q expected %q", dep[0], expected.Dependencies[i].Name)
		}

	}
}
