package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/docopt/docopt-go"
	yaml "gopkg.in/yaml.v2"
)

const usageSwagger2Yaml = `api_tool swagger2yaml
	Swagger（OpenAPI 3.0）形式のAPI定義をYAMLに変換

Usage:
  api_tool swagger2yaml <INPUT_PATH> <OUTPUT_PATH>
  api_tool swagger2yaml -h | --help

Args:
	<INPUT_PATH>           (必須)入力Swaggerファイルパス（json/yaml）
	<OUTPUT_PATH>          (必須)出力YAMLファイルパス

Options:
	-h --help              Show this screen.
`

type Swagger2YamlArg struct {
	InputPath  string
	OutputPath string
}

func NewSwagger2YamlArg(arguments map[string]interface{}) Swagger2YamlArg {
	return Swagger2YamlArg{
		InputPath:  s(arguments["<INPUT_PATH>"]),
		OutputPath: s(arguments["<OUTPUT_PATH>"]),
	}
}

func RunSwagger2Yaml() {
	arguments, err := docopt.Parse(usageSwagger2Yaml, nil, true, "", false)
	if err != nil {
		panic(err)
	}

	arg := NewSwagger2YamlArg(arguments)

	spec := loadSwaggerSpec(arg.InputPath)

	groups := convertSwaggerToGroups(spec)

	output, err := yaml.Marshal(groups)
	e(err)

	os.MkdirAll(path.Dir(arg.OutputPath), os.ModePerm)
	err = os.WriteFile(arg.OutputPath, output, os.ModePerm)
	e(err)
	fmt.Println("write:", arg.OutputPath)
}

func loadSwaggerSpec(filePath string) OpenAPISpec {
	buf, err := os.ReadFile(filePath)
	e(err)

	var spec OpenAPISpec
	if strings.HasSuffix(filePath, ".json") {
		err = json.Unmarshal(buf, &spec)
	} else {
		err = yaml.Unmarshal(buf, &spec)
	}
	e(err)
	return spec
}

func convertSwaggerToGroups(spec OpenAPISpec) Groups {
	schemas := make(map[string]ComponentSchema)
	if spec.Components != nil {
		schemas = spec.Components.Schemas
	}

	// 1. schemas を分類: enum / type / response
	enumSchemas := make(map[string]ComponentSchema)   // integer or string enum
	typeSchemas := make(map[string]ComponentSchema)    // object (non-Response)
	responseSchemas := make(map[string]ComponentSchema) // *Response

	schemaNames := sortedKeys(schemas)
	for _, name := range schemaNames {
		cs := schemas[name]
		if len(cs.Enum) > 0 {
			enumSchemas[name] = cs
		} else if len(cs.AllOf) > 0 {
			// allOf schema (e.g. CustomerListEntry) -> type
			if !strings.HasSuffix(name, "Response") {
				typeSchemas[name] = cs
			} else {
				responseSchemas[name] = cs
			}
		} else if cs.Type == "object" {
			if strings.HasSuffix(name, "Response") {
				responseSchemas[name] = cs
			} else {
				typeSchemas[name] = cs
			}
		}
	}

	// 2. Enum変換
	enums := make([]*Enum, 0)
	for _, name := range sortedKeys(enumSchemas) {
		cs := enumSchemas[name]
		enum := convertSchemaToEnum(name, cs)
		enums = append(enums, enum)
	}

	// 3. Type変換
	types := TypeList(make([]*Type, 0))
	for _, name := range sortedKeys(typeSchemas) {
		cs := typeSchemas[name]
		t := convertSchemaToType(name, cs, schemas)
		types = append(types, t)
	}

	// 4. Action変換 (paths)
	actions := make([]*Action, 0)
	pathKeys := sortedKeys(spec.Paths)
	for _, pathStr := range pathKeys {
		pathItem := spec.Paths[pathStr]
		if pathItem.Post == nil {
			continue
		}
		op := pathItem.Post

		group := ""
		if len(op.Tags) > 0 {
			group = op.Tags[0]
		}

		action := NewAction()
		action.Name = op.Summary
		action.Description = op.Description
		action.Group = group

		// Auth判定
		if len(op.Security) == 0 {
			authFalse := false
			action.Auth = &authFalse
		}
		// len > 0 with cookieAuth → auth=nil (default)

		// Request properties
		for _, param := range op.Parameters {
			prop := NewProperty()
			prop.Name = param.Name
			prop.Description = param.Description
			prop.Type = schemaRefToPropertyType(param.Schema)
			action.RequestProperties = append(action.RequestProperties, prop)
		}

		// Response properties
		resp, ok := op.Responses["200"]
		if ok && resp.Content != nil {
			mt, ok := resp.Content["application/json"]
			if ok && mt.Schema.Ref != "" {
				refName := strings.TrimPrefix(mt.Schema.Ref, "#/components/schemas/")
				if rs, ok := schemas[refName]; ok {
					action.ResponseProperties = extractPropertiesFromSchema(rs, schemas)
				}
			}
		}

		actions = append(actions, action)
	}

	// 5. グループ割り当て
	groups := Groups(make([]*Group, 0))

	// タグ順でグループを事前作成
	for _, tag := range spec.Tags {
		groups.findOrCreate(tag.Name)
	}

	// Actionをグループに割り当て
	for _, action := range actions {
		groups.AddAction(action)
	}

	// Type/Enumのグループ割り当て: Actionから参照をたどる
	typeGroupMap := make(map[string]string)
	for _, action := range actions {
		refs := collectTypeRefs(action, schemas)
		for _, ref := range refs {
			if _, exists := typeGroupMap[ref]; !exists {
				typeGroupMap[ref] = action.Group
			}
		}
	}

	firstGroup := ""
	if len(spec.Tags) > 0 {
		firstGroup = spec.Tags[0].Name
	}

	for _, enum := range enums {
		if g, ok := typeGroupMap[enum.Name]; ok {
			enum.Group = g
		} else {
			enum.Group = firstGroup
		}
		groups.AddEnum(enum)
	}

	for _, t := range types {
		if g, ok := typeGroupMap[t.Name]; ok {
			t.Group = g
		} else {
			t.Group = firstGroup
		}
		groups.AddType(t)
	}

	return groups
}

