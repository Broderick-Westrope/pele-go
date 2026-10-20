package pele_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	pele "github.com/Broderick-Westrope/pele-go"
	_ "github.com/Broderick-Westrope/pele-go/all"
)

func ExampleRender() {
	config, err := pele.ParseConfig(`{"flowchart": {"curve": "linear"}}`)
	if err != nil {
		log.Fatal(err)
	}
	res, err := pele.Render(context.Background(), "graph LR\n  A[Mermaid text] --> B[SVG]", pele.Options{
		FontSize:   pele.Ptr(14.0),
		Responsive: pele.Ptr(false),
		Config:     config,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Type, res.MermaidType)
	// Output: flowchart flowchart-v2
}

func ExampleError() {
	_, err := pele.Render(context.Background(), "just some text", pele.Options{})
	var pe *pele.Error
	if errors.As(err, &pe) {
		fmt.Printf("%s: %s\n", pe.Code, pe.Message)
	}
	// Output: unsupported-diagram: No diagram type detected for the given text.
}
