package swagger_gen

import (
	"encoding/json"
	//"fmt"
	"github.com/gedex/inflector"
	"io/ioutil"
	"strings"
)

var (
	commonResponseCodes []*ResponseMessage
	commonGetParams     []*Property
)

func init() {
	commonResponseCodes = make([]*ResponseMessage, 0)
	byt, _ := ioutil.ReadFile("responseMessages.json")
	err := json.Unmarshal(byt, &commonResponseCodes)
	if err != nil {
		panic(err)
	}

	commonGetParams = make([]*Property, 0)
	byt, _ = ioutil.ReadFile("commonGetParams.json")
	err = json.Unmarshal(byt, &commonGetParams)
	if err != nil {
		panic(err)
	}

}

func LoadDescriptionFromDb(db Db_backend, fallbackTemplate string) (*ResourceDescription, error) {
	rd := new(ResourceDescription)

	//byt, err := ioutil.ReadFile(fallbackTemplate)

	//json.Unmarshal(byt, rd)

	err := db.Load(SwaggerResourceDB, ResourceDescriptionName, rd)
	//fmt.Println("error returned by db load:", err, rd)

	if err != nil {
		//TODO - fix this stupid shadowing issue
		byt, _ := ioutil.ReadFile(fallbackTemplate)

		json.Unmarshal(byt, rd)
	}
	return rd, err
}

func LoadResourceFromDb(db Db_backend, resourcePath string, fallbackTemplate string) (*Resource, error) {
	//res, err := NewFromTemplate(fallbackTemplate)
	res := new(Resource)
	err := db.Load(SwaggerResourceDB, ResourceStem+resourcePath, &res)

	// doc wasn't found
	if err != nil {
		res, err = NewFromTemplate(fallbackTemplate)
	}

	return res, err
}

// Create a new API resource from a template file
func NewFromTemplate(filePath string) (*Resource, error) {
	res := new(Resource)
	byt, err := ioutil.ReadFile(filePath)

	json.Unmarshal(byt, res)
	return res, err
}

// we'll be making 2 paths with a handful of
// operations on each:
// /{resourceRoot} (GET and POST)
// /{resourceRoot}/{id} (GET, PUT, PATCH, DELETE)
func MakeCommonAPIs(
	prefix string,
	resourceRoot string,
	modelName string,
	modelMap map[string]*Model,
	upstreamParams []*Property,
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
	getOp := makeCollectionOperation(modelName, model, upstreamParams)
	collApi.Operations = append(collApi.Operations, getOp)

	postOp := makePostOperation(modelName, model, upstreamParams)
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

	getSingleOp := makeSingleGetOperation(modelName, model, upstreamParams)
	singleApi.Operations = append(singleApi.Operations, getSingleOp)

	putOp := makePutOperation(modelName, model, upstreamParams)
	singleApi.Operations = append(singleApi.Operations, putOp)

	patchOp := makePatchOperation(modelName, model, upstreamParams)
	singleApi.Operations = append(singleApi.Operations, patchOp)

	deleteOp := makeDeleteOperation(modelName, model, upstreamParams)
	singleApi.Operations = append(singleApi.Operations, deleteOp)

	out = append(out, singleApi)

	out = append(out, makeSubApis(singleApi.Path, model, modelMap, upstreamParams)...)

	return out
}

func makeSubApis(
	prefix string,
	model *Model,
	modelMap map[string]*Model,
	upstreamParams []*Property,
) []*Api {
	out := make([]*Api, 0)
	for _, prop := range model.Properties {
		subModelName := strings.Replace(prop.Ref, strings.Title(ContainerName), "", -1)
		resourceroot := strings.ToLower(inflector.Pluralize(inflector.Singularize(subModelName)))

		// make APIs out of any container types
		if strings.Contains(prop.Ref, strings.Title(ContainerName)) {
			out = append(out, MakeCommonAPIs(prefix, resourceroot, subModelName, modelMap, upstreamParams)...)
		}

		// TODO - HANDLE SUB-APIS FOR ARRAYS
		if prop.Type == "array" {

		}

	}
	return out
}

