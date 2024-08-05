/*
Copyright The Kubepack Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"kubepack.dev/chart-doc-gen/api"
	"kubepack.dev/chart-doc-gen/templates"
	"kubepack.dev/chart-doc-gen/values"

	"github.com/olekukonko/tablewriter"
	flag "github.com/spf13/pflag"
	ylib "k8s.io/apimachinery/pkg/util/yaml"
	yaml2 "sigs.k8s.io/yaml"
)

var (
	docFile    = flag.StringP("doc", "d", "", "Path to a project's doc.{json|yaml} info file")
	chartFile  = flag.StringP("chart", "c", "", "Path to Chart.yaml file")
	valuesFile = flag.StringP("values", "v", "", "Path to chart values file")
	schemaFile = flag.StringP("schema", "s", "", "Path to values JSON Schema file")
	tplFile    = flag.StringP("template", "t", "readme2.tpl", "Path to a doc template file")
)

func main() {
	flag.Parse()

	f, err := os.Open(*docFile)
	if err != nil {
		panic(err)
	}
	reader := ylib.NewYAMLOrJSONDecoder(f, 2048)
	var doc api.DocInfo
	err = reader.Decode(&doc)
	if err != nil && err != io.EOF {
		panic(err)
	}

	data, err := os.ReadFile(*valuesFile)
	if err != nil {
		panic(err)
	}

	parameters, err := values.LoadFromValuesFile(data)
	if err != nil && errors.Is(err, io.EOF) {
		parameters = values.List{}
	} else if err != nil {
		panic(err)
	}

	if *schemaFile != "" {
		jsonSchemaData, err := os.ReadFile(*schemaFile)
		if err != nil {
			panic(err)
		}

		schemaParameters, err := values.LoadFromJSONShema(jsonSchemaData)
		if err != nil && errors.Is(err, io.EOF) {
			parameters = values.List{}
		} else if err != nil {
			panic(err)
		}

		parameters = schemaParameters.Merge(parameters)
	}

	var params [][]string
	for _, parameter := range parameters {
		params = append(params, []string{
			parameter.Name,
			parameter.Description,
			fmt.Sprintf(
				"<code>%s</code>", // use a html code block instead of backtics so the whole block get highlighted
				strings.ReplaceAll( // replace all newlines, they generate new table columns with tablewriter
					strings.ReplaceAll(parameter.Default, "|", "&#124;"), // replace all pipe symbols with their ACSII representation, because they break the markdown table
					"\n",
					"&#13;&#10;",
				),
			),
		})
	}

	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"Parameter", "Description", "Default"})
	table.SetAutoFormatHeaders(false)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetAutoWrapText(false)
	table.SetCenterSeparator("|")
	table.AppendBulk(params) // Add Bulk Data
	table.Render()

	doc.Chart.Values = buf.String()

	if doc.Chart.ValuesExample == "" || strings.HasPrefix(doc.Chart.ValuesExample, "-- generate from values file --") {
		for _, parameter := range parameters {
			if !slices.Contains([]string{"", `""`, "{}", "[]", "true", "false", "not-ca-cert"}, parameter.Default) {
				doc.Chart.ValuesExample = fmt.Sprintf("%v=%v", parameter.Name, parameter.Default)
				break
			}
		}
	}

	{
		if *chartFile == "" {
			*chartFile = filepath.Join(filepath.Dir(*valuesFile), "Chart.yaml")
		}
		data, err := os.ReadFile(*chartFile)
		if err != nil {
			panic(err)
		}
		var ci api.ChartInfo
		if err = yaml2.Unmarshal(data, &ci); err != nil {
			panic(err)
		}
		doc.Chart.Name = ci.Name
		doc.Chart.Version = ci.Version
	}

	tplReadme, err := os.ReadFile(*tplFile)
	if err != nil {
		if os.IsNotExist(err) {
			tplReadme, err = iofs.ReadFile(templates.FS(), "readme.tpl")
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}

	tmpl, err := template.New("readme").Parse(string(tplReadme))
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(os.Stdout, doc)
	if err != nil {
		panic(err)
	}
}
