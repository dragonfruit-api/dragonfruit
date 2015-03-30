package dragonfruit

import (
	"strings"
)

const (
	ContainerName           = "container"
	SwaggerResourceDB       = "swagger_docs"
	ResourceRootName        = "resource-root"
	ResourceDescriptionName = "swagger_resource"
	ResourceStem            = "resource_"
)

// A configuration set for the service
type Conf struct {
	ContainerModels             []*Model             `json:"containerModels"`
	CommonResponseCodes         []*ResponseMessage   `json:"commonResponseCodes"`
	CommonGetParams             []*Property          `json:"commonGetParams"`
	ResourceDescriptionTemplate *ResourceDescription `json:"resourceDescriptionTemplate"`
	ResourceTemplate            *Resource            `json:"resourceTemplate"`
	Port                        string               `json:"port"`
	Host                        string               `json:"host"`
	DbServer                    string               `json:"dbserver"`
	DbPort                      string               `json:"dbport"`
	StaticDirs                  []string             `json:"staticDirs"`
}

// Describes a Swagger-doc resource description
type ResourceDescription struct {
	SwaggerVersion string             `json:"swaggerVersion"`
	APIs           []*ResourceSummary `json:"apis"`
	ApiVersion     string             `json:"apiVersion"`
	Info           struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		ToS         string `json:"termsOfServiceUrl"`
		Contact     string `json:"contact"`
		License     string `json:"license"`
		LicenseUrl  string `json:"licenseUrl"`
	} `json:"info"`
	Authorizations map[string]*Authorization `json:"authorizations"`
}

// Save the resource description to the database backend.
func (rd *ResourceDescription) Save(db Db_backend) {
	db.Save(SwaggerResourceDB, ResourceDescriptionName, rd)
}

// describes a Swagger-doc resource summary (used in the resource description)
type ResourceSummary struct {
	Path        string `json:"path"`
	Description string `json:"description"`
}

// Describes a property of a Model or a parameter for an Operation
type Property struct {
	Type         string      `json:"type,omitempty"`
	Ref          string      `json:"$ref,omitempty"`
	Format       string      `json:"format,omitempty"`
	Enum         []string    `json:"enum,omitempty"`
	Minimum      float64     `json:"minimum,string,omitempty"`
	Maximum      float64     `json:"maximum,string,omitempty"`
	Items        *ItemsRef   `json:"items,omitempty"`
	UniqueItems  bool        `json:"uniqueItems,omitempty"`
	DefaultValue interface{} `json:"defaultValue,omitempty"`
	// parameters fields -
	// properties and params share a bunch of fields
	ParamType     string `json:"paramType,omitempty"`
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	Required      bool   `json:"required,omitempty"`
	AllowMultiple bool   `json:"allowMultiple,omitempty"`
}

// Describes a reference to another model within an array property
// Can either have the type and (optionally) format fields populated or the ref
// field populated.
type ItemsRef struct {
	Type   string `json:"type,omitempty"`
	Ref    string `json:"$ref,omitempty"`
	Format string `json:"format,omitempty"`
}

// Describes a swagger model
type Model struct {
	Id            string               `json:"id"`
	Description   string               `json:"description"`
	Required      []string             `json:"required,omitempty"`
	Properties    map[string]*Property `json:"properties"`
	SubTypes      []string             `json:"subTypes,omitempty"`
	Discriminator string               `json:"discriminator,omitempty"`
}

// Describes a swagger resource
type Resource struct {
	SwaggerVersion string                    `json:"swaggerVersion"`
	ApiVersion     string                    `json:"apiVersion"`
	BasePath       string                    `json:"basePath"`
	ResourcePath   string                    `json:"resourcePath"`
	Apis           []*Api                    `json:"apis"`
	Models         map[string]*Model         `json:"models"`
	Produces       []string                  `json:"produces"`
	Consumes       []string                  `json:"consumes"`
	Authorizations map[string]*Authorization `json:"authorizations"`
}

// Save stores a resource in a database backend
func (r *Resource) Save(db Db_backend) {
	docname := ResourceStem + strings.TrimLeft(r.ResourcePath, "/")
	db.Save(SwaggerResourceDB, docname, r)
}

// Describes an authorization option for a resource (not implemented yet)
type Authorization struct{}

// Describes an API
type Api struct {
	Path        string       `json:"path"`
	Description string       `json:"string"`
	Operations  []*Operation `json:"operations"`
}

// Describes an operation (e.g. a GET, PUT or POST operation)
type Operation struct {
	Method           string                    `json:"method"`
	Type             string                    `json:"type"`
	Items            *ItemsRef                 `json:"items,omitempty"`
	Summary          string                    `json:"summary"`
	Notes            string                    `json:"notes"`
	Nickname         string                    `json:"nickname"`
	Authorizations   map[string]*Authorization `json:"authorizations,omitempty"`
	Parameters       []*Property               `json:"parameters"`
	ResponseMessages []*ResponseMessage        `json:"responseMessages"`
	Produces         []string                  `json:"produces,omitempty"`
	Consumes         []string                  `json:"consumes,omitempty"`
	Deprecated       bool                      `json:"deprecated"`
}

// Describes a response message from an API
type ResponseMessage struct {
	Code          int    `json:"code"`
	Message       string `json:"message"`
	ResponseModel string `json:"responseModel,omitempty"`
}
