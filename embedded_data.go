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
	ID    string        `yaml:"id"`
	Title string        `yaml:"title"`
	Items []ContentItem `yaml:"items"`
}

type ContentItem struct {
	Type    string  `yaml:"type"`
	Content string  `yaml:"content"`
	Example Example `yaml:"example"`
}

type Example struct {
	Type       string    `yaml:"type"`
	Label      string    `yaml:"label"`
	Code       string    `yaml:"code"`
	Rows       []GridRow `yaml:"rows"`
	Highlights []string  `yaml:"highlights"`
}

type GridRow struct {
	Items    []GridItem `yaml:"items"`
	Operator string     `yaml:"operator"`
}

type GridItem struct {
	Label      string   `yaml:"label"`
	Code       string   `yaml:"code"`
	Highlights []string `yaml:"highlights"`
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
