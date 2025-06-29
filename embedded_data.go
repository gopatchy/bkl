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

type TestCase struct {
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

type DocSection struct {
	ID    string    `yaml:"id"`
	Title string    `yaml:"title"`
	Items []DocItem `yaml:"items"`
}

type DocItem struct {
	Content    string         `yaml:"content,omitempty"`
	Example    *DocExample    `yaml:"example,omitempty"`
	Code       *DocLayer      `yaml:"code,omitempty"`         // For simple code examples
	SideBySide *DocSideBySide `yaml:"side_by_side,omitempty"` // For special two-column layout
}

type DocSideBySide struct {
	Left  DocLayer `yaml:"left"`
	Right DocLayer `yaml:"right"`
}

type DocExample struct {
	Operation string     `yaml:"operation,omitempty"` // "evaluate", "diff", "intersect", "required"
	Layers    []DocLayer `yaml:"layers,omitempty"`
	Result    DocLayer   `yaml:"result,omitempty"`
}

type DocLayer struct {
	Label      string   `yaml:"label,omitempty"`
	Code       string   `yaml:"code"`
	Highlights []string `yaml:"highlights,omitempty"`
	Languages  [][]any  `yaml:"languages,omitempty"` // List of [line, language] pairs
}

func GetTests() (map[string]*TestCase, error) {
	var tests map[string]*TestCase
	if err := toml.Unmarshal(testsData, &tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func GetDocSections() ([]DocSection, error) {
	var sections []DocSection
	if err := yaml.Unmarshal(sectionsData, &sections); err != nil {
		return nil, err
	}
	return sections, nil
}
