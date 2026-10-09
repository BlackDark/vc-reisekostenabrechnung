package export

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema.json
var schemaJSON []byte

// ValidateJSON checks abrechnung.json against the export schema.
func ValidateJSON(doc []byte) error {
	comp := jsonschema.NewCompiler()
	schema, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return err
	}
	url := "https://github.com/BlackDark/vc-reisekostenabrechnung/api/schemas/abrechnung-export-v1.schema.json"
	if err := comp.AddResource(url, schema); err != nil {
		return err
	}
	compiled, err := comp.Compile(url)
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return err
	}
	if err := compiled.Validate(inst); err != nil {
		return fmt.Errorf("abrechnung.json: %w", err)
	}
	return nil
}

// CheckFile validates one abrechnung.json on disk.
func CheckFile(path string) error {
	doc, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return ValidateJSON(doc)
}
