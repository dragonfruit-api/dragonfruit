package dragonfruit

import (
	"github.com/go-martini/martini"
	"net/url"
)

// QueryParams are a container for http path, query and body information that's
// used by the back-ends.
type QueryParams struct {
	Path        string
	PathParams  martini.Params
	QueryParams url.Values
	Body        []byte
}

// ContainerMeta is a list of metadata about a result set.
type ContainerMeta struct {
	ResponseCode    int    `json:"responseCode,omitempty"`
	ResponseMessage string `json:"responseMessage,omitempty"`
	Offset          int    `json:"offset,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	Total           int    `json:"total,omitempty"`
	Count           int    `json:"count,omitempty"`
}

// A container is a wrapper for a list of results, plus some meta information
// about the result set.
type Container struct {
	Meta          ContainerMeta `json:"meta,omitempty"`
	ContainerType string        `json:"containerType"`
	Results       []interface{} `json:"results"`
}

// The Db_backend interface describes the methods which must be implemented
// for any backends used to store data.
type Db_backend interface {
	//// internal API
	// connect to a backend server
	Connect(string) error

	// Save a document to persistence.
	// (database, document id, contents)
	Save(string, string, interface{}) (string, interface{}, error)

	// Load a document from persistence
	// (database, document id, thing to unmarshal into)
	Load(string, string, interface{}) error

	// database, id, rev
	Delete(string, string, string) error

	// external API
	// Query the datastore with a QueryParams struct
	Query(QueryParams) (interface{}, error)

	// Update a document using a QueryParams struct
	Update(QueryParams) (interface{}, error)

	// Insert a new document using a QueryParams struct
	Insert(QueryParams) (interface{}, error)

	// Delete a document with a QueryParams struct
	Remove(QueryParams) error

	// Prep prepares a database to serve a new API.  For example, create
	// tables or collections, create views, etc.
	Prep(string, *Resource) error
}
