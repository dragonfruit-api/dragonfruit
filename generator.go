package dragonfruit

import (
	"github.com/gedex/inflector"
	"strings"
)

// getCommonResponseCodes loads a set of response codes that most of the APIs
// return.
func getCommonResponseCodes(cnf Conf) []*ResponseMessage {

	return cnf.CommonResponseCodes
}

// getCommonGetParams loads a set of common params that should be added to
// all collection get operations (like limit and offset).
func getCommonGetParams(cnf Conf) []*Property {
	return cnf.CommonGetParams
}

// LoadDescriptionFromDb loads a resource description from a database backend.
// (see the backend stuff and types).
func LoadDescriptionFromDb(db Db_backend,
	cnf Conf) (*ResourceDescription, error) {

	rd := new(ResourceDescription)
	err := db.Load(SwaggerResourceDB, ResourceDescriptionName, rd)

	if err != nil {
		//TODO - fix this stupid shadowing issue
		rd = cnf.ResourceDescriptionTemplate
	}
	return rd, err
}

// LoadResourceFromDb loads a resource from a database backend
// (see the backend stuff and types).
func LoadResourceFromDb(db Db_backend,
	resourcePath string, cnf Conf) (*Resource, error) {

	res := new(Resource)
	err := db.Load(SwaggerResourceDB, ResourceStem+resourcePath, &res)

	// doc wasn't found
	if err != nil {
		res = cnf.ResourceTemplate
	}

	return res, err
}

// MakeCommonAPIs creates a set of operations and APIs for a model.
// For any model passed to the function, two paths are created with the
// following operations on each:
// /{resourceRoot} (GET and POST)
// /{resourceRoot}/{id} (GET, PUT, PATCH, DELETE)
func MakeCommonAPIs(
	prefix string,
	resourceRoot string,
	modelName string,
	modelMap map[string]*Model,
	upstreamParams []*Property,
	cnf Conf,
) []*Api {

	model := modelMap[modelName]
	out := make([]*Api, 0)
	modelDescription := inflector.Pluralize(inflector.Singularize(modelName))

	// make the collection api - get lists of models and create new models
	collApi := &Api{
		Path:        prefix + "/" + resourceRoot,
		Description: "Operations on collections of " + modelDescription,
	}

	// make the colleciton operations and parse the sub parts
	getOp := makeCollectionOperation(modelName, model, upstreamParams, cnf)
	collApi.Operations = append(collApi.Operations, getOp)

	postOp := makePostOperation(modelName, model, upstreamParams, cnf)
	collApi.Operations = append(collApi.Operations, postOp)

	// make a single API - use this for sub collections too
	idName, idparam := makePathId(model)
	upstreamParams = append(upstreamParams, idparam)

	singleApi := &Api{
		Path:        prefix + "/" + resourceRoot + "/{" + idName + "}",
		Description: "Operations on single instances of " + modelDescription,
	}

	// TODO - parallelize this...

	out = append(out, collApi)

	getSingleOp := makeSingleGetOperation(modelName, model, upstreamParams, cnf)
	singleApi.Operations = append(singleApi.Operations, getSingleOp)

	putOp := makePutOperation(modelName, model, upstreamParams, cnf)
	singleApi.Operations = append(singleApi.Operations, putOp)

	patchOp := makePatchOperation(modelName, model, upstreamParams, cnf)
	singleApi.Operations = append(singleApi.Operations, patchOp)

	deleteOp := makeDeleteOperation(modelName, model, upstreamParams, cnf)
	singleApi.Operations = append(singleApi.Operations, deleteOp)

	out = append(out, singleApi)

	subApis := makeSubApis(singleApi.Path, model, modelMap, upstreamParams, cnf)
	out = append(out, subApis...)

	return out
}

// makeSubApis creates APIs for arrays of models which appear in models.
func makeSubApis(
	prefix string,
	model *Model,
	modelMap map[string]*Model,
	upstreamParams []*Property,
	cnf Conf,
) []*Api {

	out := make([]*Api, 0)
	for _, prop := range model.Properties {
		// TODO - HANDLE SUB-APIS FOR ARRAYS OF PRIMITIVE VALUES
		if (prop.Type == "array") && (prop.Items.Ref != "") {
			subModelName := prop.Items.Ref
			st := inflector.Pluralize(inflector.Singularize(subModelName))
			resourceroot := strings.ToLower(st)
			commonApis := MakeCommonAPIs(prefix, resourceroot, subModelName,
				modelMap, upstreamParams, cnf)

			out = append(out, commonApis...)
		}
	}
	return out
}

