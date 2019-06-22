package dragonfruit

type qparam map[string]interface{}

func (q qparam) Get(key string) interface{} {
	val, ok := q[key]
	if ok {
		return val
	}
	out := ""
	return out
}

func (q qparam) Del(key string) {
	_, ok := q[key]
	if ok {
		delete(q, key)
	}
}

// QueryParams are a container for http path, query and body information that's
// used by the back-ends.
type QueryParams struct {
	Path        string
	PathParams  map[string]interface{}
	QueryParams qparam
	Body        []byte
}

// ContainerMeta is a list of metadata about a result set.
type ContainerMeta struct {
	ResponseCode    int    `json:"responseCode,omitempty"`
	ResponseMessage string `json:"responseMessage,omitempty"`
	Offset          int    `json:"offset"`
	Limit           int    `json:"limit,omitempty"`
	Total           int    `json:"total"`
	Count           int    `json:"count"`
}

// A Container is a wrapper for a list of results, plus some meta information
// about the result set.
type Container struct {
	Meta          ContainerMeta `json:"meta,omitempty"`
	ContainerType string        `json:"containerType"`
	Results       []interface{} `json:"results"`
}

// The DbBackend interface describes the methods which must be implemented
// for any backends used to store data.
type DbBackend interface {
	//// internal API
	// connect to a backend server
	Connect(string) error

	LoadDefinition(Conf) (*Swagger, error)

	SaveDefinition(*Swagger) error

	// external API
	// Query the datastore with a QueryParams struct
	Query(QueryParams) (Container, error)

	// Update a document using a QueryParams struct
	Update(QueryParams, int) (interface{}, error)

	// Insert a new document using a QueryParams struct
	Insert(QueryParams) (interface{}, error)

	// Delete a document with a QueryParams struct
	Remove(QueryParams) error

	// Prep prepares a database to serve a new API.  For example, create
	// tables or collections, create views, etc.
	Prep(string, *Swagger) error
}
