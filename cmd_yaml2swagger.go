package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/docopt/docopt-go"
	yaml "gopkg.in/yaml.v2"
)

const usageYaml2Swagger = `api_tool yaml2swagger
	API定義をSwagger（OpenAPI 3.0）形式に変換
	リクエストは全てPOST、パラメーターは全てクエリストリング

Usage:
  api_tool yaml2swagger [--only=<OUTPUT_GROUPS>] [--format=<FORMAT>] [--base-path=<BASE_PATH>] [--title=<TITLE>] [--version=<VERSION>] [--trailing-slash] <OUTPUT_PATH> INPUTS...
  api_tool yaml2swagger -h | --help

Args:
	<OUTPUT_PATH>          (必須)出力ファイルパス
	INPUTS...              入力ファイルパス（yaml/xlsx）

Options:
	--only=<OUTPUT_GROUPS>   出力するグループ名をカンマ区切りで複数指定
	--format=<FORMAT>        出力形式 json/yaml [default:json]
	--base-path=<BASE_PATH>  APIのベースパス [default:/api]
	--title=<TITLE>          API タイトル [default:API]
	--version=<VERSION>      APIバージョン [default:1.0.0]
	--trailing-slash         URLの末尾に/を追加
	-h --help                Show this screen.
`

type Yaml2SwaggerArg struct {
	Inputs        []string
	OutputGroups  []string
	OutputPath    string
	Format        string
	BasePath      string
	Title         string
	Version       string
	TrailingSlash bool
}

func NewYaml2SwaggerArg(arguments map[string]interface{}) Yaml2SwaggerArg {
	format := s(arguments["--format"])
	if format == "" {
		format = "json"
	}
	basePath := s(arguments["--base-path"])
	if basePath == "" {
		basePath = "/api"
	}
	title := s(arguments["--title"])
	if title == "" {
		title = "API"
	}
	version := s(arguments["--version"])
	if version == "" {
		version = "1.0.0"
	}

	return Yaml2SwaggerArg{
		Inputs:        sl(arguments["INPUTS"]),
		OutputGroups:  slc(arguments["--only"]),
		OutputPath:    s(arguments["<OUTPUT_PATH>"]),
		Format:        format,
		BasePath:      basePath,
		Title:         title,
		Version:       version,
		TrailingSlash: b(arguments["--trailing-slash"]),
	}
}

// OpenAPI 3.0 structures
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi" yaml:"openapi"`
	Info       OpenAPIInfo            `json:"info" yaml:"info"`
	Servers    []OpenAPIServer        `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths      map[string]PathItem    `json:"paths" yaml:"paths"`
	Components *OpenAPIComponents     `json:"components,omitempty" yaml:"components,omitempty"`
	Tags       []OpenAPITag           `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type OpenAPIInfo struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string `json:"version" yaml:"version"`
}

type OpenAPIServer struct {
	URL         string `json:"url" yaml:"url"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type OpenAPITag struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type PathItem struct {
	Post *Operation `json:"post,omitempty" yaml:"post,omitempty"`
}

type Operation struct {
	Tags        []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
	Summary     string              `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Responses   map[string]Response `json:"responses" yaml:"responses"`
}

