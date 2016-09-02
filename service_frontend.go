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
	PathParamRe *regexp.Regexp
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
	// translates Martini paths
	PathParamRe = regexp.MustCompile("(/([[:word:]]*)(/:[[:word:]]*)?)")
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
func ServeDocSet(m *martini.ClassicMartini, db Db_backend, cnf Conf) {
	m.Map(db)
	rd, err := db.LoadDefinition(cnf)
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
	for path, pathitem := range rd.Paths {

		NewApiFromSpec(path, pathitem, rd, m)
	}

}

// NewApiFromSpec creates a new API from stored swagger-doc specifications.
func NewApiFromSpec(path string, pathitem *PathItem, rd *Swagger, m *martini.ClassicMartini) {

	var pathArr = map[string]*Operation{
		"DELETE":  pathitem.Delete,
		"GET":     pathitem.Get,
		"HEAD":    pathitem.Head,
		"OPTIONS": pathitem.Options,
		"PUT":     pathitem.Put,
		"POST":    pathitem.Post,
	}

	for method, operation := range pathArr {
		if operation != nil {
			addOperation(rd.BasePath+path, pathitem, method, rd, operation, m)
		}
	}

}

// addOperation adds a single operation from an API specification.  This
// creates a GET/POST/PUT/PATCH/DELETE/OPTIONS listener and maps inbound requests to
// backend functions.
// TODO - return specified error codes instead of 500
func addOperation(path string,
	pathitem *PathItem,
	method string,
	rd *Swagger,
	op *Operation,
	m *martini.ClassicMartini,
) {

	path = TranslatePath(path)
	produces := append(rd.Produces, op.Produces...)

	consumes := append(rd.Consumes, op.Consumes...)

	switch method {
	case "GET":
		m.Get(path, func(params martini.Params, req *http.Request, db Db_backend, res http.ResponseWriter) (int, string) {
			h := res.Header()
			addHeaders(h, "Content-Type", produces)
			addHeaders(h, "Accept", consumes)

			req.ParseForm()

			// coerce any required path parameters
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}

			path = strings.TrimPrefix(path, rd.BasePath)
			q := QueryParams{
				Path:        path,
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

			addHeaders(h, "Content-Type", produces)
			addHeaders(h, "Accept", consumes)
			val, err := ioutil.ReadAll(req.Body)

			// coerce any required path parameters
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}

			path = strings.TrimPrefix(path, rd.BasePath)
			q := QueryParams{
				Path:       path,
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

			addHeaders(h, "Content-Type", produces)
			addHeaders(h, "Accept", consumes)

			val, err := ioutil.ReadAll(req.Body)
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}

			path = strings.TrimPrefix(path, rd.BasePath)
			q := QueryParams{
				Path:       path,
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

			addHeaders(h, "Content-Type", produces)
			addHeaders(h, "Accept", consumes)
			val, err := ioutil.ReadAll(req.Body)
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}

			path = strings.TrimPrefix(path, rd.BasePath)
			q := QueryParams{
				Path:       path,
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

			addHeaders(h, "Content-Type", produces)
			addHeaders(h, "Accept", consumes)
			outParams, err := coercePathParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}

			path = strings.TrimPrefix(path, rd.BasePath)
			q := QueryParams{
				Path:       path,
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

	case "OPTIONS":
		m.Options(path, func(db Db_backend, res http.ResponseWriter) (int, string) {
			h := res.Header()

			addHeaders(h, "Content-Type", produces)
			addHeaders(h, "Accept", consumes)

			for t, val := range op.Responses["200"].Headers {

				h.Add(t, val.Default.(string))
			}
			return 200, ""
		})
		break
	}
}

// ugh pretending that dynamic typing is a thing
func coercePathParam(sentParams martini.Params, apiPathParams []*Parameter) (outParams map[string]interface{}, err error) {
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

func addHeaders(h http.Header, headerType string, headArray []string) {
	for _, head := range headArray {
		h.Add(headerType, head)
	}
}

// translatePath transforms a path from swagger-doc format (/path/{id}) to
// Martini format (/path/:id).
func TranslatePath(path string) (outpath string) {
	outpath = PathRe.ReplaceAllString(path, "/$2/:$4")
	return
}
