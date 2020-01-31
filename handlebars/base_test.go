package handlebars

import (
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"testing"

	"github.com/imantung/mario"
)

// cf. https://github.com/aymerick/go-fuzz-tests/mario
const dumpTpl = false

var dumpTplNb = 0

type Test struct {
	name     string
	input    string
	data     interface{}
	privData map[string]interface{}
	helpers  map[string]interface{}
	partials map[string]string
	output   interface{}
}

func launchTests(t *testing.T, tests []Test) {
	t.Parallel()

	for _, test := range tests {
		var err error
		var tpl *mario.Template

		if dumpTpl {
			filename := strconv.Itoa(dumpTplNb)
			if err := ioutil.WriteFile(path.Join(".", "dump_tpl", filename), []byte(test.input), 0644); err != nil {
				panic(err)
			}
			dumpTplNb++
		}

		// parse template
		tpl, err = mario.Parse(test.input)
		if err != nil {
			t.Errorf("Test '%s' failed - Failed to parse template\ninput:\n\t'%s'\nerror:\n\t%s", test.name, test.input, err)
		} else {
			if len(test.helpers) > 0 {
				// register helpers
				tpl.RegisterHelpers(test.helpers)
			}

			if len(test.partials) > 0 {
				// register partials
				tpl.RegisterPartials(test.partials)
			}

			// setup private data frame
			var privData *mario.DataFrame
			if test.privData != nil {
				privData = mario.NewDataFrame()
				for k, v := range test.privData {
					privData.Set(k, v)
				}
			}

			// render template
			output, err := tpl.ExecWith(test.data, privData)
			if err != nil {
				t.Errorf("Test '%s' failed\ninput:\n\t'%s'\ndata:\n\t%s\nerror:\n\t%s\nAST:\n\t%s", test.name, test.input, mario.Str(test.data), err, tpl.PrintAST())
			} else {
				// check output
				var expectedArr []string
				expectedArr, ok := test.output.([]string)
				if ok {
					match := false
					for _, expectedStr := range expectedArr {
						if expectedStr == output {
							match = true
							break
						}
					}

					if !match {
						t.Errorf("Test '%s' failed\ninput:\n\t'%s'\ndata:\n\t%s\npartials:\n\t%s\nexpected\n\t%q\ngot\n\t%q\nAST:\n%s", test.name, test.input, mario.Str(test.data), mario.Str(test.partials), expectedArr, output, tpl.PrintAST())
					}
				} else {
					expectedStr, ok := test.output.(string)
					if !ok {
						panic(fmt.Errorf("Erroneous test output description: %q", test.output))
					}

					if expectedStr != output {
						t.Errorf("Test '%s' failed\ninput:\n\t'%s'\ndata:\n\t%s\npartials:\n\t%s\nexpected\n\t%q\ngot\n\t%q\nAST:\n%s", test.name, test.input, mario.Str(test.data), mario.Str(test.partials), expectedStr, output, tpl.PrintAST())
					}
				}
			}
		}
	}
}
