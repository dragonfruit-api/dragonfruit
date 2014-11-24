package dragonfruit

import (
	"github.com/go-martini/martini"
	"net/url"
)

type QueryParams struct {
	Path        string
	PathParams  martini.Params
	QueryParams url.Values
	Body        []byte
}

type Subpart struct {
	partName string
	id       string
}

type ContainerMeta struct {
	ResponseCode    int    `json:"responseCode,omitempty"`
	ResponseMessage string `json:"responseMessage,omitempty"`
	Offset          int    `json:"offset,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	Total           int    `json:"total,omitempty"`
	Count           int    `json:"count,omitempty"`
}

type Container struct {
	Meta    ContainerMeta `json:"meta,omitempty"`
	Type    string        `json:"type"`
	Results []interface{} `json:"results"`
}

type Db_backend interface {
	//// internal functions

	// connect
	Connect(string) error

	// database, document id, contents
	Save(string, string, interface{}) (string, interface{}, error)

	// database, id, thing to unmarshal into
	Load(string, string, interface{}) error

	// database, id, rev
	Delete(string, string, string) error

	// external API
	// database, params
	Query(QueryParams) (interface{}, error)

	//

	Insert(QueryParams) (interface{}, error)

	Remove(QueryParams) error

	Prep(string, *Resource)
}
