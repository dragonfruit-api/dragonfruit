package dragonfruit

import (
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

var (
	PathRe     *regexp.Regexp
	ReqPathRe  *regexp.Regexp
	GetDbRe    *regexp.Regexp
	ViewPathRe *regexp.Regexp
	// ugh globals
	m *martini.ClassicMartini
)

func init() {
	PathRe = regexp.MustCompile("(/([[:word:]]*)/({([[:word:]]*)}))")
	ReqPathRe = regexp.MustCompile("(/([[:word:]]*)/([[:word:]]*))")
	GetDbRe = regexp.MustCompile("^/([[:word:]]*)/?")
	ViewPathRe = regexp.MustCompile("(/([[:word:]]*)(/{[[:word:]]*})?)")
	m = martini.Classic()
}

// blarg...
func GetMartiniInstance() *martini.ClassicMartini {
	return m
}

func ServeDocSet(db Db_backend) {
	m.Map(db)
	rd, err := LoadDescriptionFromDb(db, "resourceTemplate.json")
	if err != nil {
		panic(err)
	}

	m.Get("/api-docs", func(res http.ResponseWriter) (int, string) {
		h := res.Header()

		h.Add("Content-Type", "application/json")
		docs, err := json.Marshal(rd)
		if err != nil {
			return 400, err.Error()
		}

		return 200, string(docs)
	})
	for _, res := range rd.APIs {
		fmt.Println(res.Path)
		NewDocFromSummary(res, db)
	}

	/*m.Group("/api-docs",
		func(r martini.Router) {
			r.Get("",
				func(db Db_backend) (int, string) {
					fmt.Println(db)
					return 200, "yeap"
				})
		})
	m.Get("api-docs")*/

}

func NewDocFromSummary(r *ResourceSummary, db Db_backend) {
	resp, err := LoadResourceFromDb(db, strings.TrimLeft(r.Path, "/"), "resourceTemplate.json")
	if err != nil {
		panic(err)
	}
	m.Get("/api-docs"+r.Path, func(res http.ResponseWriter) (int, string) {
		h := res.Header()

		h.Add("Content-Type", "application/json")
		doc, err := json.Marshal(resp)
		if err != nil {
			return 400, err.Error()
		}
		return 200, string(doc)
	})
	NewApiFromSpec(resp)
}

func NewApiFromSpec(resource *Resource) {

	for _, api := range resource.Apis {
		for _, op := range api.Operations {
			addOperation(api, op, m, resource.BasePath, resource.Models)
		}
	}

}

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

			h.Add("Content-Type", "application/json")
			req.ParseForm()

			q := QueryParams{
				Path:        api.Path,
				PathParams:  params,
				QueryParams: req.Form,
			}

			result, err := db.Query(q)
			if err != nil {
				return 500, err.Error()
			}
			out, err := json.Marshal(result)
			if err != nil {
				return 500, err.Error()
			}
			return 200, string(out)
		})
		break
	case "POST":
		m.Post(path, func(params martini.Params, req *http.Request, db Db_backend) (int, string) {
			val, err := ioutil.ReadAll(req.Body)
			q := QueryParams{
				Path:       api.Path,
				PathParams: params,
				Body:       val,
			}
			doc, err := db.Insert(q)
			return 200, fmt.Sprint(doc) + fmt.Sprint(err)
		})
		break
	case "DELETE":
		m.Delete(path, func(params martini.Params, db Db_backend) (int, string) {
			q := QueryParams{
				Path:       api.Path,
				PathParams: params,
			}
			err := db.Remove(q)
			return 200, fmt.Sprint(err)
		})
		break
	}
}

func translatePath(path string, basepath string) (outpath string) {
	outpath = basepath + PathRe.ReplaceAllString(path, "/$2/:$4")
	return
}
