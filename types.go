package dragonfruit

import (
//"strings"
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
	ContainerModels           []*Schema            `json:"containerModels"`
	CommonSingleResponses     map[string]*Response `json:"commonSingleResponses"`
	CommonCollectionResponses map[string]*Response `json:"commonCollectionResponses"`
	CommonGetParams           []*Parameter         `json:"commonGetParams"`
	SwaggerTemplate           *Swagger             `json:"swaggerTemplate"`
	Port                      string               `json:"port"`
	Host                      string               `json:"host"`
	DbServer                  string               `json:"dbserver"`
	DbPort                    string               `json:"dbport"`
	StaticDirs                []string             `json:"staticDirs"`
}

// Describes a Swagger-doc resource description
type Swagger struct {
	Swagger string `json:"swagger"`
	Info    struct {
		Title          string          `json:"title,omitempty"`
		Description    string          `json:"description,omitempty"`
		TermsOfService string          `json:"termsOfService,omitempty"`
		Contact        ContactLicences `json:"contact,omitempty"`
		License        ContactLicences `json:"license,omitempty"`
		Version        string          `json:"version"`
	} `json:"info"`
	Host                string                     `json:"host,omitempty"`
	BasePath            string                     `json:"basePath,omitempty"`
	Schemes             []string                   `json:"schemes,omitempty"`
	Consumes            []string                   `json:"consumes,omitempty"`
	Produces            []string                   `json:"produces,omitempty"`
	Paths               map[string]*PathItem       `json:"paths"`
	Definitions         map[string]*Schema         `json:"definitions,omitempty"`
	Parameters          map[string]*Parameter      `json:"parameters,omitempty"`
	Responses           map[string]*Response       `json:"responses,omitempty"`
	SecurityDefinitions map[string]*SecurityScheme `json:"securityDefinitions,omitempty"`
	Security            map[string][]string        `json:"security,omitempty"`
	Tags                []*Tag                     `json:"tags,omitempty"`
	ExternalDocs        *ExternalDoc               `json:"externalDocs,omitempty"`
}

// An external documentation reference
type ExternalDoc struct {
	Description string `json:"description,omitempty"`
	Url         string `json:"url,omitempty"`
}

type Tag struct {
	Name         string       `json:"name"`
	Description  string       `json:"description,omitempty"`
	ExternalDocs *ExternalDoc `json:"externalDocs,omitempty"`
}

// A swagger contact or license reference
type ContactLicences struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// Describes an API
type PathItem struct {
	Ref        *PathItem    `json:"$ref,omitempty"`
	Get        *Operation   `json:"get,omitempty"`
	Put        *Operation   `json:"put,omitempty"`
	Post       *Operation   `json:"post,omitempty"`
	Delete     *Operation   `json:"delete,omitempty"`
	Options    *Operation   `json:"options,omitempty"`
	Head       *Operation   `json:"head,omitempty"`
	Patch      *Operation   `json:"patch,omitempty"`
	Parameters []*Parameter `json:"parameters,omitempty"`
}

// Describes a property of a Model or a parameter for an Operation
type Schema struct {
	Ref              string        `json:"$ref,omitempty"`
	Format           string        `json:"format,omitempty"`
	Title            string        `json:"title,omitempty"`
	Description      string        `json:"description,omitempty"`
	Default          interface{}   `json:"default,omitempty"`
	MultipleOf       int           `json:"multipleOf,omitempty"`
	Maximum          float64       `json:"maximum,omitempty"`
	ExclusiveMaximum bool          `json:"exclusiveMaximum,omitempty"`
	Minimum          float64       `json:"minimum,omitempty"`
	ExclusiveMinimum bool          `json:"exclusiveMinimum,omitempty"`
	MaxLength        int           `json:"maxLength,omitempty"`
	MinLength        int           `json:"minLength,omitempty"`
	Pattern          string        `json:"minLength,omitempty"`
	MaxItems         int           `json:"maxitems,omitempty"`
	MinItems         int           `json:"minitems,omitempty"`
	UniqueItems      bool          `json:"uniqueItems,omitempty"`
	MaxProperties    int           `json:"maxProperties,omitempty"`
	MinProperties    int           `json:"minProperties,omitempty"`
	Required         []string      `json:"required,omitempty"`
	Enum             []interface{} `json:"enum,omitempty"`
	Type             string        `json:"type,omitempty"`

	Items           *Schema   `json:"items,omitempty"`
	AdditionalItems bool      `json:"additionalItems,omitempty"`
	AllOf           []*Schema `json:"allOf,omitempty"`

	Properties           map[string]*Schema `json:"properties,omitempty"`
	AdditionalProperties bool               `json:"additionalProperties,omitempty"`

	Discriminator string `json:"discriminator,omitempty"`
	ReadOnly      bool   `json:"readOnly,omitempty"`
	// parameters fields -
	// properties and params share a bunch of fields
	XML          *XmlRef      `json:"xml,omitempty"`
	ExternalDocs *ExternalDoc `json:"externalDocs,omitempty"`
	Example      interface{}  `json:"example,omitempty"`
}