// makePathId determines what property to use as the ID param when for paths
// which have parameterized IDs (e.g. /model_name/{id}
func makePathId(model *Model) (propName string, idparam *Property) {
	// find a property with "ID" in the name
	for propName, propValue := range model.Properties {

		// The property must be a primitive value - it can't be an array
		// or a reference to another model
		if propValue.Type != "array" && propValue.Ref == "" {
			if propName == "id" {
				idparam = &Property{
					Name:      propName,
					Type:      propValue.Type,
					ParamType: "path",
					Format:    propValue.Format,
					Required:  true,
				}
				return propName, idparam
			}

			if strings.Contains(propName, "Id") && propValue.Type != "" {
				idparam = &Property{
					Name:      propName,
					Type:      propValue.Type,
					ParamType: "path",
					Format:    propValue.Format,
					Required:  true,
				}
				return propName, idparam
			}
		}
	}

	// if there's no ID parameter, use the position in the array as
	// the ID
	idparam = &Property{
		Name:      "pos",
		Type:      "integer",
		ParamType: "path",
		Required:  true,
	}
	return "pos", idparam
}

// makeDeleteOperation creates operations to delete single instances of a model.
func makeDeleteOperation(modelName string,
	model *Model, upstreamParams []*Property, cnf Conf) (deleteOp *Operation) {

	deleteOp = &Operation{
		Method:   "DELETE",
		Type:     "void",
		Nickname: "delete" + modelName,
		Summary:  "Delete a " + modelName + " object.",
	}

	deleteResp := &ResponseMessage{
		Code:    200,
		Message: "Successfully deleted",
	}
	deleteOp.ResponseMessages = append(getCommonResponseCodes(cnf), deleteResp)

	deleteOp.Parameters = append(deleteOp.Parameters, upstreamParams...)
	return

}

// makeSingleGetOperation makes operations to load single instances of models.
// Basically for URLs ending /{model id}.
func makeSingleGetOperation(modelName string, model *Model,
	upstreamParams []*Property, cnf Conf) (patchOp *Operation) {

	patchOp = &Operation{
		Method:   "GET",
		Type:     modelName + strings.Title(ContainerName),
		Nickname: "getSingle" + modelName,
		Summary:  "Get a single " + modelName + " object.",
	}

	// Add a 200 response for successful operations
	putResp := &ResponseMessage{
		Code:          200,
		Message:       "Ok",
		ResponseModel: modelName + strings.Title(ContainerName),
	}
	patchOp.ResponseMessages = append(getCommonResponseCodes(cnf), putResp)

	patchOp.Parameters = append(patchOp.Parameters, upstreamParams...)
	return
}

// makePatchOperation creates operations to partially update models.
func makePatchOperation(modelName string, model *Model,
	upstreamParams []*Property, cnf Conf) (patchOp *Operation) {
	// Create the put operation

	patchOp = &Operation{
		Method:   "PATCH",
		Type:     modelName,
		Nickname: "updatePartial" + modelName,
		Summary:  "Partially update a  " + modelName + " object.",
	}

	// Add a 200 response for successful operations
	putResp := &ResponseMessage{
		Code:          200,
		Message:       "Successfully updated",
		ResponseModel: modelName,
	}
	patchOp.ResponseMessages = append(getCommonResponseCodes(cnf), putResp)

	// The patch body
	bodyParam := &Property{
		Name:      "body",
		ParamType: "body",
		Ref:       modelName,
		Required:  true,
		Type:      modelName,
	}

	patchOp.Parameters = append(patchOp.Parameters, bodyParam)
	patchOp.Parameters = append(patchOp.Parameters, upstreamParams...)
	return
}

// makePutOperation creates operations to update models.
func makePutOperation(modelName string,
	model *Model, upstreamParams []*Property, cnf Conf) (putOp *Operation) {

	// Create the put operation
	putOp = &Operation{
		Method:   "PUT",
		Type:     modelName,
		Nickname: "update" + modelName,
		Summary:  "Update a " + modelName + " object.",
	}

	// Add a 200 response for successful operations
	putResp := &ResponseMessage{
		Code:          200,
		Message:       "Successfully updated",
		ResponseModel: modelName,
	}
	putOp.ResponseMessages = append(getCommonResponseCodes(cnf), putResp)

	// The put body
	bodyParam := &Property{
		Name:      "body",
		ParamType: "body",
		Ref:       modelName,
		Required:  true,
		Type:      modelName,
	}

	putOp.Parameters = append(putOp.Parameters, bodyParam)
	putOp.Parameters = append(putOp.Parameters, upstreamParams...)
	return
}

