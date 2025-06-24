package bkl

import (
	_ "embed"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

//go:embed tests.toml
var testsData []byte

//go:embed docs/sections.yaml
var sectionsData []byte

type MCPTestCase struct {
	Description string            `toml:"description"`
	Eval        []string          `toml:"eval"`
	Format      string            `toml:"format"`
	Expected    string            `toml:"expected,omitempty"`
	Error       string            `toml:"error,omitempty"`
	Files       map[string]string `toml:"files"`
	Diff        bool              `toml:"diff,omitempty"`
	Intersect   bool              `toml:"intersect,omitempty"`
	Required    bool              `toml:"required,omitempty"`
	Skip        bool              `toml:"skip,omitempty"`
}

type MCPDocSection struct {
	ID    string           `yaml:"id"`
	Title string           `yaml:"title"`
	Items []MCPContentItem `yaml:"items"`
}

type MCPContentItem struct {
	Type    string     `yaml:"type"`
	Content string     `yaml:"content"`
	Example MCPExample `yaml:"example"`
}

type MCPExample struct {
	Type       string       `yaml:"type"`
	Label      string       `yaml:"label"`
	Code       string       `yaml:"code"`
	Rows       []MCPGridRow `yaml:"rows"`
	Highlights []string     `yaml:"highlights"`
}

type MCPGridRow struct {
	Items    []MCPGridItem `yaml:"items"`
	Operator string        `yaml:"operator"`
}

type MCPGridItem struct {
	Label      string   `yaml:"label"`
	Code       string   `yaml:"code"`
	Highlights []string `yaml:"highlights"`
}

func GetMCPTests() (map[string]*MCPTestCase, error) {
	var tests map[string]*MCPTestCase
	if err := toml.Unmarshal(testsData, &tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func GetMCPDocSections() ([]MCPDocSection, error) {
	var sections []MCPDocSection
	if err := yaml.Unmarshal(sectionsData, &sections); err != nil {
		return nil, err
	}
	return sections, nil
}
