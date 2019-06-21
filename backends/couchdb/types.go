package couchdb

import (
	"github.com/fjl/go-couchdb"
)

// A CouchDB view.
type view struct {
	MapFunc    string `json:"map"`
	ReduceFunc string `json:"reduce,omitempty"`
}

// A CouchDB design document.
type viewDoc struct {
	ID       string          `json:"_id"`
	Rev      string          `json:"_rev,omitempty"`
	Language string          `json:"language"`
	Views    map[string]view `json:"views"`
}

// DbBackendCouch is the exported client that you would use in your app.
type DbBackendCouch struct {
	client     *couchdb.Client
	connection chan bool
}

// viewParams are used to create design documents during the prep phase
type viewParam struct {
	path         string
	singlepath   string
	paramname    string
	paramtype    string
	propertyname string
}

// Represents a row returned by a couchdb result
type couchdbRow struct {
	Doc   map[string]interface{} `json:"doc,omitempty"`
	Id    string                 `json:"id"`
	Key   interface{}            `json:"key"`
	Value map[string]interface{} `json:"value"`
}

// Represents a couchdb result
type couchDbResponse struct {
	Rows      []couchdbRow `json:"rows"`
	Offset    int          `json:"offset"`
	TotalRows int          `json:"total_rows"`
	Limit     int          `json:"-"`
}