// makePostOperation makes a POST operation to create new instances of models.
func makePostOperation(modelName string,
	model *Model, upstreamParams []*Property, cnf Conf) (postOp *Operation) {

	postOp = &Operation{
		Method:   "POST",
		Type:     modelName,
		Nickname: "new" + modelName,
		Summary:  "Create a new " + modelName + " object.",
	}

	// Add a 201 response for newly created models
	postResp := &ResponseMessage{
		Code:          201,
		Message:       "Successfully created",
		ResponseModel: modelName,
	}
	postOp.ResponseMessages = append(getCommonResponseCodes(cnf), postResp)

	// Post body to create the new model.
	bodyParam := &Property{
		Name:      "body",
		ParamType: "body",
		Ref:       modelName,
		Required:  true,
		Type:      modelName,
	}
	postOp.Parameters = append(postOp.Parameters, bodyParam)
	postOp.Parameters = append(postOp.Parameters, upstreamParams...)
	return

}

// makeCollectionOperations defines GET calls for collections of the model.
// Basically, GET operations on URLs ending with /
func makeCollectionOperation(modelName string, model *Model, upstreamParams []*Property, cnf Conf) (getOp *Operation) {
	getOp = &Operation{
		Method:   "GET",
		Type:     modelName + strings.Title(ContainerName),
		Nickname: "get" + modelName + "Collection",
		Summary:  "Get multiple " + modelName + " objects.",
	}

	// add the response messages
	getResp := &ResponseMessage{
		Code:          200,
		Message:       "Successful Lookup",
		ResponseModel: modelName + strings.Title(ContainerName),
	}
	getOp.ResponseMessages = append(getCommonResponseCodes(cnf), getResp)

	// add the parameters
	getOp.Parameters = getCommonGetParams(cnf)
	for propName, prop := range model.Properties {
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

/* The following make*Param functions look at Swagger model properties and
translate them into query params for the APIs being generated. Swagger
properties and params have the same structure so these functions return
[]*Property. Some of the functions only return slices with a length of one, but
slices are always returned to keep the API consistent. */

// makeGenParam makes a generic parameter using the type, enum, name and
// format of the property.
func makeGenParams(propName string, prop *Property) (p []*Property) {
	pr := &Property{
		Type:      prop.Type,
		Enum:      prop.Enum,
		ParamType: "query",
		Name:      propName,
		Format:    prop.Format,
	}
	p = append(p, pr)
	return
}

// makeArrayParam makes a parameter to query elements in an array of primitive
// values.  The type and format are used from the array's Item property.
func makeArrayParams(propName string, prop *Property) (p []*Property) {
	pr := &Property{
		Name:      propName,
		Type:      prop.Items.Type,
		Format:    prop.Items.Format,
		ParamType: "query",
	}
	p = append(p, pr)
	return
}

// makeNumParam makes query parameters for numerical values.  If the
// property does NOT have an enum property, a range query is defined.
func makeNumParams(propName string, prop *Property) (p []*Property) {
	pr := &Property{
		Type:      prop.Type,
		Minimum:   prop.Minimum,
		Maximum:   prop.Maximum,
		Format:    prop.Format,
		ParamType: "query",
		Name:      propName,
	}
	p = append(p, pr)

	if len(pr.Enum) == 0 {
		prRange := pr
		prRange.AllowMultiple = true
		prRange.Name = propName + "Range"
		p = append(p, prRange)
	}

	return
}

// makeStringParams makes query parameters for string values.  If the
// property is a date or date-time, it adds a range query as well.
func makeStringParams(propName string, prop *Property) (p []*Property) {
	pr := &Property{
		Type:      prop.Type,
		Enum:      prop.Enum,
		ParamType: "query",
		Name:      propName,
		Format:    prop.Format,
	}
	p = append(p, pr)
	if (prop.Format == "date" || prop.Format == "date-time") && len(prop.Enum) == 0 {
		rangeField := pr
		rangeField.Name = propName + "Range"
		rangeField.AllowMultiple = true

		p = append(p, rangeField)
	}

	return
}
