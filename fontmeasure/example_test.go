package fontmeasure_test

import (
	"fmt"
	"log"
	"os"

	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/fontmeasure"
)

func ExampleNew() {
	var faces [4][]byte
	for i, name := range []string{"DejaVuSans.ttf", "DejaVuSans-Bold.ttf", "DejaVuSans-Oblique.ttf", "DejaVuSans-BoldOblique.ttf"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			log.Fatal(err)
		}
		faces[i] = data
	}
	m, err := fontmeasure.New(faces[0], faces[1], faces[2], faces[3])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(m.Width("Hello, World", 16, 0), m.Width("Hello, World", 16, pele.Bold))
	// Output: 96.578125 110.75
}