func convertSchemaToEnum(name string, cs ComponentSchema) *Enum {
	enum := NewEnum()
	enum.Name = name

	if cs.Type == "string" {
		// 文字列Enum: enum値が文字列そのもの
		desc, _ := parseEnumDescriptionLines(cs.Description)
		enum.Description = desc
		for i, v := range cs.Enum {
			member := &EnumMember{
				Name:    fmt.Sprintf("%v", v),
				Ordinal: i,
			}
			enum.Members = append(enum.Members, member)
		}
	} else {
		// 整数Enum: descriptionから "- N: MemberName" をパース
		desc, members := parseIntEnumDescription(cs.Description)
		enum.Description = desc
		if len(members) > 0 {
			enum.Members = members
		} else {
			// descriptionにメンバー情報がない場合、enum値をそのまま使う
			for i, v := range cs.Enum {
				ordinal := i
				if n, ok := toInt(v); ok {
					ordinal = n
				}
				member := &EnumMember{
					Name:    fmt.Sprintf("value_%d", ordinal),
					Ordinal: ordinal,
				}
				enum.Members = append(enum.Members, member)
			}
		}
	}

	return enum
}

func parseIntEnumDescription(desc string) (string, []*EnumMember) {
	lines := strings.Split(desc, "\n")
	members := make([]*EnumMember, 0)
	descLines := make([]string, 0)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			body := strings.TrimPrefix(trimmed, "- ")
			parts := strings.SplitN(body, ": ", 2)
			if len(parts) == 2 {
				ordinal, err := strconv.Atoi(strings.TrimSpace(parts[0]))
				if err == nil {
					members = append(members, &EnumMember{
						Name:    strings.TrimSpace(parts[1]),
						Ordinal: ordinal,
					})
					continue
				}
			}
		}
		descLines = append(descLines, line)
	}

	baseDesc := strings.TrimSpace(strings.Join(descLines, "\n"))
	return baseDesc, members
}

func parseEnumDescriptionLines(desc string) (string, []string) {
	lines := strings.Split(desc, "\n")
	memberNames := make([]string, 0)
	descLines := make([]string, 0)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			memberNames = append(memberNames, strings.TrimPrefix(trimmed, "- "))
		} else if trimmed != "" {
			descLines = append(descLines, line)
		}
	}

	baseDesc := strings.TrimSpace(strings.Join(descLines, "\n"))
	return baseDesc, memberNames
}

func convertSchemaToType(name string, cs ComponentSchema, allSchemas map[string]ComponentSchema) *Type {
	t := NewType()
	t.Name = name
	t.Description = cs.Description

	if len(cs.AllOf) > 0 {
		// allOfの全パーツのプロパティをマージ
		for _, part := range cs.AllOf {
			if part.Ref != "" {
				refName := strings.TrimPrefix(part.Ref, "#/components/schemas/")
				if refSchema, ok := allSchemas[refName]; ok {
					props := extractPropertiesFromSchema(refSchema, allSchemas)
					t.Properties = append(t.Properties, props...)
				}
			}
			if part.Properties != nil {
				propNames := sortedKeys(part.Properties)
				for _, pName := range propNames {
					sp := part.Properties[pName]
					prop := schemaPropertyToProperty(pName, sp)
					t.Properties = append(t.Properties, prop)
				}
			}
		}
	} else {
		props := extractPropertiesFromSchema(cs, allSchemas)
		t.Properties = props
	}

	return t
}

