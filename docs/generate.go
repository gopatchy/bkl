package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	"github.com/gopatchy/bkl"
	"gopkg.in/yaml.v3"
)

type TemplateData struct {
	Sections []bkl.DocSection
}

func main() {
	data, err := os.ReadFile("sections.yaml")
	if err != nil {
		log.Fatal(err)
	}

	var sections []bkl.DocSection
	if err := yaml.Unmarshal(data, &sections); err != nil {
		log.Fatal(err)
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

	templateData := TemplateData{
		Sections: sections,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("index.html", buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Generated index.html")
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
