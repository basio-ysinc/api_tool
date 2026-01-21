package main

import (
	"strings"
)

type Type struct {
	Modifier string `yaml:"modifier,omitempty"`
	Name     string

	Description string `yaml:"description,omitempty"`
	Group       string `yaml:"-"`
	Properties  []*Property

	allProperties []*Property

	Comments map[int][]string `yaml:"comments,omitempty"`
}

func (this *Type) AllProperties() []*Property {
	if this.allProperties != nil {
		return this.allProperties
	}
	res := make([]*Property, 0)
	res = append(res, this.Properties...)

	this.allProperties = res
	return res
}

func (this *Type) FirstProperty() *Property {
	list := this.AllProperties()
	return list[0]
}

func NewType() *Type {
	return &Type{
		Properties: make([]*Property, 0),
	}
}

type TypeList []*Type

func (this TypeList) Get(name string) *Type {
	for _, t := range this {
		if t.Name == name {
			return t
		}
	}
	return nil
}

type Action struct {
	Name        string
	Description string `yaml:"description,omitempty"`
	Group       string `yaml:"-"`
	Auth        *bool  `yaml:"auth,omitempty"` // nil or true = 認証必要, false = 認証不要

	RequestProperties  []*Property `yaml:"requestProperties"`
	ResponseProperties []*Property `yaml:"responseProperties"`

	allRequestProperties  []*Property
	allResponseProperties []*Property

	Comments map[int][]string `yaml:"comments,omitempty"`
}

// BaseTypeをすべて解決したPropertyリスト取得
func (this *Action) AllRequestProperties() []*Property {
	//fmt.Println("AllRequestProperties")
	if this.allRequestProperties != nil {
		//dump(this)
		return this.allRequestProperties
	}
	res := make([]*Property, 0)
	res = append(res, this.RequestProperties...)

	this.allRequestProperties = res

	//dump(this)
	return res
}

// BaseTypeをすべて解決したPropertyリスト取得
func (this *Action) AllResponseProperties() []*Property {
	if this.allResponseProperties != nil {
		return this.allResponseProperties
	}
	res := make([]*Property, 0)
	res = append(res, this.ResponseProperties...)

	this.allResponseProperties = res
	return res
}

func NewAction() *Action {
	return &Action{
		RequestProperties:  make([]*Property, 0),
		ResponseProperties: make([]*Property, 0),
	}
}

type Property struct {
	Name        string
	Type        PropertyType
	Description string `yaml:"description,omitempty"`
}

func NewProperty() *Property {
	return &Property{}
}

type PropertyType string

type Enum struct {
	Modifier string `yaml:"modifier,omitempty"`
	Name     string

	Description string `yaml:"description,omitempty"`
	Group       string `yaml:"-"`

	Members []*EnumMember
}

func NewEnum() *Enum {
	return &Enum{
		Members: make([]*EnumMember, 0),
	}
}

type EnumMember struct {
	Name        string
	Ordinal     int
	DisplayName string   `yaml:"displayName,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Comments    []string `yaml:"comments,omitempty"`
}

type Group struct {
	Name string

	Actions []*Action
	Types   TypeList
	Enums   []*Enum
}

func NewGroup(name string) *Group {
	return &Group{
		Name:    name,
		Actions: make([]*Action, 0),
		Types:   TypeList(make([]*Type, 0)),
		Enums:   make([]*Enum, 0),
	}
}

type Groups []*Group

func (this *Groups) AddEnum(e *Enum) {
	g := this.findOrCreate(e.Group)
	g.Enums = append(g.Enums, e)
}
func (this *Groups) AddType(t *Type) {
	g := this.findOrCreate(t.Group)
	g.Types = append(g.Types, t)
}
func (this *Groups) AddAction(a *Action) {
	g := this.findOrCreate(a.Group)
	g.Actions = append(g.Actions, a)
}
func (this *Groups) findOrCreate(name string) *Group {
	for _, g := range *this {
		if g.Name == name {
			return g
		}
	}

	res := NewGroup(name)
	*this = append(*this, res)
	//fmt.Println("append", name, len(*this))
	return res
}
func GroupNameFromSheetName(sheetName string) string {
	//tmp := strings.SplitN(sheetName, "_", 2)
	//if len(tmp) < 2{
	//	return ""
	//}
	return strings.SplitN(sheetName, "_", 2)[1]
}
