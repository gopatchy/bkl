package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	"github.com/gopatchy/bkl"
)

type TemplateData struct {
	Sections []bkl.DocSection
}

func main() {
	allSections, err := bkl.GetDocSections()
	if err != nil {
		log.Fatal(err)
	}

	sectionsBySource := make(map[string][]bkl.DocSection)
	for _, section := range allSections {
		sectionsBySource[section.Source] = append(sectionsBySource[section.Source], section)
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

	for source, sections := range sectionsBySource {
		templateData := TemplateData{
			Sections: sections,
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, templateData); err != nil {
			log.Printf("Error executing template for %s: %v", source, err)
			continue
		}

		outputFile := source + ".html"
		if err := os.WriteFile(outputFile, buf.Bytes(), 0o644); err != nil {
			log.Printf("Error writing %s: %v", outputFile, err)
			continue
		}

		fmt.Printf("Generated %s from embedded %s.yaml\n", outputFile, source)
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

func slice(values ...any) []any {
	return values
}