// Figure out what to use as an ID param
func makePathId(model *Model) (propName string, idparam *Property) {
	// find a property with "ID" in the name
	for propName, propValue := range model.Properties {
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

func makeDeleteOperation(modelName string, model *Model, upstreamParams []*Property) (deleteOp *Operation) {
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
	deleteOp.ResponseMessages = append(commonResponseCodes, deleteResp)

	deleteOp.Parameters = append(deleteOp.Parameters, upstreamParams...)
	return

}

// Get a single model
func makeSingleGetOperation(modelName string, model *Model, upstreamParams []*Property) (patchOp *Operation) {
	// Create the put operation

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
	patchOp.ResponseMessages = append(commonResponseCodes, putResp)

	patchOp.Parameters = append(patchOp.Parameters, upstreamParams...)
	return
}

// Update models
func makePatchOperation(modelName string, model *Model, upstreamParams []*Property) (patchOp *Operation) {
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
	patchOp.ResponseMessages = append(commonResponseCodes, putResp)

	// The put body
	bodyParam := &Property{
		Name:      "body",
		ParamType: "body",
		Ref:       modelName,
		Required:  true,
	}

	patchOp.Parameters = append(patchOp.Parameters, bodyParam)
	patchOp.Parameters = append(patchOp.Parameters, upstreamParams...)
	return
}

// Update models
func makePutOperation(modelName string, model *Model, upstreamParams []*Property) (putOp *Operation) {
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
	putOp.ResponseMessages = append(commonResponseCodes, putResp)

	// The put body
	bodyParam := &Property{
		Name:      "body",
		ParamType: "body",
		Ref:       modelName,
		Required:  true,
	}

	putOp.Parameters = append(putOp.Parameters, bodyParam)
	putOp.Parameters = append(putOp.Parameters, upstreamParams...)
	return
}

// Make a POST operation to create new models
func makePostOperation(modelName string, model *Model, upstreamParams []*Property) (postOp *Operation) {
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
	postOp.ResponseMessages = append(commonResponseCodes, postResp)

	// Post body to create the new model.
	bodyParam := &Property{
		Name:      "body",
		ParamType: "body",
		Ref:       modelName,
		Required:  true,
	}
	postOp.Parameters = append(postOp.Parameters, bodyParam)
	postOp.Parameters = append(postOp.Parameters, upstreamParams...)
	return

}

// Create a GET operation for collections of the model and associated filters
func makeCollectionOperation(modelName string, model *Model, upstreamParams []*Property) (getOp *Operation) {

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
	getOp.ResponseMessages = append(commonResponseCodes, getResp)

	// add the parameters
	getOp.Parameters = commonGetParams
	for propName, prop := range model.Properties {
		//fmt.Println(propName, prop)
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
			param := makeArrayParam(propName, prop)
			getOp.Parameters = append(getOp.Parameters, param)
			break
		// ints and numbers...
		case "number":
		case "integer":
			params := makeNumParams(propName, prop)
			getOp.Parameters = append(getOp.Parameters, params...)
			break
		// anything else (bools)
		default:
			param := makeGenParam(propName, prop)
			getOp.Parameters = append(getOp.Parameters, param)
		}
	}
	getOp.Parameters = append(getOp.Parameters, upstreamParams...)

	return
}

func makeArrayParam(propName string, prop *Property) (pr *Property) {
	//fmt.Println("in func", propName, prop)
	pr = new(Property)
	pr.Name = propName
	pr.Type = prop.Items.Type
	pr.Format = prop.Items.Format
	pr.ParamType = "query"
	return
}

func makeNumParams(propName string, prop *Property) (p []*Property) {
	//fmt.Println(prop.Minimum, prop.Maximum)
	pr := Property{
		Type:      prop.Type,
		Minimum:   prop.Minimum,
		Maximum:   prop.Maximum,
		Format:    prop.Format,
		ParamType: "query",
		Name:      propName,
	}
	p = append(p, &pr)

	if len(pr.Enum) == 0 {
		prRange := pr
		prRange.AllowMultiple = true
		prRange.Name = propName + "Range"
		p = append(p, &prRange)
	}

	return
}

func makeGenParam(propName string, prop *Property) (pr *Property) {
	pr = &Property{
		Type:      prop.Type,
		Enum:      prop.Enum,
		ParamType: "query",
		Name:      propName,
		Format:    prop.Format,
	}
	return
}

func makeStringParams(propName string, prop *Property) (p []*Property) {
	pr := Property{
		Type:      prop.Type,
		Enum:      prop.Enum,
		ParamType: "query",
		Name:      propName,
		Format:    prop.Format,
	}
	p = append(p, &pr)
	if (prop.Format == "date" || prop.Format == "date-time") && len(prop.Enum) == 0 {
		rangeField := pr
		rangeField.Name = propName + "Range"
		rangeField.AllowMultiple = true

		p = append(p, &rangeField)
	}

	return
}
