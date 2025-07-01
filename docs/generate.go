package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopatchy/bkl"
	"gopkg.in/yaml.v3"
)

type TemplateData struct {
	Sections []bkl.DocSection
}

func main() {
	yamlFiles, err := filepath.Glob("*.yaml")
	if err != nil {
		log.Fatal(err)
	}

	if len(yamlFiles) == 0 {
		log.Fatal("No YAML files found in the current directory")
	}

	templateContent, err := os.ReadFile("template.html")
	if err != nil {
		log.Fatal(err)
	}

	tmpl := template.Must(template.New("html").Funcs(template.FuncMap{
		"formatContent": formatContent,
		"formatLayer":   formatLayer,
		"dict":          dict,
	}).Parse(string(templateContent)))

	for _, yamlFile := range yamlFiles {
		data, err := os.ReadFile(yamlFile)
		if err != nil {
			log.Printf("Error reading %s: %v", yamlFile, err)
			continue
		}

		var sections []bkl.DocSection
		if err := yaml.Unmarshal(data, &sections); err != nil {
			log.Printf("Error unmarshaling %s: %v", yamlFile, err)
			continue
		}

		templateData := TemplateData{
			Sections: sections,
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, templateData); err != nil {
			log.Printf("Error executing template for %s: %v", yamlFile, err)
			continue
		}

		outputFile := strings.TrimSuffix(yamlFile, filepath.Ext(yamlFile)) + ".html"
		if err := os.WriteFile(outputFile, buf.Bytes(), 0o644); err != nil {
			log.Printf("Error writing %s: %v", outputFile, err)
			continue
		}

		fmt.Printf("Generated %s from %s\n", outputFile, yamlFile)
	}
}

func formatContent(content string) template.HTML {
	return template.HTML(content)
}

func formatLayer(layer bkl.DocLayer) template.HTML {
	code := strings.TrimSpace(layer.Code)

	// Apply syntax highlighting with highlights
	result := applySyntaxHighlighting(code, layer.Languages, layer.Highlights)

	return template.HTML(result)
}

// dict creates a map from pairs of arguments (key1, value1, key2, value2, ...)
func dict(values ...any) map[string]any {
	if len(values)%2 != 0 {
		panic("dict requires an even number of arguments")
	}
	m := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			panic(fmt.Sprintf("dict keys must be strings, got %T", values[i]))
		}
		m[key] = values[i+1]
	}
	return m
}
