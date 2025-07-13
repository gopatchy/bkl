package bkl

import (
	_ "embed"
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

//go:embed tests.toml
var testsData []byte

//go:embed docs/index.yaml
var sectionsData []byte

//go:embed docs/k8s.yaml
var k8sData []byte

//go:embed docs/fixit.yaml
var fixitData []byte

type TestCase struct {
	Description string            `toml:"description" json:"description"`
	Eval        []string          `toml:"eval" json:"eval"`
	Format      string            `toml:"format" json:"format"`
	Expected    string            `toml:"expected,omitempty" json:"expected,omitempty"`
	Files       map[string]string `toml:"files" json:"files"`
	Errors      []string          `toml:"errors,omitempty" json:"errors,omitempty"`
	RootPath    string            `toml:"rootPath,omitempty" json:"rootPath,omitempty"`
	Env         map[string]string `toml:"env,omitempty" json:"env,omitempty"`
	Diff        bool              `toml:"diff,omitempty" json:"diff,omitempty"`
	Intersect   bool              `toml:"intersect,omitempty" json:"intersect,omitempty"`
	Required    bool              `toml:"required,omitempty" json:"required,omitempty"`
	Compare     bool              `toml:"compare,omitempty" json:"compare,omitempty"`
	Benchmark   bool              `toml:"benchmark,omitempty" json:"benchmark,omitempty"`
	Selector    string            `toml:"selector,omitempty" json:"selector,omitempty"`
	SortPath    string            `toml:"sortPath,omitempty" json:"sortPath,omitempty"`
}

type DocSection struct {
	ID     string    `yaml:"id" json:"id"`
	Title  string    `yaml:"title" json:"title"`
	Items  []DocItem `yaml:"items" json:"items"`
	Source string    `yaml:"-" json:"-"`
}

type DocItem struct {
	Content    string         `yaml:"content,omitempty" json:"content,omitempty"`
	Example    *DocExample    `yaml:"example,omitempty" json:"example,omitempty"`
	Code       *DocLayer      `yaml:"code,omitempty" json:"code,omitempty"`
	SideBySide *DocSideBySide `yaml:"side_by_side,omitempty" json:"side_by_side,omitempty"`
}

type DocSideBySide struct {
	Left  DocLayer `yaml:"left" json:"left"`
	Right DocLayer `yaml:"right" json:"right"`
}

type DocExample struct {
	Operation string     `yaml:"operation,omitempty" json:"operation,omitempty"`
	Layers    []DocLayer `yaml:"layers,omitempty" json:"layers,omitempty"`
	Result    DocLayer   `yaml:"result,omitempty" json:"result,omitempty"`
}

type DocLayer struct {
	Label      string   `yaml:"label,omitempty" json:"label,omitempty"`
	Code       string   `yaml:"code" json:"code"`
	Highlights []string `yaml:"highlights,omitempty" json:"highlights,omitempty"`
	Languages  [][]any  `yaml:"languages,omitempty" json:"languages,omitempty"`
	Expandable bool     `yaml:"expandable,omitempty" json:"expandable,omitempty"`
	Collapsed  bool     `yaml:"collapsed,omitempty" json:"collapsed,omitempty"`
}

func GetTests() (map[string]*TestCase, error) {
	var tests map[string]*TestCase
	if err := toml.Unmarshal(testsData, &tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func GetDocSections() ([]DocSection, error) {
	var allSections []DocSection

	files := map[string][]byte{
		"index": sectionsData,
		"k8s":   k8sData,
		"fixit": fixitData,
	}

	for name, data := range files {
		var sections []DocSection
		if err := yaml.Unmarshal(data, &sections); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s.yaml: %w", name, err)
		}
		for i := range sections {
			sections[i].Source = name
		}
		allSections = append(allSections, sections...)
	}

	return allSections, nil
}
