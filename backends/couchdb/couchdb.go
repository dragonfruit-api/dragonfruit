package couchdb

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.com/dragonfruit-api/dragonfruit"
	"github.com/fjl/go-couchdb"

	"github.com/gedex/inflector"
	"github.com/pborman/uuid"
)

const (
	SwaggerResourceDB       = "swagger_docs"
	ResourceDescriptionName = "swagger_resource"
)

// findPropertyFromPath introspects a path and returns the property that
// corresponds to it.  So basically looks at something like /doc/1/stuff/3 and
// determines that it's looking for "stuff" with a value of "3" inside a
// document.
func findPropertyFromPath(model string, path string,
	resource *dragonfruit.Swagger) (string, *dragonfruit.Schema) {
	p := strings.ToLower(inflector.Singularize(path))
	m, ok := resource.Definitions[model]
	if ok {
		for k, v := range m.Properties {
			if strings.ToLower(k) == p {
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

/* This function is rediculous */
func (d *Db_backend_couch) getPathSpecificStuff(params dragonfruit.QueryParams) ([][]string,
	couchdbRow, string, interface{}, error) {
	var v interface{}

	pathmap := dragonfruit.PathParamRe.FindAllStringSubmatch(params.Path, -1)

	doc, id, err := d.getRootDocument(params)

	if err != nil {
		return pathmap, doc, id, v, err
	}

	// unwrap the body of the document into an interface
	if len(params.Body) > 0 {
		err = json.Unmarshal(params.Body, &v)
	}

	if v == nil {
		return pathmap, doc, id, v, err
	}

	return pathmap, doc, id, v, nil
}

// Update updates a document
// TODO - partial document updates
func (d *Db_backend_couch) Update(params dragonfruit.QueryParams, operation int) (interface{},
	error) {

	pathmap, doc, id, v, err := d.getPathSpecificStuff(params)

	if err != nil {
		return nil, err
	}

	newdoc, partial, err := findSubDoc(pathmap[1:],
		params,
		reflect.ValueOf(doc.Value),
		reflect.ValueOf(v),
		operation)
	if err != nil {
		return nil, err
	}

	database := getDatabaseName(params)
	_, out, err := d.save(database, id, newdoc.Interface())
	if err != nil {
		return out, err
	}
	out, err = sanitizeDoc(partial.Interface())

	return out, err

}

/* findSubDoc uses reflection to aSADSDhsadvghds what is that burnt toast smell? */
func findSubDoc(pathslice [][]string,
	params dragonfruit.QueryParams,
	document reflect.Value,
	bodyParams reflect.Value,
	operation int) (reflect.Value, reflect.Value, error) {

	var partial reflect.Value

	if len(pathslice) == 0 && (operation == dragonfruit.PUT || operation == dragonfruit.PATCH) {
		switch bodyParams.Type().Kind() {
		default:
			return document, document, errors.New("body params must be a map")

		case reflect.Map:

			var manipulator func(reflect.Value, reflect.Value) (reflect.Value, error)

			if operation == dragonfruit.PUT {
				manipulator = replace
			} else {
				manipulator = partialReplace
			}
			outdoc, err := manipulator(document, bodyParams)
			return outdoc, outdoc, err
		}

	} else {
		var currKey reflect.Value
		var currItem reflect.Value
		if len(pathslice) > 0 {

			currKey = reflect.ValueOf(pathslice[0][4])
			currItem = reflect.ValueOf(inflector.Singularize(pathslice[0][2]))
		} else {
			endOfPath := dragonfruit.EndOfPathRe.FindStringSubmatch(params.Path)
			currItem = reflect.ValueOf(endOfPath[0])
		}

		switch document.Type().Kind() {
		default:
			break

		case reflect.Interface:
			// if it's an interface, return the elem
			newdoc, part, err := findSubDoc(pathslice, params, document.Elem(), bodyParams, operation)
			if err != nil {
				return document, part, err
			}
			document = newdoc
			partial = part
		case reflect.Map:
			newdoc, part, err := findSubDoc(pathslice, params, document.MapIndex(currItem), bodyParams, operation)
			if err != nil {
				return document, part, err
			}
			document.SetMapIndex(currItem, newdoc)
			partial = part
		case reflect.Slice:
			// the cyclomatic complexity is too damn high...

			// for posts, add an element to the slice
			if (operation == dragonfruit.POST) && (len(pathslice) == 1) {
				newDoc := reflect.Append(document, bodyParams)
				return newDoc, bodyParams, nil
			}

			for i := 0; i < document.Len(); i++ {
				d := document.Index(i)
				if d.Kind() == reflect.Interface {
					d = d.Elem()
				}

				switch d.Type().Kind() {
				default:
					break
				case reflect.Map:

					if matchInterfaceKeys(d.MapIndex(currKey).Elem(), params.PathParams[currKey.String()]) {
						if operation == dragonfruit.DELETE {
							if i == 0 {
								document = document.Slice(1, document.Len())
							} else if (i + 1) == document.Len() {
								document = document.Slice(0, i)
							} else {
								document = reflect.AppendSlice(document.Slice(0, i), document.Slice(i+1, document.Len()))

							}
							partial = reflect.ValueOf(nil)
						} else {
							newdoc, part, err := findSubDoc(pathslice[1:], params, d, bodyParams, operation)
							if err != nil {
								return document, part, err
							}
							document.Index(i).Set(newdoc)
							partial = part
						}

					}
				}

			}

		}

	}

	return document, partial, nil
}

func (d *Db_backend_couch) getRootDocument(params dragonfruit.QueryParams) (couchdbRow, string, error) {

	viewpath := dragonfruit.ViewPathParamRe.FindAllStringSubmatch(params.Path, -1)
	// take the first segment from the update path
	newPath := viewpath[0][0]

	// make a new
	newPathParams := make(map[string]interface{})
	// extract
	newPathParams[viewpath[0][3]] = params.PathParams[viewpath[0][3]]

	newparams := dragonfruit.QueryParams{
		Path:       newPath,
		PathParams: newPathParams,
	}

	_, result, err := d.queryView(newparams)
	if err != nil {
		return couchdbRow{}, "", err
	}
	if len(result.Rows) == 0 {
		return couchdbRow{}, "", errors.New(dragonfruit.NOTFOUNDERROR)
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

	var doc interface{}
	// if there are no path parameters, this is a new primary document
	// just save it
	if len(params.PathParams) == 0 {
		docID := uuid.New()
		_, doc, err = d.save(database, docID, document)
	} else {
		pathmap, couchdoc, id, newDoc, err := d.getPathSpecificStuff(params)
		if err != nil {
			return doc, err
		}
		docVal, partialVal, err := findSubDoc(pathmap[1:],
			params,
			reflect.ValueOf(couchdoc.Value),
			reflect.ValueOf(newDoc),
			dragonfruit.POST)
		if err != nil {
			return nil, err
		}
		_, _, err = d.save(database, id, docVal.Interface())
		if err != nil {
			return nil, err
		}
		doc = partialVal.Interface()
	}

	if err != nil {
		return doc, err
	}

	out, err := sanitizeDoc(doc)

	return out, err
}

// LoadDescriptionFromDb loads a resource description from a database backend.
// (see the backend stuff and types).
func (d *Db_backend_couch) LoadDefinition(cnf dragonfruit.Conf) (*dragonfruit.Swagger, error) {

	rd := &dragonfruit.Swagger{}
	err := d.load(SwaggerResourceDB, ResourceDescriptionName, rd)

	if err != nil {
		//TODO - fix this stupid shadowing issue
		rd = cnf.SwaggerTemplate
		return rd, nil
	}
	return rd, err
}

// LoadDescriptionFromDb loads a resource description from a database backend.
// (see the backend stuff and types).
func (d *Db_backend_couch) SaveDefinition(sw *dragonfruit.Swagger) error {

	_, _, err := d.save(SwaggerResourceDB, ResourceDescriptionName, sw)

	return err
}

// Remove deletes a document from the database
func (d *Db_backend_couch) Remove(params dragonfruit.QueryParams) error {
	database := getDatabaseName(params)
	if len(params.PathParams) == 1 {
		_, result, err := d.queryView(params)
		if err != nil {
			return err
		}

		if len(result.Rows) == 0 {
			return errors.New(dragonfruit.NOTFOUNDERROR)
		}

		target := result.Rows[0]
		id := target.Id
		err = d.ensureConnection()
		if err != nil {
			return err
		}
		rev, err := d.client.DB(database).Rev(id)
		if err != nil {
			return err
		}

		return d.delete(database, id, rev)
	} else {
		pathmap, couchdoc, id, newDoc, err := d.getPathSpecificStuff(params)
		if err != nil {
			return err
		}

		docVal, _, err := findSubDoc(pathmap[1:],
			params,
			reflect.ValueOf(couchdoc.Value),
			reflect.ValueOf(newDoc),
			dragonfruit.DELETE)

		if err != nil {
			return err
		}

		_, _, err = d.save(database, id, docVal.Interface())

		return err
	}

}

// Delete removes a document from the database
// this will be made private
func (d *Db_backend_couch) delete(database string, id string,
	rev string) error {
	_, err := d.client.DB(database).Delete(id, rev)
	return err
}

// Save saves a document to the database.
// This should also be made
func (d *Db_backend_couch) save(database string,
	documentId string,
	document interface{}) (string, interface{}, error) {
	err := d.ensureConnection()
	if err != nil {
		return "", nil, err
	}

	db, err := d.client.EnsureDB(database)
	if err != nil {
		return "", nil, err
	}

	rev, err := db.Rev(documentId)
	if err != nil {
		return "", nil, err
	}

	_, err = db.Put(documentId, document, rev)
	if err != nil {
		return "", nil, err
	}

	return documentId, document, err
}

// Query queries a view and returns a result
func (d *Db_backend_couch) Query(params dragonfruit.QueryParams) (dragonfruit.Container, error) {

	num, result, err := d.queryView(params)
	if err != nil {
		return dragonfruit.Container{}, err
	}

	returnType := makeTypeName(params.Path)

	c := dragonfruit.Container{}
	c.Meta.Count = len(result.Rows)
	c.Meta.Total = num
	c.Meta.Offset = result.Offset
	c.Meta.ResponseCode = 200
	c.Meta.ResponseMessage = "Ok."
	c.ContainerType = strings.Title(returnType + strings.Title(dragonfruit.ContainerName))
	c.Results = make([]interface{}, 0)
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

	err = d.ensureConnection()
	if err != nil {
		return 0, couchDbResponse{}, err
	}

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
	result.Offset = offset

	totalResults := result.TotalRows

	// if there are any query params that were not applied using a view,
	// run additional filters on the result set
	if len(params.QueryParams) > 0 {
		totalResults, result, err = filterResultSet(result, params, limit, offset)
	}

	return totalResults, result, err
}

// setLimitAndOffset parses limit and offset queries from a set of query params
func setLimitAndOffset(params dragonfruit.QueryParams) (limit int,
	offset int) {

	limit, offset = 10, 0

	l := params.QueryParams.Get("limit")

	if l != "" {
		switch l := l.(type) {
		case int64:
			limit = int(l)
		case int:
			limit = l
		}

		params.QueryParams.Del("limit")
	}

	o := params.QueryParams.Get("offset")
	if o != "" {
		switch o := o.(type) {
		case int64:
			offset = int(o)
		case int:
			offset = o
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
	limit int, offset int) (int, couchDbResponse, error) {

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
		outResult.Rows = make([]couchdbRow, 0)
	} else if int(limit+offset) > len(outResult.Rows) {
		outResult.Rows = outResult.Rows[offset:len(outResult.Rows)]
	} else {
		outResult.Rows = outResult.Rows[offset:(offset + limit)]
	}

	return totalNum, outResult, nil
}

// Load loads a document from the database.
func (d *Db_backend_couch) load(database string, documentID string, doc interface{}) error {
	err := d.ensureConnection()
	if err != nil {
		return err
	}
	db, err := d.client.EnsureDB(database)
	if err != nil {
		return err
	}

	// mutate the doc
	err = db.Get(documentID, doc, nil)
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
	limit int,
	offset int) (string, bool) {

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

	viewMatches := dragonfruit.PathParamRe.FindAllStringSubmatch(params.Path, -1)
	// for
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
	pathOrder := dragonfruit.ViewPathParamRe.FindAllStringSubmatch(params.Path, -1)
	for _, pathElement := range pathOrder {
		if pathElement[3] != "" {
			param := params.PathParams[pathElement[3]]
			key = append(key, param)
		}

	}
	if ok {
		opts["key"] = key
	} else {
		tmpMap := make(map[string]interface{})
		opts["startkey"] = key
		opts["endkey"] = append(key, tmpMap)
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
	err := d.load(database, "_design/core", &vd)
	if err != nil {
		panic(err)
	}

	// iterate over the passed queryParams map
	for queryParam, _ := range params.QueryParams {
		var q string
		var startKey interface{}
		var endKey interface{}
		var searchKey string
		var searchValue interface{}
		reverseSearch := false

		rangeStart := strings.Contains(queryParam, dragonfruit.RANGESTART)
		rangeEnd := strings.Contains(queryParam, dragonfruit.RANGEEND)

		// ugh ...
		// TODO - refactor so it's not embarassing...
		if rangeStart {
			q = strings.Replace(queryParam, dragonfruit.RANGESTART, "", -1)
			startKey = params.QueryParams.Get(queryParam)
			searchKey = q + dragonfruit.RANGEEND

			searchValue = params.QueryParams.Get(searchKey)
			strSearch, empty := searchValue.(string)

			if empty && (len(strSearch) == 0) {
				endKey = make(map[string]string)
			} else {
				endKey = searchValue
			}

		} else if rangeEnd {
			q = strings.Replace(queryParam, dragonfruit.RANGEEND, "", -1)

			searchKey = q + dragonfruit.RANGESTART

			searchValue = params.QueryParams.Get(searchKey)
			strSearch, empty := searchValue.(string)

			if empty && (len(strSearch) == 0) {
				startKey = params.QueryParams.Get(queryParam)
				reverseSearch = true
			} else {
				endKey = params.QueryParams.Get(queryParam)
				startKey = searchValue
			}
		} else {
			q = queryParam
		}

		_, exists := vd.Views[makeQueryViewName(q)]
		if exists {
			if rangeEnd || rangeStart {
				opts["startkey"] = startKey
				opts["endkey"] = endKey
				if reverseSearch {
					opts["descending"] = true
				}
				params.QueryParams.Del(queryParam)
				params.QueryParams.Del(searchKey)
			} else {
				opts["key"] = params.QueryParams.Get(q)
				params.QueryParams.Del(q)

			}
			return makeQueryViewName(q), true
		}
	}
	return "", false

}

func matchInterfaceKeys(needle reflect.Value, haystack interface{}) bool {
	switch t := haystack.(type) {
	case string:
		return needle.String() == t
	case int64:
		switch needle.Kind() {
		case reflect.Int:
			return needle.Int() == t
		case reflect.String:
			st := needle.String()
			outst, err := strconv.ParseInt(st, 10, 64)
			if err != nil {
				return false
			}
			return outst == t
		case reflect.Float32:
		case reflect.Float64:
			ft := needle.Float()
			return int64(ft) == t

		}

	}
	return false
}
