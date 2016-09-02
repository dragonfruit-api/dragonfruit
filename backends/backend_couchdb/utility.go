package backend_couchdb

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gedex/inflector"
	"github.com/ideo/dragonfruit"
	"os/exec"
	"reflect"
	"strings"
	"time"
)

// sanitizeDoc removes _id and _rev keys from the document map
func sanitizeDoc(doc interface{}) (interface{}, error) {
	out, err := sanitizeDocInternal(reflect.ValueOf(doc))
	if err != nil {
		return doc, err
	}
	return out.Interface(), nil
}

// see sanitizeDoc
func sanitizeDocInternal(doc reflect.Value) (reflect.Value, error) {
	switch doc.Kind() {
	default:
		return doc, errors.New("invalid doc type")
	case reflect.Interface:
		return sanitizeDocInternal(doc.Elem())
		break
	case reflect.Map:

		for _, key := range doc.MapKeys() {
			if key.String() == "_id" ||
				key.String() == "_rev" {

				doc.SetMapIndex(key, reflect.ValueOf(nil))
			}
		}
	}
	return doc, nil
}

// modelizePath inflects a model name
func modelizePath(modelName string) string {
	return strings.Title(inflector.Singularize(modelName))
}

// modelizeContainer extracts a model name from a container for that model
func modelizeContainer(container string) string {
	return strings.Replace(container, strings.Title(dragonfruit.ContainerName), "", -1)
}

// makeQueryViewName makes canonical view names for GET queries
func makeQueryViewName(param string) string {
	return "by_query_" + param
}

// makePathViewName makes canonical view names for path parameters
func makePathViewName(path string) string {
	matches := dragonfruit.PathParamRe.FindAllStringSubmatch(path, -1)
	out := make([]string, 0)

	for _, match := range matches {
		out = append(out, match[2])
	}

	return "by_path_" + strings.Join(out, "_")
}

// makeTypeName returns a content type from path parameters.
func makeTypeName(path string) string {
	matches := dragonfruit.ViewPathRe.FindAllStringSubmatch(path, -1)
	return inflector.Singularize(matches[(len(matches) - 1)][2])
}

func (d *Db_backend_couch) ensureConnection() (err error) {
	defer func() {
		<-d.connection
	}()

	// Block other attempts to ensure the connection while we're making sure it's connected.
	d.connection <- true
	err = d.client.Ping()
	if err == nil {
		return
	}

	_, err = exec.Command("couchdb", "-b").Output()
	if err != nil {
		return err
	}

	var s func() error
	s = func() error {
		var err error
		fmt.Println("Waiting for couchdb to start...")
		s_out, err := exec.Command("couchdb", "-s").CombinedOutput()

		z := []byte{10}
		if bytes.Equal(z, s_out) {
			time.Sleep(1000 * time.Millisecond)
			return s()
		}

		if bytes.Contains(s_out, []byte("Apache CouchDB is running as process")) {
			time.Sleep(1000 * time.Millisecond)
			return err
		}
		if bytes.Contains(s_out, []byte("Apache CouchDB is not running.")) {
			fmt.Println("not running")
			time.Sleep(1000 * time.Millisecond)
			return s()
		}
		if err != nil {
			fmt.Println(s_out, string(s_out))
			fmt.Println("Launch error: ", err, "please send this to Peter O.")
		}
		if err != nil {
			return err
		}

		fmt.Println("So this happened: ", string(s_out), "... Please send this to Peter O.")
		return errors.New(string(s_out))

	}

	err = s()

	return
}