type Parameter struct {
	Name        string      `json:"name" yaml:"name"`
	In          string      `json:"in" yaml:"in"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool        `json:"required,omitempty" yaml:"required,omitempty"`
	Schema      SchemaRef   `json:"schema" yaml:"schema"`
}

type SchemaRef struct {
	Type   string     `json:"type,omitempty" yaml:"type,omitempty"`
	Format string     `json:"format,omitempty" yaml:"format,omitempty"`
	Items  *SchemaRef `json:"items,omitempty" yaml:"items,omitempty"`
	Ref    string     `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

type Response struct {
	Description string                `json:"description" yaml:"description"`
	Content     map[string]MediaType  `json:"content,omitempty" yaml:"content,omitempty"`
}

type MediaType struct {
	Schema SchemaRef `json:"schema" yaml:"schema"`
}

type OpenAPIComponents struct {
	Schemas map[string]ComponentSchema `json:"schemas,omitempty" yaml:"schemas,omitempty"`
}

type ComponentSchema struct {
	Type        string                    `json:"type" yaml:"type"`
	Description string                    `json:"description,omitempty" yaml:"description,omitempty"`
	Properties  map[string]SchemaProperty `json:"properties,omitempty" yaml:"properties,omitempty"`
	Enum        []interface{}             `json:"enum,omitempty" yaml:"enum,omitempty"`
}

type SchemaProperty struct {
	Type        string     `json:"type,omitempty" yaml:"type,omitempty"`
	Format      string     `json:"format,omitempty" yaml:"format,omitempty"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	Items       *SchemaRef `json:"items,omitempty" yaml:"items,omitempty"`
	Ref         string     `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

func RunYaml2Swagger() {
	arguments, err := docopt.Parse(usageYaml2Swagger, nil, true, "", false)
	if err != nil {
		panic(err)
	}

	arg := NewYaml2SwaggerArg(arguments)

	enums, types, actions, groups := load(arg.Inputs, arg.OutputGroups)

	spec := buildOpenAPISpec(arg, enums, types, actions, groups)

	var output []byte
	if arg.Format == "yaml" {
		output, err = yaml.Marshal(spec)
	} else {
		output, err = json.MarshalIndent(spec, "", "  ")
	}
	e(err)

	os.MkdirAll(path.Dir(arg.OutputPath), os.ModePerm)
	err = os.WriteFile(arg.OutputPath, output, os.ModePerm)
	e(err)
	fmt.Println("write:", arg.OutputPath)
}

func buildOpenAPISpec(arg Yaml2SwaggerArg, enums []*Enum, types []*Type, actions []*Action, groups []*Group) OpenAPISpec {
	spec := OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:   arg.Title,
			Version: arg.Version,
		},
		Paths: make(map[string]PathItem),
	}

	// Add tags from groups
	for _, g := range groups {
		spec.Tags = append(spec.Tags, OpenAPITag{
			Name: g.Name,
		})
	}

	// Build components (schemas)
	components := &OpenAPIComponents{
		Schemas: make(map[string]ComponentSchema),
	}

	// Add enums to schemas
	for _, enum := range enums {
		enumValues := make([]interface{}, len(enum.Members))
		for i, m := range enum.Members {
			enumValues[i] = m.Ordinal
		}
		components.Schemas[enum.Name] = ComponentSchema{
			Type:        "integer",
			Description: enum.Description,
			Enum:        enumValues,
		}
	}

	// Add types to schemas
	for _, t := range types {
		props := make(map[string]SchemaProperty)
		for _, p := range t.Properties {
			props[p.Name] = propertyToSchemaProperty(p)
		}
		components.Schemas[t.Name] = ComponentSchema{
			Type:        "object",
			Description: t.Description,
			Properties:  props,
		}
	}

	if len(components.Schemas) > 0 {
		spec.Components = components
	}

	// Build paths from actions
	for _, action := range actions {
		pathStr := arg.BasePath + "/" + action.Group + "/" + action.Name
		if arg.TrailingSlash {
			pathStr += "/"
		}

		// Convert request properties to query parameters
		params := make([]Parameter, 0)
		for _, p := range action.RequestProperties {
			params = append(params, Parameter{
				Name:        p.Name,
				In:          "query",
				Description: p.Description,
				Required:    false,
				Schema:      propertyTypeToSchemaRef(p.Type),
			})
		}

		// Build response schema
		responseProps := make(map[string]SchemaProperty)
		for _, p := range action.ResponseProperties {
			responseProps[p.Name] = propertyToSchemaProperty(p)
		}

		responses := map[string]Response{
			"200": {
				Description: "成功",
				Content: map[string]MediaType{
					"application/json": {
						Schema: SchemaRef{
							Type: "object",
						},
					},
				},
			},
		}

		// If there are response properties, create inline schema
		if len(responseProps) > 0 {
			responseName := action.Name + "Response"
			components.Schemas[responseName] = ComponentSchema{
				Type:       "object",
				Properties: responseProps,
			}
			responses["200"].Content["application/json"] = MediaType{
				Schema: SchemaRef{
					Ref: "#/components/schemas/" + responseName,
				},
			}
		}

		operation := &Operation{
			Tags:        []string{action.Group},
			Summary:     action.Name,
			Description: action.Description,
			OperationID: toOperationID(action.Group, action.Name),
			Parameters:  params,
			Responses:   responses,
		}

		spec.Paths[pathStr] = PathItem{
			Post: operation,
		}
	}

	return spec
}

func propertyToSchemaProperty(p *Property) SchemaProperty {
	schemaRef := propertyTypeToSchemaRef(p.Type)
	return SchemaProperty{
		Type:        schemaRef.Type,
		Format:      schemaRef.Format,
		Description: p.Description,
		Items:       schemaRef.Items,
		Ref:         schemaRef.Ref,
	}
}

func propertyTypeToSchemaRef(pt PropertyType) SchemaRef {
	if pt.IsArray() {
		itemType := pt.GetArrayItemType()
		itemRef := propertyTypeToSchemaRef(itemType)
		return SchemaRef{
			Type:  "array",
			Items: &itemRef,
		}
	}

	typeStr := string(pt)
	switch typeStr {
	case "null", "nil":
		return SchemaRef{Type: "string"}
	case "integer", "int", "int8", "int16", "int32", "sint32":
		return SchemaRef{Type: "integer", Format: "int32"}
	case "int64", "long", "sint64", "timestamp":
		return SchemaRef{Type: "integer", Format: "int64"}
	case "uint8", "uint16", "uint32":
		return SchemaRef{Type: "integer", Format: "int32"}
	case "uint64":
		return SchemaRef{Type: "integer", Format: "int64"}
	case "text", "string":
		return SchemaRef{Type: "string"}
	case "binary":
		return SchemaRef{Type: "string", Format: "byte"}
	case "boolean", "bool":
		return SchemaRef{Type: "boolean"}
	case "number", "float32", "float":
		return SchemaRef{Type: "number", Format: "float"}
	case "float64", "double":
		return SchemaRef{Type: "number", Format: "double"}
	default:
		// Check if it's a defined type or enum
		if _, ok := typeMap[typeStr]; ok {
			return SchemaRef{Ref: "#/components/schemas/" + typeStr}
		}
		if _, ok := enumMap[typeStr]; ok {
			return SchemaRef{Ref: "#/components/schemas/" + typeStr}
		}
		// Unknown type, treat as string
		return SchemaRef{Type: "string"}
	}
}

func toOperationID(group, name string) string {
	// Convert to camelCase operationId
	return strings.ToLower(group[:1]) + group[1:] + "_" + name
}
