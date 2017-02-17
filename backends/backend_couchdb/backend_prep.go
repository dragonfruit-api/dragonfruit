package backend_couchdb

import (
	"github.com/gedex/inflector"
	"github.com/ideo/dragonfruit"
	"strings"
)

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
	resource *dragonfruit.Swagger) error {

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
	for path, api := range resource.Paths {
		if strings.HasPrefix(path, "/"+database) {
			vd.makePathParamView(api, path, api.Get, resource)
			vd.makeQueryParamView(api, api.Get, resource)
		}
	}
	d.save(database, id, vd)
	return nil
}

// Add adds a view to a view doc.
func (vd *viewDoc) add(viewname string, v view) {
	vd.Views[viewname] = v
}

// makeQueryParamView creates views for filter queries (i.e. queries passed
// through GET params)
// TODO - range queries
func (vd *viewDoc) makeQueryParamView(
	api *dragonfruit.PathItem,
	op *dragonfruit.Operation,
	resource *dragonfruit.Swagger) {

	modelName := dragonfruit.DeRef(op.Responses["200"].Schema.Ref)
	responseModel := strings.Replace(modelName, strings.Title(dragonfruit.ContainerName), "", -1)

	model := resource.Definitions[responseModel]
	for _, param := range op.Parameters {
		if param.In == "query" {
			for propname, prop := range model.Properties {
				if param.Name == propname {
					if prop.Type != "array" {
						viewname := makeQueryViewName(param.Name)
						vw := view{}
						vw.MapFunc = "function(doc){ emit(doc." + propname + ", doc); }"
						vd.add(viewname, vw)
					}
				}
			}
		}
	}
}

// makePathParamView creates views for values passed through path parameters
func (vd *viewDoc) makePathParamView(api *dragonfruit.PathItem,
	path string,
	op *dragonfruit.Operation,
	resource *dragonfruit.Swagger) {

	matches := dragonfruit.PathRe.FindAllStringSubmatch(path, -1)
	tpath := dragonfruit.TranslatePath(path)
	viewname := makePathViewName(tpath)

	if len(matches) == 1 {
		// regex voodoo
		paramName := matches[0][4]
		//pathName := matches[0][2]
		vw := view{}
		vw.MapFunc = "function(doc){ emit(doc." + paramName + ", doc); }"
		vd.add(viewname, vw)
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

			vw.MapFunc = vw.MapFunc + " function(" + emit[(idx+1)].singlepath + "){ "

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

		vd.add(viewname, vw)

	}

}