func extractPropertiesFromSchema(cs ComponentSchema, allSchemas map[string]ComponentSchema) []*Property {
	props := make([]*Property, 0)

	if len(cs.AllOf) > 0 {
		for _, part := range cs.AllOf {
			if part.Ref != "" {
				refName := strings.TrimPrefix(part.Ref, "#/components/schemas/")
				if refSchema, ok := allSchemas[refName]; ok {
					props = append(props, extractPropertiesFromSchema(refSchema, allSchemas)...)
				}
			}
			if part.Properties != nil {
				propNames := sortedKeys(part.Properties)
				for _, pName := range propNames {
					sp := part.Properties[pName]
					prop := schemaPropertyToProperty(pName, sp)
					props = append(props, prop)
				}
			}
		}
		return props
	}

	if cs.Properties == nil {
		return props
	}

	propNames := sortedKeys(cs.Properties)
	for _, pName := range propNames {
		sp := cs.Properties[pName]
		prop := schemaPropertyToProperty(pName, sp)
		props = append(props, prop)
	}
	return props
}

func schemaPropertyToProperty(name string, sp SchemaProperty) *Property {
	prop := NewProperty()
	prop.Name = name
	prop.Description = sp.Description

	if sp.Ref != "" {
		prop.Type = PropertyType(strings.TrimPrefix(sp.Ref, "#/components/schemas/"))
	} else if len(sp.AllOf) > 0 {
		// allOf内の$refから型を取得
		for _, item := range sp.AllOf {
			if item.Ref != "" {
				prop.Type = PropertyType(strings.TrimPrefix(item.Ref, "#/components/schemas/"))
				break
			}
		}
	} else {
		prop.Type = schemaRefToPropertyType(SchemaRef{
			Type:   sp.Type,
			Format: sp.Format,
			Items:  sp.Items,
		})
	}
	return prop
}

func schemaRefToPropertyType(sr SchemaRef) PropertyType {
	if sr.Ref != "" {
		name := strings.TrimPrefix(sr.Ref, "#/components/schemas/")
		return PropertyType(name)
	}

	if len(sr.AllOf) > 0 {
		for _, item := range sr.AllOf {
			if item.Ref != "" {
				return PropertyType(strings.TrimPrefix(item.Ref, "#/components/schemas/"))
			}
		}
	}

	if sr.Type == "array" && sr.Items != nil {
		itemType := schemaRefToPropertyType(*sr.Items)
		return PropertyType(string(itemType) + "[]")
	}

	switch sr.Type {
	case "string":
		if sr.Format == "byte" {
			return PropertyType("binary")
		}
		return PropertyType("text")
	case "integer":
		if sr.Format == "int64" {
			return PropertyType("int64")
		}
		return PropertyType("int")
	case "boolean":
		return PropertyType("bool")
	case "number":
		if sr.Format == "double" {
			return PropertyType("double")
		}
		return PropertyType("float")
	default:
		return PropertyType("text")
	}
}

// Actionのリクエスト/レスポンスから参照される型名を再帰的に収集
func collectTypeRefs(action *Action, schemas map[string]ComponentSchema) []string {
	seen := make(map[string]bool)
	var collect func(typeName string)
	collect = func(typeName string) {
		if seen[typeName] {
			return
		}
		seen[typeName] = true
		if cs, ok := schemas[typeName]; ok {
			if cs.Properties != nil {
				for _, sp := range cs.Properties {
					if sp.Ref != "" {
						ref := strings.TrimPrefix(sp.Ref, "#/components/schemas/")
						collect(ref)
					}
					for _, a := range sp.AllOf {
						if a.Ref != "" {
							collect(strings.TrimPrefix(a.Ref, "#/components/schemas/"))
						}
					}
					if sp.Items != nil && sp.Items.Ref != "" {
						collect(strings.TrimPrefix(sp.Items.Ref, "#/components/schemas/"))
					}
				}
			}
			for _, part := range cs.AllOf {
				if part.Ref != "" {
					collect(strings.TrimPrefix(part.Ref, "#/components/schemas/"))
				}
			}
		}
	}

	for _, p := range action.RequestProperties {
		typeName := string(p.Type)
		typeName = strings.TrimSuffix(typeName, "[]")
		if _, ok := schemas[typeName]; ok {
			collect(typeName)
		}
	}
	for _, p := range action.ResponseProperties {
		typeName := string(p.Type)
		typeName = strings.TrimSuffix(typeName, "[]")
		if _, ok := schemas[typeName]; ok {
			collect(typeName)
		}
	}

	// Response schema自体からも参照を収集
	responseName := action.Name + "Response"
	if cs, ok := schemas[responseName]; ok {
		if cs.Properties != nil {
			for _, sp := range cs.Properties {
				if sp.Ref != "" {
					collect(strings.TrimPrefix(sp.Ref, "#/components/schemas/"))
				}
				for _, a := range sp.AllOf {
					if a.Ref != "" {
						collect(strings.TrimPrefix(a.Ref, "#/components/schemas/"))
					}
				}
				if sp.Items != nil && sp.Items.Ref != "" {
					collect(strings.TrimPrefix(sp.Items.Ref, "#/components/schemas/"))
				}
			}
		}
	}

	result := make([]string, 0, len(seen))
	for name := range seen {
		// Responseスキーマは除外
		if !strings.HasSuffix(name, "Response") {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case int64:
		return int(n), true
	}
	return 0, false
}

// mapのキーをソートして返す汎用ヘルパー
func sortedKeys[M ~map[K]V, K ~string, V any](m M) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
