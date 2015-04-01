package backend_couchdb

import (
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fjl/go-couchdb"
	"github.com/gedex/inflector"
	"github.com/ideo/dragonfruit"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// A CouchDB view.
type view struct {
	MapFunc    string `json:"map"`
	ReduceFunc string `json:"reduce,omitempty"`
}

// A CouchDB design document.
type viewDoc struct {
	Id       string          `json:"_id"`
	Rev      string          `json:"_rev,omitempty"`
	Language string          `json:"language"`
	Views    map[string]view `json:"views"`
}

// Db_backend_couch is the exported client that you would use in your app.
type Db_backend_couch struct {
	client     *couchdb.Client
	connection chan bool
}

// viewParams are used to create design documents during the prep phase
type viewParam struct {
	path         string
	singlepath   string
	paramname    string
	paramtype    string
	propertyname string
}

// Represents a row returned by a couchdb result
type couchdbRow struct {
	Doc   map[string]interface{} `json:"doc,omitempty"`
	Id    string                 `json:"id"`
	Key   interface{}            `json:"key"`
	Value map[string]interface{} `json:"value"`
}

// Represents a couchdb result
type couchDbResponse struct {
	Rows      []couchdbRow `json:"rows"`
	Offset    int          `json:"offset"`
	TotalRows int          `json:"total_rows"`
	Limit     int          `json:"-"`
}

// modelizePath inflects a model name
func modelizePath(modelName string) string {
	return strings.Title(inflector.Singularize(modelName))
}

// modelizeContainer extracts a model name from a container for that model
func modelizeContainer(container string) string {
	return strings.Replace(container, strings.Title(dragonfruit.ContainerName), "", -1)
}

// Prep prepares a database to accept API data.
// In this case, it creates a database (conventionally namped after the base
// model that's returned and a design document for querying the docs.
//
// There are two types of views in the design document (although all of this
// is mostly invisible to the front-end):
//
// - query views handle parameters passed via http GETs
//
// - path views handle parameters embedded in a path
//
// the Query method defines access rules and priorities
func (d *Db_backend_couch) Prep(database string,
	resource *dragonfruit.Resource) error {

	vd := viewDoc{}
	id := "_design/core"
	vd.Id = id
	vd.Views = make(map[string]view)
	dbz, err := d.client.EnsureDB(database)
	if err != nil {
		return err
	}

	err = dbz.Get(id, &vd, nil)

	vd.Language = "javascript"

	// well this is ugly...
	for _, api := range resource.Apis {
		for _, operation := range api.Operations {
			if operation.Method == "GET" {
				vd.makePathParamView(api, operation, resource)
				vd.makeQueryParamView(api, operation, resource)
			}

		}
	}
	d.Save(database, id, vd)
	return nil
}

// Add adds a view to a view doc.
func (vd *viewDoc) Add(viewname string, v view) {
	vd.Views[viewname] = v
}

// makeQueryParamView creates views for filter queries (i.e. queries passed
// through GET params)
// TODO - range queries
func (vd *viewDoc) makeQueryParamView(api *dragonfruit.Api,
	op *dragonfruit.Operation,
	resource *dragonfruit.Resource) {

	responseModel := strings.Replace(op.Type, strings.Title(dragonfruit.ContainerName), "", -1)
	model := resource.Models[responseModel]
	for _, param := range op.Parameters {
		if param.ParamType == "query" {
			for propname, prop := range model.Properties {
				if param.Name == propname {
					if prop.Type != "array" {
						viewname := makeQueryViewName(param.Name)
						vw := view{}
						vw.MapFunc = "function(doc){ emit(doc." + propname + ",doc); }"
						vd.Add(viewname, vw)
					}
				}
			}
		}
	}
}

// makePathParamView creates views for values passed through path parameters
func (vd *viewDoc) makePathParamView(api *dragonfruit.Api,
	op *dragonfruit.Operation,
	resource *dragonfruit.Resource) {

	matches := dragonfruit.PathRe.FindAllStringSubmatch(api.Path, -1)
	viewname := makePathViewName(api.Path)

	if len(matches) == 1 {
		// regex voodoo
		paramName := matches[0][4]
		//pathName := matches[0][2]
		vw := view{}
		vw.MapFunc = "function(doc){ emit(doc." + paramName + ",doc); }"
		vd.Add(viewname, vw)
	}
	if len(matches) > 1 {
		vw := view{}
		model := modelizePath(matches[0][2])
		paramName := matches[0][4]
		emit := make([]viewParam, 1)

		emit[0] = viewParam{
			path:         matches[0][2],
			paramname:    paramName,
			paramtype:    "id",
			singlepath:   "doc",
			propertyname: "doc",
		}

		for _, path := range matches[1:] {
			propertyname, property := findPropertyFromPath(model, path[2], resource)
			p := viewParam{
				path:         path[2],
				paramname:    path[4],
				singlepath:   inflector.Singularize(path[2]),
				propertyname: propertyname,
			}
			if path[4] == "pos" {
				p.paramtype = "index"
			} else {
				p.paramtype = "id"
			}
			emit = append(emit, p)
			if property != nil {
				model = modelizeContainer(property.Ref)
			}
		}

		emitholder := make([]string, 0)

		vw.MapFunc = "function(doc){"
		for idx, emitted := range emit[:(len(emit) - 1)] {
			// for the join later

			vw.MapFunc = vw.MapFunc + emitted.propertyname + "." + emit[(idx+1)].propertyname + ".forEach("
			if emit[(idx+1)].paramtype == "index" {
				vw.MapFunc = vw.MapFunc + " function(" + emit[(idx+1)].singlepath + "," + emit[(idx+1)].singlepath + "Index){ "
			} else {
				vw.MapFunc = vw.MapFunc + " function(" + emit[(idx+1)].singlepath + "){ "
			}
		}

		for _, emitted := range emit {
			var curvar string
			if emitted.paramtype == "index" {
				curvar = "(" + emitted.singlepath + "Index).toString()"
			} else {
				curvar = emitted.singlepath + "." + emitted.paramname
			}
			emitholder = append(emitholder, curvar)
		}

		vw.MapFunc = vw.MapFunc + " emit(["

		vw.MapFunc = vw.MapFunc + strings.Join(emitholder, ",")

		vw.MapFunc = vw.MapFunc + "]," + emit[len(emit)-1].singlepath + "); "

		for _, _ = range emit[:(len(emit) - 1)] {
			vw.MapFunc = vw.MapFunc + " } );"
		}

		vw.MapFunc = vw.MapFunc + "} "

		vd.Add(viewname, vw)

	}

}

// makeQueryViewName makes canonical view names for GET queries
func makeQueryViewName(param string) string {
	return "by_query_" + param
}

// makePathViewName makes canonical view names for path parameters
func makePathViewName(path string) string {
	matches := dragonfruit.ViewPathRe.FindAllStringSubmatch(path, -1)
	out := make([]string, 0)

	for _, match := range matches {
		out = append(out, match[2])
	}

	return "by_path_" + strings.Join(out, "_")
}

// findPropertyFromPath introspects a path and returns the property that
// corresponds to it.  So basically looks at something like /doc/1/stuff/3 and
// determines that it's looking for "stuff" with a value of "3" inside a
// document.
func findPropertyFromPath(model string, path string,
	resource *dragonfruit.Resource) (string, *dragonfruit.Property) {

	m, ok := resource.Models[model]
	if ok {
		for k, v := range m.Properties {
			if strings.ToLower(k) == strings.ToLower(path) {
				return k, v
			}
		}
	}
	return "", nil
}

// Connect connects to a couchdb server.
func (d *Db_backend_couch) Connect(url string) error {
	db, err := couchdb.NewClient(url, nil)
	if err != nil {
		return err
	}

	d.client = db
	d.connection = make(chan bool, 1)

	return nil
}

// getDatabaseName returns the database that corresponds to the URL path.
func getDatabaseName(params dragonfruit.QueryParams) (database string) {
	dbp := dragonfruit.GetDbRe.FindStringSubmatch(params.Path)
	database = dbp[1]
	return
}

// Update updates a document
// TODO - partial document updates
func (d *Db_backend_couch) Update(params dragonfruit.QueryParams, fullOverwrite bool) (interface{},
	error) {

	doc, id, err := d.getRootDocument(params)
	if err != nil {
		return doc, err
	}

	pathmap := dragonfruit.PathRe.FindAllStringSubmatch(params.Path, -1)

	//pathmap = pathmap[1:len(pathmap)]

	var v interface{}
	err = json.Unmarshal(params.Body, &v)

	if v == nil {
		return v, err
	}

	var manipulator func(reflect.Value, reflect.Value) (reflect.Value, error)

	if fullOverwrite {
		manipulator = replace
	} else {
		manipulator = partialReplace
	}

	newdoc, partial, err := findSubDoc(pathmap[1:],
		params.PathParams,
		reflect.ValueOf(doc.Value),
		reflect.ValueOf(v),
		manipulator)
	database := getDatabaseName(params)
	_, out, err := d.Save(database, id, newdoc.Interface())
	if err != nil {
		return out, err
	}

	out, err = sanitizeDoc(partial.Interface())

	return out, err

}

func sanitizeDoc(doc interface{}) (interface{}, error) {
	out, err := sanitizeDocInternal(reflect.ValueOf(doc))
	if err != nil {
		return doc, err
	}
	return out.Interface(), nil
}

func sanitizeDocInternal(doc reflect.Value) (reflect.Value, error) {
	switch doc.Kind() {
	default:
		return doc, errors.New("invalid doc type")
	case reflect.Interface:
		return sanitizeDocInternal(doc.Elem())
		break
	case reflect.Map:

		for _, key := range doc.MapKeys() {
			if key.String() == "_id" ||
				key.String() == "_rev" {

				doc.SetMapIndex(key, reflect.ValueOf(nil))
			}
		}
	}
	return doc, nil
}

/* findSubDoc uses reflection to aSADSDhsadvghds what is that burnt toast smell? */
func findSubDoc(pathslice [][]string,
	pathParams map[string]interface{},
	document reflect.Value,
	bodyParams reflect.Value,
	// this would literally be easier in Haskell
	manipulator func(reflect.Value, reflect.Value) (reflect.Value, error)) (reflect.Value, reflect.Value, error) {

	var partial reflect.Value

	if len(pathslice) == 0 {
		switch bodyParams.Type().Kind() {
		default:
			return document, document, errors.New("Body Params must be a map.")
			break
		case reflect.Map:
			outdoc, err := manipulator(document, bodyParams)
			return outdoc, outdoc, err
		}

	} else {
		currKey := pathslice[0][4]
		currItem := reflect.ValueOf(pathslice[0][2])
		switch document.Type().Kind() {
		default:
			//fmt.Printf("unexpected type %T", document[currItem])
			break

		case reflect.Interface:
			// if it's an interface, return the elem
			newdoc, part, err := findSubDoc(pathslice, pathParams, document.Elem(), bodyParams, manipulator)
			if err != nil {
				return document, part, err
			}
			document = newdoc
			partial = part
			break
		case reflect.Map:
			newdoc, part, err := findSubDoc(pathslice, pathParams, document.MapIndex(currItem), bodyParams, manipulator)
			if err != nil {
				return document, part, err
			}
			document.SetMapIndex(currItem, newdoc)
			partial = part
			break
		case reflect.Slice:

			for i := 0; i < document.Len(); i++ {
				d := document.Index(i)
				if d.Kind() == reflect.Interface {
					d = d.Elem()
				}

				switch d.Type().Kind() {
				default:
					break
				case reflect.Map:
					vo := reflect.ValueOf(currKey)
					if d.MapIndex(vo).Elem().String() == pathParams[currKey] {
						newdoc, part, err := findSubDoc(pathslice[1:], pathParams, d, bodyParams, manipulator)
						if err != nil {
							return document, part, err
						}
						document.Index(i).Set(newdoc)
						partial = part
					}
				}

			}
			break

		}

	}

	return document, partial, nil
	//return v, nil
}

func (d *Db_backend_couch) getRootDocument(params dragonfruit.QueryParams) (couchdbRow, string, error) {

	viewpath := dragonfruit.PathRe.FindAllStringSubmatch(params.Path, -1)

	// take the first segment from the update path
	newPath := viewpath[0][0]

	newPathParams := make(map[string]interface{})
	newPathParams[viewpath[0][4]] = params.PathParams[viewpath[0][4]]

	newparams := dragonfruit.QueryParams{
		Path:       newPath,
		PathParams: newPathParams,
	}

	_, result, err := d.queryView(newparams)
	if err != nil {
		return couchdbRow{}, "", err
	}
	if len(result.Rows) == 0 {
		return couchdbRow{}, "", errors.New("not found error")
	}

	row := result.Rows[0]
	id := result.Rows[0].Id

	return row, id, err

}

// Manipulator functions

func replace(original reflect.Value, newDoc reflect.Value) (reflect.Value, error) {
	return newDoc, nil
}

func partialReplace(original reflect.Value, newDoc reflect.Value) (reflect.Value, error) {
	orig := fixStupidInterfaceRefs(original)
	newer := fixStupidInterfaceRefs(newDoc)
	if orig.Kind() != reflect.Map || newer.Kind() != reflect.Map {
		return orig, errors.New("Both document and replacement must be a map")
	}
	for _, field := range newer.MapKeys() {

		orig.SetMapIndex(field, newer.MapIndex(field))
	}
	return orig, nil
}

func fixStupidInterfaceRefs(val reflect.Value) reflect.Value {
	if val.Kind() == reflect.Interface {
		return val.Elem()
	}
	return val
}

// Insert adds a new document to the database
// TODO - Add subdocuments
func (d *Db_backend_couch) Insert(params dragonfruit.QueryParams) (interface{},
	error) {

	database := getDatabaseName(params)
	var document map[string]interface{}
	err := json.Unmarshal(params.Body, &document)
	if err != nil {
		return nil, err
	}
	_, doc, err := d.Save(database, uuid.New(), document)
	if err != nil {
		return doc, err
	}

	out, err := sanitizeDoc(doc)

	return out, err

}

// Remove deletes a document from the database
// TODO - remove subdocuments
func (d *Db_backend_couch) Remove(params dragonfruit.QueryParams) error {
	_, result, err := d.queryView(params)
	if err != nil {
		return err
	}

	if len(result.Rows) == 0 {
		return errors.New("not found error")
	}

	target := result.Rows[0]
	id := target.Id
	database := getDatabaseName(params)
	rev, err := d.client.DB(database).Rev(id)
	return d.Delete(database, id, rev)

}

// Delete removes a document from the database
// this will be made private
func (d *Db_backend_couch) Delete(database string, id string,
	rev string) error {

	_, err := d.client.DB(database).Delete(id, rev)
	return err
}

// Save saves a document to the database.
// This should also be made
func (d *Db_backend_couch) Save(database string,
	documentId string,
	document interface{}) (string, interface{}, error) {

	db, err := d.client.EnsureDB(database)

	rev, err := db.Rev(documentId)

	_, err = db.Put(documentId, document, rev)

	return documentId, document, err
}

// Query queries a view and returns a result
func (d *Db_backend_couch) Query(params dragonfruit.QueryParams) (interface{}, error) {
	num, result, err := d.queryView(params)
	if err != nil {
		return nil, err
	}

	returnType := makeTypeName(params.Path)

	c := dragonfruit.Container{}
	c.Meta.Count = len(result.Rows)
	c.Meta.Total = num
	c.Meta.Offset = result.Offset
	c.Meta.ResponseCode = 200
	c.Meta.ResponseMessage = "Ok."
	c.ContainerType = strings.Title(returnType + strings.Title(dragonfruit.ContainerName))
	for _, row := range result.Rows {
		outRow, err := sanitizeDoc(row.Value)
		if err != nil {
			return c, err
		}
		c.Results = append(c.Results, outRow)
	}

	return c, err
}

// queryView queries a couchDB view and returns the number of results,
// a couchDbResponse object and/or an error object.
//
// Views can either be path views (from path parameters) or query views (from
// GET query parameters).  Since only one view can be applied during any query,
// if more than one parameter is sent during a request, the result set is
// filtered using the filterResultSet function below.
//
// The pickView method selects the appropriate view, based on the inbound
// parameters.
func (d *Db_backend_couch) queryView(params dragonfruit.QueryParams) (int,
	couchDbResponse, error) {

	var (
		result couchDbResponse
		err    error
	)

	limit, offset := setLimitAndOffset(params)

	if limit < 1 {
		return 0, couchDbResponse{}, errors.New("Limit must be greater than 0")
	}

	database := getDatabaseName(params)
	db := d.client.DB(database)

	// map to hold view query options
	opts := make(map[string]interface{})

	viewName, viewExists := d.pickView(params, opts, limit, offset)

	// if we found a view, query it
	if viewExists {
		err = db.View("_design/core", viewName, &result, opts)
	} else {
		// if there is no view, use AllDocs
		// this theoretically shouldn't happen
		opts["include_docs"] = true
		err = db.AllDocs(&result, opts)
	}

	totalResults := result.TotalRows

	// if there are any query params that were not applied using a view,
	// run additional filters on the result set
	if len(params.QueryParams) > 0 {
		totalResults, result, err = filterResultSet(result, params, limit, offset)
	}

	return totalResults, result, err
}

// setLimitAndOffset parses limit and offset queries from a set of query params
func setLimitAndOffset(params dragonfruit.QueryParams) (limit int64,
	offset int64) {

	limit, offset = 10, 0

	l := params.QueryParams.Get("limit")

	if l != "" {
		num, err := strconv.ParseInt(l, 10, 0)
		if err == nil {
			limit = num
		}
		params.QueryParams.Del("limit")
	}

	o := params.QueryParams.Get("offset")
	if o != "" {
		num, err := strconv.ParseInt(o, 10, 0)
		if err == nil {
			offset = num
		}
		params.QueryParams.Del("offset")
	}

	return
}

// filsterResultSet applys a set of filters (GET query params basically) to a
// result set after it is loaded from an initial view (since the CouchDB views
// created by Prep aren't set up to filter with more than one parameter)
//
// After a result set is returned from a view,
func filterResultSet(result couchDbResponse, params dragonfruit.QueryParams,
	limit int64, offset int64) (int, couchDbResponse, error) {

	if len(params.QueryParams) < 1 {
		return len(result.Rows), result, nil
	}
	outResult := result

	outResult.Rows = make([]couchdbRow, 0)
	for _, v := range result.Rows {
		for queryParam := range params.QueryParams {

			val, ok := v.Value[queryParam]
			if ok && (params.QueryParams.Get(queryParam) == val) {
				/*switch val.(type) {}*/

				outResult.Rows = append(outResult.Rows, v)
			}
		}
	}
	totalNum := len(outResult.Rows)
	if int(offset) > totalNum {
		outResult.Rows = make([]couchdbRow, 0, 0)
	} else if int(limit+offset) > len(outResult.Rows) {
		outResult.Rows = outResult.Rows[offset:len(outResult.Rows)]
	} else {
		outResult.Rows = outResult.Rows[offset:(offset + limit)]
	}

	return totalNum, outResult, nil
}

func (d *Db_backend_couch) ensureConnection() (err error) {
	defer func() {
		<-d.connection
	}()

	// only do this stuff if no one
	d.connection <- true
	err = d.client.Ping()
	if err == nil {
		return
	}

	_, err = exec.Command("couchdb", "-b").Output()
	if err != nil {
		return err
	}

	var s func() error
	s = func() error {
		var err error
		fmt.Println("Waiting for couchdb to start...")
		s_out, err := exec.Command("couchdb", "-s").CombinedOutput()
		if err != nil {
			fmt.Println("Launch error: ", err, "please send this to Peter O.")
		}
		if bytes.Contains(s_out, []byte("Apache CouchDB is running as process")) {
			time.Sleep(1000 * time.Millisecond)
			return err
		}
		if bytes.Contains(s_out, []byte("Apache CouchDB is not running.")) {
			time.Sleep(1000 * time.Millisecond)
			return s()
		}

		if err != nil {
			return err
		}

		fmt.Println("So this happened: ", string(s_out), "... Please send this to Peter O.")
		return errors.New(string(s_out))

	}

	err = s()

	return
}

// Load loads a document from the database.
// TODO - THIS WILL PROBABLY MOVE TO A NON-EXPORTED METHOD
func (d *Db_backend_couch) Load(database string, documentId string, doc interface{}) error {
	d.ensureConnection()
	db, err := d.client.EnsureDB(database)
	if err != nil {
		return err
	}

	// mutate the doc
	err = db.Get(documentId, doc, nil)
	if err != nil {
		return err
	}

	return err
}

// pickView looks at the possible CouchDB views that could be queried and
// determines which view actually gets used to generate an initial result set.
//
// The opts parameter gets mutated - be careful.
//
// It returns a string view name and a boolean value to indicate whether
// a view was found at not.
func (d *Db_backend_couch) pickView(params dragonfruit.QueryParams,
	opts map[string]interface{},
	limit int64,
	offset int64) (string, bool) {

	viewName := makePathViewName(params.Path)

	// if there's no query parameters to filter, you can go
	// ahead and use the passed limit and offset
	// and apply it during the query to the view
	// otherwise it has to be applied during the filter step
	if len(params.QueryParams) == 0 {
		opts["limit"] = limit
		opts["skip"] = offset
	}

	// if there aren't any path params (e.g. /paramname/{value}), use a query
	// view.
	if len(params.PathParams) == 0 {
		if len(params.QueryParams) > 0 {
			queryView, found := d.findQueryView(params, opts)
			if found {

				// since params.QueryParams has now been mutated,
				// re-check to see if you need to add the
				// limit and offset
				if len(params.QueryParams) == 0 {
					opts["limit"] = limit
					opts["skip"] = offset
				}

				return queryView, true
			}
		}

		return viewName, true
	}

	viewMatches := dragonfruit.ViewPathRe.FindAllStringSubmatch(params.Path, -1)
	// use a non-array key
	if (len(params.PathParams) == 1) && (len(viewMatches) == 1) {
		// ugh i know...
		for _, v := range params.PathParams {
			opts["key"] = v
		}

		return viewName, true
	}

	ok := true

	// I wish golang had functional slice functions...
	for _, vm := range viewMatches {
		if vm[3] == "" {
			ok = false
		}
	}

	key := make([]interface{}, 0)
	for _, param := range params.PathParams {

		key = append(key, param)
	}

	if ok {
		opts["key"] = key
	} else {
		opts["startkey"] = key
		//opts["endkey"] = append(key, "{}")
	}
	return viewName, true

}

// findQueryView selects a view from the design document to query with the
// passed QueryParams map.  This mutates both the opts and params maps.
//
// If it finds a param to query, it removes that query from the QueryParams map
// and adds options to the opts map.
//
// It returns a boolean to indicate the view was found and the name of the
// view to use.
func (d *Db_backend_couch) findQueryView(params dragonfruit.QueryParams,
	opts map[string]interface{}) (string, bool) {

	var vd viewDoc
	//d.L
	database := getDatabaseName(params)
	err := d.Load(database, "_design/core", &vd)
	if err != nil {
		panic(err)
	}

	// iterate over the passed queryParams map
	for queryParam, queryValue := range params.QueryParams {
		isRange := strings.Contains(queryParam, "Range")
		if isRange {
			queryParam = strings.Replace(queryParam, "Range", "", -1)
		}
		_, exists := vd.Views[makeQueryViewName(queryParam)]
		if exists {
			if isRange {
				opts["startKey"] = queryValue[0]
				opts["endKey"] = queryValue[(len(queryValue) - 1)]
			} else {
				opts["key"] = params.QueryParams.Get(queryParam)
			}
			params.QueryParams.Del(queryParam)
			return makeQueryViewName(queryParam), true
		}
	}
	return "", false
}

// makeTypeName returns a content type from path parameters.
func makeTypeName(path string) string {
	matches := dragonfruit.ViewPathRe.FindAllStringSubmatch(path, -1)
	return inflector.Singularize(matches[(len(matches) - 1)][2])
}
