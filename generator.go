package dragonfruit

import (
	"fmt"
	"github.com/gedex/inflector"
	"strings"
)

// getCommonResponseCodes loads a set of response codes that most of the APIs
// return.
func getCommonResponseCodes(cnf Conf, typ string) map[string]*Response {
	if typ == "collection" {
		return cnf.CommonCollectionResponses
	}
	return cnf.CommonSingleResponses
}

// getCommonGetParams loads a set of common params that should be added to
// all collection get operations (like limit and offset).
func getCommonGetParams(cnf Conf) []*Parameter {
	return cnf.CommonGetParams
}

// LoadDescriptionFromDb loads a resource description from a database backend.
// (see the backend stuff and types).
func LoadDescriptionFromDb(db Db_backend,
	cnf Conf) (*Swagger, error) {

	rd := new(Swagger)
	err := db.Load(SwaggerResourceDB, ResourceDescriptionName, rd)

	if err != nil {
		//TODO - fix this stupid shadowing issue
		rd = cnf.SwaggerTemplate
		return rd, nil
	}
	return rd, err
}

// MakeCommonAPIs creates a set of operations and APIs for a model.
// For any model passed to the function, two paths are created with the
// following operations on each:
// /{pathRoot} (GET and POST)
// /{pathRoot}/{id} (GET, PUT, PATCH, DELETE)
func MakeCommonAPIs(
	prefix string,
	pathRoot string,
	schemaName string,
	schemaMap map[string]*Schema,
	upstreamParams []*Parameter,
	cnf Conf,
) map[string]*PathItem {

	schema := schemaMap[schemaName]
	out := make(map[string]*PathItem)
	//modelDescription := inflector.Pluralize(inflector.Singularize(schemaName))

	// make the collection api - get lists of models and create new models
	collectionPath := prefix + "/" + pathRoot
	collectionApi := &PathItem{}
	collectionApi.Get = makeCollectionOperation(schemaName, schema, upstreamParams, cnf)
	collectionApi.Post = makePostOperation(schemaName, schema, upstreamParams, cnf)
	collectionApi.Options = makeCollectionOptionsOperation()

	out[collectionPath] = collectionApi

	// make a single API - use this for sub collections too
	idName, idparam := makePathId(schema)
	upstreamParams = append(upstreamParams, idparam)

	// TODO - parallelize this...
	individualPath := prefix + "/" + pathRoot + "/{" + idName + "}"
	individualApi := &PathItem{}
	individualApi.Delete = makeDeleteOperation(schemaName, upstreamParams, cnf)

	individualApi.Get = makeSingleGetOperation(schemaName, upstreamParams, cnf)
	individualApi.Put = makePutOperation(schemaName, schema, upstreamParams, cnf)
	individualApi.Patch = makePatchOperation(schemaName, upstreamParams, cnf)
	individualApi.Options = makeSingleOptionsOperation()

	out[individualPath] = individualApi

	subApis := makeSubApis(individualPath, schema, schemaMap, upstreamParams, cnf)
	for k, v := range subApis {
		out[k] = v
	}

	return out
}

// makeSubApis creates APIs for arrays of models which appear in models.
func makeSubApis(
	prefix string,
	schema *Schema,
	schemaMap map[string]*Schema,
	upstreamParams []*Parameter,
	cnf Conf,
) map[string]*PathItem {

	out := make(map[string]*PathItem)
	for _, propSchema := range schema.Properties {
		if (propSchema.Type == "array") && (propSchema.Items.Ref != "") {
			subModelName := DeRef(propSchema.Items.Ref)
			st := inflector.Pluralize(inflector.Singularize(subModelName))
			resourceroot := strings.ToLower(st)
			commonApis := MakeCommonAPIs(prefix, resourceroot, subModelName,
				schemaMap, upstreamParams, cnf)

			for k, v := range commonApis {
				out[k] = v
			}
		}
	}
	return out
}

