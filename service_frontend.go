package dragonfruit

import (
	"encoding/json"

	"errors"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/cors"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	PathRe          *regexp.Regexp
	TerminalPath    *regexp.Regexp
	ReqPathRe       *regexp.Regexp
	GetDbRe         *regexp.Regexp
	ViewPathRe      *regexp.Regexp
	ViewPathParamRe *regexp.Regexp
	PathParamRe     *regexp.Regexp
	EndOfPathRe     *regexp.Regexp
	PostPathRe      *regexp.Regexp
	m               *martini.ClassicMartini
)

const (
	GET = iota
	PUT
	PATCH
	POST
	DELETE
)

const (
	NOTFOUNDERROR = "Entity not found."
)

// init sets up some basic regular expressions used by the frontend and
// builds the base Martini instance.
func init() {
	// there are too many of these and they are confusing...

	// looks for paths terminating with {params}
	PathRe = regexp.MustCompile("(/([[:word:]]+)/({([[:word:]]+)}){1})")

	// paths terminating with a param
	TerminalPath = regexp.MustCompile("{[[:word:]]+}$")
	ReqPathRe = regexp.MustCompile("(/([[:word:]]*)/([[:word:]]*))")

	// pull the initial segment out of the path
	GetDbRe = regexp.MustCompile("^/([[:word:]]*)/?")

	// translates Martini paths
	PathParamRe = regexp.MustCompile("(/([[:word:]]*)(/:([[:word:]]*))?)")

	// used by the couch backend ...
	ViewPathParamRe = regexp.MustCompile("/([[:word:]]*)(/:([[:word:]]*))?")
	ViewPathRe = regexp.MustCompile("(/([[:word:]]*)(/{[[:word:]]*})?)")
	EndOfPathRe = regexp.MustCompile("[^/]+$")
	m = martini.Classic()
	m.Use(cors.Allow(&cors.Options{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"PUT", "PATCH", "POST", "GET", "OPTIONS", "DELETE"},
		AllowHeaders: []string{"Origin", "Expires", "Cache-Control", "X-Requested-With", "Content-Type"},
	}))
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
		"PATCH":   pathitem.Patch,
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
			isCollection := false

			h := res.Header()
			addHeaders(h, "Content-Type", produces)
			addHeaders(h, "Accept", consumes)

			req.ParseForm()

			// coerce path parameters
			outParams, err := coerceParam(params, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}

			// coerce query parameters
			qParams, err := coerceQueryParam(req.Form, op.Parameters)
			if err != nil {
				outerr, _ := json.Marshal(err.Error())
				return 409, string(outerr)
			}

			path = strings.TrimPrefix(path, rd.BasePath)

			pathcount := strings.Split(path, "/")
			if (len(pathcount) % 2) == 0 {
				isCollection = true
			}

			q := QueryParams{
				Path:        path,
				PathParams:  outParams,
				QueryParams: qParams,
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
			if result.Meta.Count == 0 && (isCollection == false) {
				notFoundError := errors.New("Entity not found.")
				return 404, notFoundError.Error()
			}

			return 200, string(out)
		})
		break
	case "POST":
		m.Post(path, func(params martini.Params, req *http.Request, db Db_backend, res http.ResponseWriter) (int, string) {
			h := res.Header()

			addHeaders(h, "Content-Type", produces)
			addHeaders(h, "Accept", consumes)
			// TODO - validate post body.

			val, err := ioutil.ReadAll(req.Body)

			// coerce any required path parameters
			outParams, err := coerceParam(params, op.Parameters)
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
			return 201, string(out)
		})
		break
	case "PUT":
		m.Put(path, func(params martini.Params, req *http.Request, db Db_backend, res http.ResponseWriter) (int, string) {
			h := res.Header()

			addHeaders(h, "Content-Type", produces)
			addHeaders(h, "Accept", consumes)

			// TODO - validate put bodies.
			val, err := ioutil.ReadAll(req.Body)
			outParams, err := coerceParam(params, op.Parameters)
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
				if err.Error() == NOTFOUNDERROR {
					return 404, string(err.Error())
				}

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
			outParams, err := coerceParam(params, op.Parameters)
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

			if err.Error() == NOTFOUNDERROR {
				return 404, string(err.Error())
			}

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
			outParams, err := coerceParam(params, op.Parameters)
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

			if err != nil && err.Error() == NOTFOUNDERROR {
				return 404, string(err.Error())
			}

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

// Coerces string parameters from the path to the type specified
// in the API definition.
func coerceParam(sentParams martini.Params,
	apiPathParams []*Parameter) (outParams map[string]interface{},
	err error) {
	outParams = make(map[string]interface{})

	// and here is where we wish Golang had functional-style
	// list manipulation stuff
	for key, sentParam := range sentParams {
		present := false

		for _, apiParam := range apiPathParams {

			if key == apiParam.Name {
				present = true

				switch apiParam.Type {
				case "integer":
					val, parseErr := strconv.ParseInt(sentParam, 10, 0)
					if parseErr != nil {
						err = parseErr
						return
					}
					outParams[key] = val
					break
				case "number":
					val, parseErr := strconv.ParseFloat(sentParam, 32)
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

		if present == false {
			return outParams, errors.New("The parameter " + key + " is not valid.")
		}
	}

	return

}

// Coerces string parameters from the query to the type specified
// in the API definition. Returns an error if a param cannot be coerced.
// TODO - merge with above function to avoid copypasta
func coerceQueryParam(sentParams url.Values,
	apiPathParams []*Parameter) (outParams map[string]interface{},
	err error) {
	outParams = make(map[string]interface{})

	for key, _ := range sentParams {
		present := false
		for _, apiParam := range apiPathParams {

			if key == apiParam.Name {
				present = true
				sentParam := sentParams.Get(key)

				// this feels way too verbose...
				if apiParam.Minimum != apiParam.Maximum {
					inBound := checkBounds(apiParam, key, sentParam)
					if inBound != nil {
						err = inBound
						return
					}
				}

				switch apiParam.Type {
				case "integer":
					val, parseErr := strconv.ParseInt(sentParam, 10, 0)
					if parseErr != nil {
						err = parseErr
						return
					}

					intEnumErr := checkIntEnum(apiParam, key, val)
					if intEnumErr != nil {
						err = intEnumErr
						return
					}

					outParams[key] = val
					break
				case "number":
					val, parseErr := strconv.ParseFloat(sentParam, 32)
					if parseErr != nil {
						err = parseErr
						return
					}

					floatEnumErr := checkFloatEnum(apiParam, key, val)
					if floatEnumErr != nil {
						err = floatEnumErr
						return
					}

					outParams[key] = val
					break

				default:
					strEnumErr := checkStrEnum(apiParam, key, sentParam)
					if strEnumErr != nil {
						err = strEnumErr
						return
					}
					outParams[key] = sentParam

					break
				}
				// done with this particular key
				break
			}
		}

		if present == false {
			return outParams, errors.New("The parameter " + key + " is not valid.")
		}
	}

	return

}

func checkStrEnum(apiParam *Parameter, paramName string, sentParam string) error {
	if len(apiParam.Enum) == 0 {
		return nil
	}

	sl := make([]string, 0, 0)

	for _, v := range apiParam.Enum {
		sl = append(sl, v.(string))
		if sentParam == v.(string) {
			return nil
		}
	}

	strList := strings.Join(sl, ", ")
	msg := fmt.Sprintf("You sent the value \"%s\" for the parameter \"%s\".  The sent parameter must be within the enumerated set [%s].", sentParam, paramName, strList)

	return errors.New(msg)
}

func checkFloatEnum(apiParam *Parameter, paramName string, sentParam float64) error {
	if len(apiParam.Enum) == 0 {
		return nil
	}

	sl := make([]string, 0, 0)

	for _, v := range apiParam.Enum {
		st := strconv.FormatFloat(v.(float64), 'f', -1, 64)
		sl = append(sl, st)
		if sentParam == v.(float64) {
			return nil
		}
	}

	strList := strings.Join(sl, ", ")
	msg := fmt.Sprintf("You sent the value \"%f\" for the parameter \"%s\".  The sent parameter must be within the enumerated set [%s].", sentParam, paramName, strList)

	return errors.New(msg)
}

func checkIntEnum(apiParam *Parameter, paramName string, sentParam int64) error {
	if len(apiParam.Enum) == 0 {
		return nil
	}

	sl := make([]string, 0, 0)

	for _, v := range apiParam.Enum {
		st := strconv.FormatFloat(v.(float64), 'f', 0, 64)
		sl = append(sl, st)
		if sentParam == int64(v.(float64)) {
			return nil
		}
	}

	strList := strings.Join(sl, ", ")
	msg := fmt.Sprintf("You sent the value \"%d\" for the parameter \"%s\".  The sent parameter must be within the enumerated set [%s].", sentParam, paramName, strList)

	return errors.New(msg)
}

func checkBounds(apiParam *Parameter, paramName string, sentParam string) error {
	if apiParam.Type != "integer" && apiParam.Type != "number" {
		return nil
	}

	val, parseErr := strconv.ParseFloat(sentParam, 32)

	if parseErr != nil {
		return parseErr
	}

	if apiParam.Type == "integer" {
		minFormat := int(math.Trunc(apiParam.Minimum))
		maxFormat := int(math.Trunc(apiParam.Maximum))
		if val > apiParam.Maximum {
			msg := fmt.Sprintf("The parameter %s cannot be more than %d.  %s was sent.", paramName, maxFormat, sentParam)
			return errors.New(msg)
		}

		if val < apiParam.Minimum {
			msg := fmt.Sprintf("The parameter %s cannot be less than %d.  %s was sent.", paramName, minFormat, sentParam)
			return errors.New(msg)
		}
	}

	if val > apiParam.Maximum {
		msg := fmt.Sprintf("The parameter %s cannot be more than %f.  %s was sent.", paramName, apiParam.Maximum, sentParam)
		return errors.New(msg)
	}

	if val < apiParam.Minimum {
		msg := fmt.Sprintf("The parameter %s cannot be less than %f.  %s was sent.", paramName, apiParam.Minimum, sentParam)
		return errors.New(msg)
	}

	return nil

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
