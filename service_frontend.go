package dragonfruit

import (
	"encoding/json"
	"github.com/go-martini/martini"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var (
	PathRe      *regexp.Regexp
	ReqPathRe   *regexp.Regexp
	GetDbRe     *regexp.Regexp
	ViewPathRe  *regexp.Regexp
	EndOfPathRe *regexp.Regexp
	PostPathRe  *regexp.Regexp
	m           *martini.ClassicMartini
)

const (
	GET = iota
	PUT
	PATCH
	POST
	DELETE
)

// init sets up some basic regular expressions used by the frontend and
// builds the base Martini instance.
func init() {
	PathRe = regexp.MustCompile("(/([[:word:]]*)/({([[:word:]]*)}))")
	ReqPathRe = regexp.MustCompile("(/([[:word:]]*)/([[:word:]]*))")
	GetDbRe = regexp.MustCompile("^/([[:word:]]*)/?")
	ViewPathRe = regexp.MustCompile("(/([[:word:]]*)(/{[[:word:]]*})?)")
	EndOfPathRe = regexp.MustCompile("[^/]+$")
	m = martini.Classic()
}

// GetMartiniInstance returns a Martini instance (so that it can be used by
// a larger Martini app)
func GetMartiniInstance(cnf Conf) *martini.ClassicMartini {
	//
	return m
}

// ServeDocSet sets up the paths which serve the api documentation
func ServeDocSet(db Db_backend, cnf Conf) {
	m.Map(db)
	rd, err := LoadDescriptionFromDb(db, cnf)
	if err != nil {
		panic(err)
	}

	m.Get("/api-docs", func(res http.ResponseWriter) (int, string) {
		h := res.Header()

		h.Add("Content-Type", "application/json;charset=utf-8")
		docs, err := json.Marshal(rd)
		if err != nil {
			return 400, err.Error()
		}

		return 200, string(docs)
	})
	// create a path for each API described in the doc set
	for _, res := range rd.APIs {
		newDocFromSummary(res, db, cnf)
	}

}

// newDocFromSummary adds a new api documentation endpoint to the documentation
// path set up in the function above.
func newDocFromSummary(r *ResourceSummary, db Db_backend, cnf Conf) {
	// load either a populated resource or an empty one.
	resp, err := LoadResourceFromDb(db, strings.TrimLeft(r.Path, "/"), cnf)

	if err != nil {
		panic(err)
	}

	m.Get("/api-docs"+r.Path, func(res http.ResponseWriter) (int, string) {
		h := res.Header()

		h.Add("Content-Type", "application/json;charset=utf-8")
		doc, err := json.Marshal(resp)
		if err != nil {
			return 400, err.Error()
		}
		return 200, string(doc)
	})
	NewApiFromSpec(resp)
}

// NewApiFromSpec creates a new API from stored swagger-doc specifications.
func NewApiFromSpec(resource *Resource) {

	for _, api := range resource.Apis {
		for _, op := range api.Operations {
			addOperation(api, op, m, resource.BasePath, resource.Models)
		}
	}

}

// addOperation adds a single operation from an API specification.  This
// creates a GET/POST/PUT/PATCH/DELETE listener and maps inbound requests to
// backend functions.
// TODO - return specified error codes instead of 500
func addOperation(api *Api,
	op *Operation,
	m *martini.ClassicMartini,
	basePath string,
	models map[string]*Model) {

	path := translatePath(api.Path, basePath)

	switch op.Method {
	case "GET":
		m.Get(path, func(params martini.Params, req *http.Request, db Db_backend, res http.ResponseWriter) (int, string) {
			h := res.Header()

			h.Add("Content-Type", "application/json;charset=utf-8")
			req.ParseForm()

			// coerce any required path parameters
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}

			q := QueryParams{
				Path:        api.Path,
				PathParams:  outParams,
				QueryParams: req.Form,
			}

			result, err := db.Query(q)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 500, string(outerr)
			}
			out, err := json.Marshal(result)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 500, string(outerr)
			}
			return 200, string(out)
		})
		break
	case "POST":
		m.Post(path, func(params martini.Params, req *http.Request, db Db_backend, res http.ResponseWriter) (int, string) {
			h := res.Header()

			h.Add("Content-Type", "application/json;charset=utf-8")
			val, err := ioutil.ReadAll(req.Body)

			// coerce any required path parameters
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}

			q := QueryParams{
				Path:       api.Path,
				PathParams: outParams,
				Body:       val,
			}
			doc, err := db.Insert(q)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 500, string(outerr)
			}
			out, err := json.Marshal(doc)
			return 200, string(out)
		})
		break
	case "PUT":
		m.Put(path, func(params martini.Params, req *http.Request, db Db_backend, res http.ResponseWriter) (int, string) {
			h := res.Header()

			h.Add("Content-Type", "application/json;charset=utf-8")
			val, err := ioutil.ReadAll(req.Body)
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}
			q := QueryParams{
				Path:       api.Path,
				PathParams: outParams,
				Body:       val,
			}
			doc, err := db.Update(q, PUT)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 500, string(outerr)
			}
			out, err := json.Marshal(doc)
			return 200, string(out)
		})
		break
	case "PATCH":
		m.Patch(path, func(params martini.Params, req *http.Request, db Db_backend, res http.ResponseWriter) (int, string) {
			h := res.Header()

			h.Add("Content-Type", "application/json;charset=utf-8")
			val, err := ioutil.ReadAll(req.Body)
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}
			q := QueryParams{
				Path:       api.Path,
				PathParams: outParams,
				Body:       val,
			}
			doc, err := db.Update(q, PATCH)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 500, string(outerr)
			}
			out, err := json.Marshal(doc)
			return 200, string(out)
		})
		break
	case "DELETE":
		m.Delete(path, func(params martini.Params, db Db_backend, res http.ResponseWriter) (int, string) {
			h := res.Header()

			h.Add("Content-Type", "application/json;charset=utf-8")
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}
			q := QueryParams{
				Path:       api.Path,
				PathParams: outParams,
			}
			err = db.Remove(q)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 500, string(outerr)
			}
			return 200, ""
		})
		break
	}
}

// ugh pretending that dynamic typing is a thing
func coercePathParam(sentParams martini.Params, apiPathParams []*Property) (outParams map[string]interface{}, err error) {
	outParams = make(map[string]interface{})
	// and here is where we wish Golang had functional-style
	// list manipulation stuff
	for key, sentParam := range sentParams {
		for _, apiParam := range apiPathParams {
			if key == apiParam.Name {
				switch apiParam.Type {
				case "integer":
					val, parseErr := strconv.ParseInt(sentParam, 10, 0)
					if parseErr != nil {
						err = parseErr
						return
					}
					outParams[key] = val
					break

				default:
					outParams[key] = sentParam
					break
				}
				// done with this particular key
				break
			}
		}
	}

	return

}

// translatePath transforms a path from swagger-doc format (/path/{id}) to
// Martini format (/path/:id).
func translatePath(path string, basepath string) (outpath string) {
	outpath = basepath + PathRe.ReplaceAllString(path, "/$2/:$4")
	return
}