// makePathId determines what property to use as the ID param when for paths
// which have parameterized IDs (e.g. /model_name/{id}
func makePathId(schema *Schema) (propName string, idparam *Parameter) {
	// find a property with "ID" in the name
	for propName, propValue := range schema.Properties {

		// The property must be a primitive value - it can't be an array
		// or a reference to another model
		if propValue.Type != "array" && propValue.Ref == "" {
			if propName == "id" {
				idparam = &Parameter{
					Name:     propName,
					Type:     propValue.Type,
					In:       "path",
					Format:   propValue.Format,
					Required: true,
				}
				return propName, idparam
			}

			if strings.Contains(propName, "Id") && propValue.Type != "" {
				idparam = &Parameter{
					Name:     propName,
					Type:     propValue.Type,
					In:       "path",
					Format:   propValue.Format,
					Required: true,
				}
				return propName, idparam
			}
		}
	}

	// if there's no ID parameter, make one and mutate the schema
	// this is bad, but if you don't name your fields, that's what you
	// get I suppose
	propname := schema.Title + "Id"
	idparam = &Parameter{
		Name:     propname,
		Type:     "integer",
		In:       "path",
		Required: true,
	}

	// mutation...
	schema.Properties[propname] = &Schema{
		Title: propname,
		Type:  "integer",
	}
	schema.Required = []string{propname}
	return propname, idparam
}

// makeDeleteOperation creates operations to delete single instances of a model.
func makeDeleteOperation(schemaName string,
	upstreamParams []*Parameter, cnf Conf) (deleteOp *Operation) {

	deleteOp = &Operation{
		OperationId: "delete" + schemaName,
		Summary:     "Delete a " + schemaName + " object.",
		Responses:   copyResponseMap(cnf.CommonSingleResponses),
	}

	responseSchema := buildSimpleResponseSchema("200", "Successfully deleted")

	deleteOp.Responses["200"] = &Response{
		Description: "Successful deletion",
		Schema:      responseSchema,
	}

	deleteOp.Parameters = append(deleteOp.Parameters, upstreamParams...)
	return

}

// makeSingleGetOperation makes operations to load single instances of models.
// Basically for URLs ending /{model id}.
func makeSingleGetOperation(schemaName string, upstreamParams []*Parameter,
	cnf Conf) (getOp *Operation) {

	getOp = &Operation{
		OperationId: "getSingle" + schemaName,
		Summary:     "Get a single " + schemaName + " object.",
		Responses:   copyResponseMap(cnf.CommonSingleResponses),
	}

	ref := MakeRef(schemaName + strings.Title(ContainerName))

	// Add a 200 response for successful operations
	getOp.Responses["200"] = &Response{
		Schema: &Schema{
			Ref: ref,
		},
		Description: "A single " + schemaName,
	}

	getOp.Parameters = append(getOp.Parameters, upstreamParams...)
	return
}

// makePatchOperation creates operations to partially update models.
func makePatchOperation(schemaName string,
	upstreamParams []*Parameter, cnf Conf) (patchOp *Operation) {
	// Create the put operation

	patchOp = &Operation{
		OperationId: "updatePartial" + schemaName,
		Summary:     "Partially update a  " + schemaName + " object.",
		Responses:   copyResponseMap(cnf.CommonSingleResponses),
	}

	ref := MakeRef(schemaName + strings.Title(ContainerName))

	ioSchema := &Schema{
		Ref: ref,
	}

	// Add a 200 response for successful operations
	patchOp.Responses["200"] = &Response{
		Schema:      ioSchema,
		Description: "Successfully updated " + schemaName,
	}

	// The patch body
	bodyParam := &Parameter{
		Name:        "body",
		In:          "body",
		Description: "A partial " + schemaName,
		Required:    true,
		Schema:      ioSchema,
	}

	patchOp.Parameters = append(patchOp.Parameters, bodyParam)
	patchOp.Parameters = append(patchOp.Parameters, upstreamParams...)
	return
}

