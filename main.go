package main

import (
	"fmt"

	"os"

	"github.com/docopt/docopt-go"
)

const usageRoot = `api_tool
    API定義ツール群

Usage:
    api_tool COMMAND
    api_tool -h | --help

Arg:
    "xlsx2yaml"      API定義の変換 xlsx -> yaml
    "yaml2xlsx"      API定義の変換 yaml -> xlsx
    "yaml2swagger"   API定義の変換 yaml -> swagger (OpenAPI 3.0)
    "gen-single"     API定義からテキスト生成 pongo2
    "gen-multiple"   API定義から複数のテキスト生成 pongo2
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usageRoot)
		return
	}
	arguments, err := docopt.Parse(usageRoot, os.Args[1:2], true, "", false)

	e(err)

	switch arguments["COMMAND"] {
	default:
		fmt.Println(usageRoot)
	case "gen-single":
		RunGenSingle()
	case "gen-multiple":
		RunGenMultiple()
	case "xlsx2yaml":
		RunXlsx2Yaml()
	case "yaml2xlsx":
		RunYaml2Xlsx()
	case "yaml2swagger":
		RunYaml2Swagger()
	}
}
