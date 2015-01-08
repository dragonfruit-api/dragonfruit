package backend_couchdb

import (
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"errors"
	"github.com/fjl/go-couchdb"
	"github.com/gedex/inflector"
	"github.com/ideo/dragonfruit"
	"strconv"
	"strings"
)

type view struct {
	MapFunc    string `json:"map"`
	ReduceFunc string `json:"reduce,omitempty"`
}

type viewDoc struct {
	Id       string          `json:"_id"`
	Rev      string          `json:"_rev,omitempty"`
	Language string          `json:"language"`
	Views    map[string]view `json:"views"`
}

type Db_backend_couch struct {
	client *couchdb.Client
}

type viewParam struct {
	path         string
	singlepath   string
	paramname    string
	paramtype    string
	propertyname string
}

type couchdbRow struct {
	Doc   map[string]interface{} `json:"doc,omitempty"`
	Id    string                 `json:"id"`
	Key   interface{}            `json:"key"`
	Value map[string]interface{} `json:"value"`
}

type couchDbResponse struct {
	Rows      []couchdbRow `json:"rows"`
	Offset    int          `json:"offset"`
	TotalRows int          `json:"total_rows"`
	Limit     int          `json:"-"`
}

func modelizePath(modelName string) string {
	return strings.Title(inflector.Singularize(modelName))
}

func modelizeContainer(container string) string {
	return strings.Replace(container, strings.Title(dragonfruit.ContainerName), "", -1)
}

func (d *Db_backend_couch) Prep(database string, resource *dragonfruit.Resource) error {
	vd := viewDoc{}
	id := "_design/core"
	vd.Id = id
	vd.Views = make(map[string]view)
	dbz, err := d.client.EnsureDB(database)
	if err != nil {
		// do something here
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

func makeQueryViewName(param string) string {
	return "by_query_" + param
}

func makePathViewName(path string) string {
	matches := dragonfruit.ViewPathRe.FindAllStringSubmatch(path, -1)
	out := make([]string, 0)

	for _, match := range matches {
		out = append(out, match[2])
		//out = append(out, v.paramname)
	}

	return "by_path_" + strings.Join(out, "_")

}

func findPropertyFromPath(model string, path string, resource *dragonfruit.Resource) (string, *dragonfruit.Property) {
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

func (d *Db_backend_couch) Connect(url string) error {
	db, err := couchdb.NewClient(url, nil)
	if err != nil {
		return err
	}

	d.client = db

	return nil
}

func getDatabaseName(params dragonfruit.QueryParams) (database string) {
	dbp := dragonfruit.GetDbRe.FindStringSubmatch(params.Path)
	database = dbp[1]
	return
}

func (d *Db_backend_couch) Update(params dragonfruit.QueryParams) (interface{}, error) {
	database := getDatabaseName(params)
	_, result, err := d.queryView(params)
	if err != nil {
		return nil, err
	}

	if len(result.Rows) == 0 {
		return nil, errors.New("not found error")
	}

	var document map[string]interface{}
	err = json.Unmarshal(params.Body, &document)
	if err != nil {
		return nil, err
	}
	documentId := result.Rows[0].Id

	_, out, err := d.Save(database, documentId, document)

	return out, err

}

func (d *Db_backend_couch) Insert(params dragonfruit.QueryParams) (interface{}, error) {
	database := getDatabaseName(params)
	var document map[string]interface{}
	err := json.Unmarshal(params.Body, &document)
	if err != nil {
		return nil, err
	}
	_, doc, err := d.Save(database, uuid.New(), document)

	return doc, err

}

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

func (d *Db_backend_couch) Delete(database string, id string, rev string) error {
	_, err := d.client.DB(database).Delete(id, rev)
	return err
}

func (d *Db_backend_couch) Save(database string,
	documentId string,
	document interface{}) (string, interface{}, error) {

	db, err := d.client.EnsureDB(database)

	rev, err := db.Rev(documentId)

	_, err = db.Put(documentId, document, rev)

	return documentId, document, err
}

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
		c.Results = append(c.Results, row.Value)
	}

	return c, err
}

func (d *Db_backend_couch) queryView(params dragonfruit.QueryParams) (int, couchDbResponse, error) {
	var (
		result couchDbResponse
		err    error
	)
	opts := make(map[string]interface{})

	database := getDatabaseName(params)

	limit, offset := setLimitAndOffset(params)

	if limit < 1 {
		return 0, couchDbResponse{}, errors.New("Limit must be greater than 0")
	}

	db := d.client.DB(database)
	viewName, ok := d.pickView(params, opts, limit, offset)

	if ok {
		err = db.View("_design/core", viewName, &result, opts)
	} else {
		opts["include_docs"] = true
		err = db.AllDocs(&result, opts)
	}

	totalResults := result.TotalRows

	if len(params.QueryParams) > 0 {
		totalResults, result, err = filterResultSet(result, params, limit, offset)
	}

	return totalResults, result, err
}

func setLimitAndOffset(params dragonfruit.QueryParams) (limit int64, offset int64) {
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

func filterResultSet(result couchDbResponse,
	params dragonfruit.QueryParams,
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

func (d *Db_backend_couch) Load(database string, documentId string, doc interface{}) error {
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

func (vd *viewDoc) Add(viewname string, v view) {
	vd.Views[viewname] = v
}

// this MUTATES the opts parameter, be careful
func (d *Db_backend_couch) pickView(params dragonfruit.QueryParams,
	opts map[string]interface{},
	limit int64,
	offset int64) (string, bool) {
	// use all docs or a view query

	viewName := makePathViewName(params.Path)
	viewMatches := dragonfruit.ViewPathRe.FindAllStringSubmatch(params.Path, -1)
	if len(params.PathParams) == 0 {
		if len(params.QueryParams) > 0 {
			queryView, ok := d.findQueryView(params, opts)
			if ok {
				if len(params.QueryParams) == 0 {
					opts["limit"] = limit
					opts["skip"] = offset
				}
				return queryView, true
			}
		}
		opts["limit"] = limit
		opts["skip"] = offset

		return viewName, true
	}

	if len(params.QueryParams) == 0 {
		opts["limit"] = limit
		opts["skip"] = offset
	}

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

// this also mutates opts
func (d *Db_backend_couch) findQueryView(params dragonfruit.QueryParams, opts map[string]interface{}) (string, bool) {
	var vd viewDoc
	//d.L
	database := getDatabaseName(params)
	err := d.Load(database, "_design/core", &vd)
	if err != nil {
		panic(err)
	}
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

// just don't ask....
func makeTypeName(path string) string {
	matches := dragonfruit.ViewPathRe.FindAllStringSubmatch(path, -1)
	return inflector.Singularize(matches[(len(matches) - 1)][2])
}