// makePutOperation creates operations to update models.
func makePutOperation(schemaName string, schema *Schema,
	upstreamParams []*Parameter, cnf Conf) (putOp *Operation) {

	// Create the put operation
	putOp = &Operation{
		OperationId: "update" + schemaName,
		Summary:     "Update a " + schemaName + " object.",
		Responses:   copyResponseMap(cnf.CommonSingleResponses),
	}

	ref := MakeRef(schemaName + strings.Title(ContainerName))

	ioSchema := &Schema{
		Ref: ref,
	}

	// Add a 200 response for successful operations
	putOp.Responses["200"] = &Response{
		Schema:      ioSchema,
		Description: "Successfully updated " + schemaName,
	}

	// The patch body
	bodyParam := &Parameter{
		Name:        "body",
		In:          "body",
		Description: "A partial " + schemaName,
		Required:    true,
		Schema:      schema,
	}

	putOp.Parameters = append(putOp.Parameters, bodyParam)
	putOp.Parameters = append(putOp.Parameters, upstreamParams...)
	return
}

// makePostOperation makes a POST operation to create new instances of models.
func makePostOperation(schemaName string, schema *Schema,
	upstreamParams []*Parameter, cnf Conf) (postOp *Operation) {

	postOp = &Operation{
		OperationId: "new" + schemaName,
		Summary:     "Create a new " + schemaName + " object.",
		Responses:   make(map[string]*Response),
	}
	ref := MakeRef(schemaName + strings.Title(ContainerName))

	ioSchema := &Schema{
		Ref: ref,
	}
	// Add a 201 response for newly created models
	postResp := &Response{

		Schema: ioSchema,
	}
	postOp.Responses["201"] = postResp
	// Post body to create the new model.
	bodyParam := &Parameter{
		Name:        "body",
		In:          "body",
		Schema:      schema,
		Required:    true,
		Description: "A new " + schemaName,
	}
	postOp.Parameters = append(postOp.Parameters, bodyParam)
	postOp.Parameters = append(postOp.Parameters, upstreamParams...)
	return

}

// makeCollectionOperations defines GET calls for collections of the model.
// Basically, GET operations on URLs ending with /
func makeCollectionOperation(schemaName string, schema *Schema,
	upstreamParams []*Parameter,
	cnf Conf) (getOp *Operation) {

	getOp = &Operation{
		OperationId: "get" + schemaName + "Collection",
		Summary:     "Get multiple " + inflector.Pluralize(inflector.Singularize(schemaName)) + ".",
		Responses:   copyResponseMap(cnf.CommonCollectionResponses),
	}

	ref := MakeRef(schemaName + strings.Title(ContainerName))

	ioSchema := &Schema{
		Ref: ref,
	}

	getOp.Responses["200"] = &Response{
		Schema:      ioSchema,
		Description: "A collection of " + schemaName,
	}

	// add the parameters
	getOp.Parameters = getCommonGetParams(cnf)
	for propName, prop := range schema.Properties {
		fmt.Println("property: ", propName, prop.Type, prop)

		switch prop.Type {
		// if there is no type, the item is a ref
		// don't add any properties...
		case "":
			break

		// strings check for dates and add ranges for date fields
		case "string":
			params := makeStringParams(propName, prop)
			getOp.Parameters = append(getOp.Parameters, params...)
			break
		// arrays query against their type
		case "array":
			if prop.Items.Type != "" {
				param := makeArrayParams(propName, prop)
				getOp.Parameters = append(getOp.Parameters, param...)
			}
			break
		// ints and numbers...
		case "number":
			params := makeNumParams(propName, prop)
			fmt.Printf("%+v", params)
			getOp.Parameters = append(getOp.Parameters, params...)
			break
		case "integer":
			params := makeNumParams(propName, prop)
			getOp.Parameters = append(getOp.Parameters, params...)
			break
		// anything else (bools)
		default:
			param := makeGenParams(propName, prop)
			getOp.Parameters = append(getOp.Parameters, param...)
		}
	}
	getOp.Parameters = append(getOp.Parameters, upstreamParams...)

	return
}

// makeCollectionOptionsOperation returns the options on
// a collection url
func makeCollectionOptionsOperation() (optOp *Operation) {
	headers := make(map[string]*Items)
	optOp = &Operation{}
	optOp.Responses = make(map[string]*Response)

	var header = &Items{
		Type:    "string",
		Default: "GET, POST",
	}

	headers["Allow"] = header

	optOp.Responses["200"] = &Response{
		Headers:     headers,
		Description: "This url allows GET and POST operations.",
	}
	return
}

