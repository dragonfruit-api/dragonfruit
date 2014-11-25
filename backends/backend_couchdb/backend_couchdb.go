package backend_couchdb

import (
	//"fmt"
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"errors"
	//"fmt"
	"github.com/fjl/go-couchdb"
	"github.com/gedex/inflector"
	"github.com/ideo/dragonfruit"
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
}

func modelizePath(modelName string) string {
	return strings.Title(inflector.Singularize(modelName))
}

func modelizeContainer(container string) string {
	return strings.Replace(container, strings.Title(dragonfruit.ContainerName), "", -1)
}

func (d *Db_backend_couch) Prep(database string, resource *dragonfruit.Resource) {
	vd := viewDoc{}
	id := "_design/core"
	vd.Id = id
	vd.Views = make(map[string]view)
	dbz, err := d.client.EnsureDB(database)
	if err != nil {
		// do something here
	}

	err = dbz.Get(id, &vd, nil)

	vd.Language = "javascript"

	// well this is ugly...
	for _, api := range resource.Apis {
		for _, operation := range api.Operations {
			if operation.Method == "GET" {
				vd.makePathParamView(api, operation, resource)
			}

		}
	}
	d.Save(database, id, vd)
}

func (vd *viewDoc) makePathParamView(api *dragonfruit.Api, op *dragonfruit.Operation, resource *dragonfruit.Resource) {
	matches := dragonfruit.PathRe.FindAllStringSubmatch(api.Path, -1)
	viewname := makePathViewName(api.Path)
	//fmt.Println("matches: ", api.Path, len(matches), matches)

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
			//fmt.Println("searching for:", model, path[2])
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
	result, err := d.queryView(params)
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
	result, err := d.queryView(params)
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
	//fmt.Println("reverr", rev, err)
	/*if err != nil {
		fmt.Println("this is the error", err, rev)
		return "", nil, err
	}*/

	_, err = db.Put(documentId, document, rev)
	//fmt.Println("save error: ", err)

	return documentId, document, err
}

func (d *Db_backend_couch) Query(params dragonfruit.QueryParams) (interface{}, error) {

	result, err := d.queryView(params)

	returnType := makeTypeName(params.Path)

	c := dragonfruit.Container{}
	c.Meta.Count = len(result.Rows)
	c.Meta.Total = result.TotalRows
	c.Meta.Offset = result.Offset
	c.Meta.ResponseCode = 200
	c.Meta.ResponseMessage = "Ok."
	c.Type = strings.Title(returnType + strings.Title(dragonfruit.ContainerName))
	for _, row := range result.Rows {
		c.Results = append(c.Results, row.Value)
	}

	return c, err
}

func (d *Db_backend_couch) queryView(params dragonfruit.QueryParams) (couchDbResponse, error) {
	var (
		result couchDbResponse
		err    error
	)

	dbp := dragonfruit.GetDbRe.FindStringSubmatch(params.Path)

	database := dbp[1]

	db := d.client.DB(database)
	opts := make(map[string]interface{})
	viewName, ok := pathView(params, opts)

	if ok {
		err = db.View("_design/core", viewName, &result, opts)
	} else {
		opts["include_docs"] = true
		err = db.AllDocs(&result, opts)
		//fmt.Println(result)
	}
	return result, err
}

func (d *Db_backend_couch) Load(database string, documentId string, doc interface{}) error {
	//fmt.Println(database)
	db, err := d.client.EnsureDB(database)
	if err != nil {
		return err
	}

	//var doc interface{}
	// mutate the doc
	err = db.Get(documentId, doc, nil)
	if err != nil {
		//fmt.Println("~~~~~~~~~~~~~~~~~~~", err)
	}

	return err
}

func (vd *viewDoc) Add(viewname string, v view) {
	vd.Views[viewname] = v
}

// this MUTATES the opts parameter, be careful
func pathView(params dragonfruit.QueryParams, opts map[string]interface{}) (string, bool) {
	// use all docs or a view query

	viewName := makePathViewName(params.Path)
	viewMatches := dragonfruit.ViewPathRe.FindAllStringSubmatch(params.Path, -1)
	if len(params.PathParams) == 0 {

		return viewName, true
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

	//matches := pathRe.FindAllStringSubmatch(params.path, -1)
	key := make([]interface{}, 0)
	//	fmt.Println(params.PathParams)
	for _, param := range params.PathParams {
		//		fmt.Println(param, 0, "0")

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

// just don't ask....
func makeTypeName(path string) string {
	matches := dragonfruit.ViewPathRe.FindAllStringSubmatch(path, -1)
	return inflector.Singularize(matches[(len(matches) - 1)][2])
}