// A definition of the XML represenation of a schema.
type XmlRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Attribute bool   `json:"attribute,omitempty"`
	Wrapped   bool   `json:"wrapped,omitempty"`
}

// Save stores a resource in a database backend
/*func (r *Resource) Save(db Db_backend) {
	docname := ResourceStem + strings.TrimLeft(r.ResourcePath, "/")
	db.Save(SwaggerResourceDB, docname, r)
}*/

// Describes an authorization option for a resource (not implemented yet)
type Authorization struct{}

// Describes an operation (e.g. a GET, PUT or POST operation)
type Operation struct {
	Tags         []string             `json:"tags,omitempty"`
	Summary      string               `json:"summary,omitempty"`
	Description  string               `json:"description,omitempty"`
	ExternalDocs *ExternalDoc         `json:"externalDocs,omitempty"`
	OperationId  string               `json:"operationId,omitempty"`
	Produces     []string             `json:"produces,omitempty"`
	Consumes     []string             `json:"consumes,omitempty"`
	Parameters   []*Parameter         `json:"parameters,omitempty"`
	Responses    map[string]*Response `json:"responses"`
	Schemes      []string             `json:"schemes,omitempty"`
	Deprecated   bool                 `json:"deprecated,omitempty"`
	Security     map[string][]string  `json:"authorizations,omitempty"`
}

// Describes a parameter
type Parameter struct {
	Name             string        `json:"name"`
	In               string        `json:"in"`
	Description      string        `json:"description,omitempty"`
	Required         bool          `json:"required,omitempty"`
	Schema           *Schema       `json:"schema,omitempty"`
	Type             string        `json:"type,omitempty"`
	Format           string        `json:"format,omitempty"`
	AllowEmptyValue  bool          `json:"allowElmptyValue,omitempty"`
	Items            *Items        `json:"items,omitempty"`
	CollectionFormat string        `json:"collectionFormat,omitempty"`
	Default          interface{}   `json:"default,omitempty"`
	Maximum          float64       `json:"maximum,omitempty"`
	ExclusiveMaximum bool          `json:"exclusiveMaximum,omitempty"`
	Minimum          float64       `json:"minimum,omitempty"`
	ExclusiveMinimum bool          `json:"exclusiveMinimum,omitempty"`
	MaxLength        int           `json:"maxLength,omitempty"`
	MinLength        int           `json:"minLength,omitempty"`
	Pattern          string        `json:"minLength,omitempty"`
	MaxItems         int           `json:"maxitems,omitempty"`
	MinItems         int           `json:"minitems,omitempty"`
	UniqueItems      bool          `json:"uniqueItems,omitempty"`
	MaxProperties    int           `json:"maxProperties,omitempty"`
	MinProperties    int           `json:"minProperties,omitempty"`
	Enum             []interface{} `json:"enum,omitempty"`
	MultipleOf       int           `json:"multipleOf,omitempty"`
}

// Describes an array item in a parameter
type Items struct {
	Type             string        `json:"type"`
	Format           string        `json:"format,omitempty"`
	Items            *Items        `json:"items,omitempty"`
	CollectionFormat string        `json:"collectionFormat,omitempty"`
	Default          interface{}   `json:"default,omitempty"`
	Maximum          float64       `json:"maximum,omitempty"`
	ExclusiveMaximum bool          `json:"exclusiveMaximum,omitempty"`
	Minimum          float64       `json:"minimum,omitempty"`
	ExclusiveMinimum bool          `json:"exclusiveMinimum,omitempty"`
	MaxLength        int           `json:"maxLength,omitempty"`
	MinLength        int           `json:"minLength,omitempty"`
	Pattern          string        `json:"minLength,omitempty"`
	MaxItems         int           `json:"maxitems,omitempty"`
	MinItems         int           `json:"minitems,omitempty"`
	UniqueItems      bool          `json:"uniqueItems,omitempty"`
	MaxProperties    int           `json:"maxProperties,omitempty"`
	MinProperties    int           `json:"minProperties,omitempty"`
	Required         bool          `json:"required,omitempty"`
	Enum             []interface{} `json:"enum,omitempty"`
	MultipleOf       int           `json:"multipleOf,omitempty"`
}

// Describes a response message from an API
type Response struct {
	Description string  `json:"description"`
	Schema      *Schema `json:"schema,omitempty"`
	// headers and items are functionally equivalent
	Headers  map[string]*Items      `json:"headers,omitempty"`
	Examples map[string]interface{} `json:"example,omitempty"`
}

type SecurityScheme struct {
	Type             string            `json:"type"`
	Description      string            `json:"description,omitempty"`
	Name             string            `json:"name"`
	In               string            `json:"in"`
	Flow             string            `json:"flow"`
	AuthorizationUrl string            `json:"authorizationUrl"`
	TokenUrl         string            `json:"tokenUrl"`
	Scopes           map[string]string `json:"scopes"`
}