// makeCollectionOptionsOperation returns the options on
// a single url
func makeSingleOptionsOperation() (optOp *Operation) {
	headers := make(map[string]*Items)

	optOp = &Operation{}
	optOp.Responses = make(map[string]*Response)

	var header = &Items{
		Type:    "string",
		Default: "GET, PUT, DELETE, PATCH",
	}

	headers["Allow"] = header

	optOp.Responses["200"] = &Response{
		Description: "This url allows GET, PUT, PATCH and DELETE operations.",
		Headers:     headers,
	}
	return
}

/* The following make*Param functions look at Swagger model properties and
translate them into query params for the APIs being generated. Swagger
properties and params have the same structure so these functions return
[]*Property. Some of the functions only return slices with a length of one, but
slices are always returned to keep the API consistent. */

// makeGenParam makes a generic parameter using the type, enum, name and
// format of the property.
func makeGenParams(propName string, schema *Schema) (p []*Parameter) {
	pr := &Parameter{
		Type:   schema.Type,
		Enum:   schema.Enum,
		In:     "query",
		Name:   propName,
		Format: schema.Format,
	}
	p = append(p, pr)
	return
}

// makeArrayParam makes a parameter to query elements in an array of primitive
// values.  The type and format are used from the array's Item property.
func makeArrayParams(propName string, schema *Schema) (p []*Parameter) {
	pr := &Parameter{
		Name:   propName,
		Type:   schema.Items.Type,
		Format: schema.Items.Format,
		In:     "query",
	}
	p = append(p, pr)
	return
}

// makeNumParam makes query parameters for numerical values.  If the
// property does NOT have an enum property, a range query is defined.
func makeNumParams(propName string, schema *Schema) (p []*Parameter) {
	fmt.Println(propName)
	p = make([]*Parameter, 0, 0)
	pr := Parameter{
		Type:    schema.Type,
		Minimum: schema.Minimum,
		Maximum: schema.Maximum,
		Format:  schema.Format,
		In:      "query",
		Name:    propName,
	}

	if len(schema.Enum) == 0 {
		prRange := pr
		prRange.Type = "array"
		prRange.Items = &Items{
			Type:    schema.Type,
			Minimum: schema.Minimum,
			Maximum: schema.Maximum,
			Format:  schema.Format,
		}
		prRange.CollectionFormat = "csv"
		prRange.Name = propName + "Range"
		p = append(p, &prRange)
	} else {
		pr.Enum = schema.Enum
	}
	p = append(p, &pr)
	return
}

// makeStringParams makes query parameters for string values.  If the
// property is a date or date-time, it adds a range query as well.
func makeStringParams(propName string, schema *Schema) (p []*Parameter) {
	pr := &Parameter{
		Type:   schema.Type,
		Enum:   schema.Enum,
		In:     "query",
		Name:   propName,
		Format: schema.Format,
	}
	p = append(p, pr)
	if (schema.Format == "date" || schema.Format == "date-time") && len(schema.Enum) == 0 {
		rangeField := &Parameter{
			Type:   "array",
			In:     "query",
			Name:   propName,
			Format: schema.Format,
		}
		rangeField.Name = propName + "Range"
		rangeField.CollectionFormat = "csv"

		p = append(p, rangeField)
	}

	return
}

// Convenience function for building simple code/message responses
func buildSimpleResponseSchema(code string, message string) (out *Schema) {
	out = &Schema{}
	out.Properties = make(map[string]*Schema)

	out.Properties["code"] = &Schema{
		Type: "integer",
	}
	out.Properties["message"] = &Schema{
		Type: "string",
	}
	out.Required = []string{"type", "message"}
	out.Example = struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{
		code,
		message,
	}
	return
}

// Convenience function for copying a response map because Golang Reasons...
func copyResponseMap(in map[string]*Response) map[string]*Response {
	out := make(map[string]*Response)
	for k, v := range in {
		out[k] = v
	}
	return out
}
