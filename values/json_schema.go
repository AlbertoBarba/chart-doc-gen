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

package values

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func LoadFromJSONShema(content []byte) (List, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("values.schema.json", bytes.NewBuffer(content)); err != nil {
		return nil, err
	}

	schema, err := compiler.Compile("values.schema.json")
	if err != nil {
		return nil, err
	}

	return GenerateFromJSONSchema(schema)
}

func GenerateFromJSONSchema(schema *jsonschema.Schema) (List, error) {
	parameters := List{}
	parseSchema(schema, "", parameters)

	return parameters, nil
}

func parseSchema(schema *jsonschema.Schema, prefix string, parameters List) {
	if len(schema.Properties) == 0 && schema.Ref == nil {
		parameters[prefix] = Parameter{
			Name:        prefix,
			Description: schema.Description,
			Default:     fmt.Sprintf("`%v`", schema.Default),
		}

		return
	}

	if schema.Ref != nil {
		parseSchema(schema.Ref, prefix, parameters)
	}

	for propertyName, propertySchema := range schema.Properties {
		key := propertyName
		if prefix != "" {
			key = strings.Join([]string{prefix, propertyName}, ".")
		}

		parseSchema(propertySchema, key, parameters)
	}
}
