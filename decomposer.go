package dragonfruit

import (
	"encoding/json"
	"github.com/gedex/inflector"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Decompose is the only exported function in this file.  It takes a set of
// sample data, introspects it and converts it into a map of Swagger models.
// It returns the model map and/or any errors.
func Decompose(sampledata []byte, baseType string, cnf Conf) (m map[string]*Model, err error) {

	var receiver interface{}

	baseType = strings.Title(baseType)

	basecontainers := cnf.ContainerModels

	m = make(map[string]*Model)
	m[strings.Title(ContainerName)] = basecontainers[0]
	m["Metalist"] = basecontainers[1]

	appendSubtype(baseType, m)

	err = json.Unmarshal(sampledata, &receiver)
	if err != nil {
		panic("invalid json")
	}

	v := reflect.ValueOf(receiver)
	buildModel(baseType, m, v)

	return

}

// appendSubtype appends container types to models within the model map.
// It returns the subtype name (whether it's new or extant).
func appendSubtype(baseSubtype string, m map[string]*Model) string {
	cname := strings.Title(ContainerName)
	// the canonical name of the subtype container
	subtype := strings.Title(baseSubtype + cname)

	// check to see if the subtype has been defined
	// ugh
	for _, v := range m[cname].SubTypes {
		if subtype == v {
			return subtype
		}
	}

	// add the subtype to the model's list of subtypes
	m[cname].SubTypes = append(m[cname].SubTypes, subtype)

	// Create a new model in the model list
	m[subtype] = new(Model)
	m[subtype].Id = subtype
	m[subtype].Description = "A container for " + inflector.Pluralize(baseSubtype)
	m[subtype].Properties = make(map[string]*Property)
	m[subtype].Properties["results"] = &Property{
		Type: "array",
		Items: &ItemsRef{
			Ref: baseSubtype,
		},
	}
	return subtype
}

// buildModel initializes a new model.  It adds the new model to the model map
// and then creates new properties as needed.
//
// If the model exists for some reason (if for example a piece of sample data
// has multiple references to a particular sub-model), the existing model will
// be used.  If there are multiple instances of a model within some sample data
// with different properties, the final specified model will contain the union
// of all distinct properties from all instances of the model.
//
// Returns an error.
// TODO - this should probably be a method not a function.
func buildModel(baseType string,
	m map[string]*Model,
	v reflect.Value) (err error) {

	baseType = strings.Title(baseType)
	_, ok := m[baseType]
	if !ok {
		m[baseType] = &Model{
			Id:         baseType,
			Properties: make(map[string]*Property),
		}
	}

	for _, propindex := range v.MapKeys() {
		_, ok := m[baseType].Properties[propindex.String()]
		if !ok {
			buildProperty(propindex.String(), baseType, m, v.MapIndex(propindex))
		}

	}
	return
}

// build property creates a model property from a discrete piece of sample data.
// It adds the property to the model within the larger model slice.
// It returns an error of something blows up.
// TODO - this should probably be a method not a function.
func buildProperty(propName string,
	modelName string,
	models map[string]*Model,
	v reflect.Value,
) (err error) {
	datatype, sanitized := translateKind(v)

	switch datatype {
	case "model":
		prop := &Property{
			Ref: strings.Title(propName),
		}
		buildModel(propName, models, sanitized)
		models[modelName].Properties[propName] = prop
		break

	case "array":
		iref := new(ItemsRef)

		prop := &Property{}
		prop.buildSliceProperty(inflector.Singularize(propName), iref, sanitized, models)

		// always pluralize array stuff
		pname := inflector.Pluralize(inflector.Singularize(propName))

		models[modelName].Properties[pname] = prop
		break
	case "string":
		prop := processString(sanitized)
		models[modelName].Properties[propName] = prop
		break
	case "number":
		prop := processNumber(sanitized)
		models[modelName].Properties[propName] = prop
		break
	default:
		prop := &Property{
			Type: datatype,
		}
		models[modelName].Properties[propName] = prop
		break

	}
	return
}

// processString builds a new property from a string value in sample data.
// It checks for enumerated values and adds optional format data if appropriate.
// It returns a pointer to a new property.
func processString(v reflect.Value) *Property {
	prop := &Property{Type: "string"}

	tst := v.String()

	if strings.Contains(tst, "|") {
		prop.processSplit(tst)
	} else {
		prop.Format, _ = introspectFormat(tst)
	}
	return prop
}

// processSplit handles enumerated value hints (basically, strings with a pipe
// symbol).
// It mutates the property passed to it.
func (prop *Property) processSplit(str string) {
	split := strings.Split(str, "|")

	// test for max and min values
	// if there are only two elements in the split array,
	//
	if len(split) == 2 {
		// if the string parses as an int or a float
		// ... set the min and max and the type to number
		// this is kind of ugly...
		_, interr1 := strconv.ParseInt(split[0], 10, 0)
		_, interr2 := strconv.ParseInt(split[1], 10, 0)
		fltval1, flterr1 := strconv.ParseFloat(split[0], 64)
		fltval2, flterr2 := strconv.ParseFloat(split[1], 64)

		// if both values in the split are integers...
		if (interr1 == nil) && (interr2 == nil) {
			prop.Type = "integer"
			prop.Minimum = math.Trunc(math.Min(fltval1, fltval2))
			prop.Maximum = math.Trunc(math.Max(fltval1, fltval2))

			// if both are floats ...
		} else if (flterr1 == nil) && (flterr2 == nil) {
			prop.Type = "number"
			prop.Minimum = math.Min(fltval1, fltval2)
			prop.Maximum = math.Max(fltval1, fltval2)

			// else assume they are strings
		} else {
			prop.Type = "string"
			prop.Enum = split
			prop.Format, _ = introspectFormat(split[0])
		}

	} else {
		prop.Enum = split
		prop.Type = "string"
		prop.Format, _ = introspectFormat(split[0])
	}

}

// processNumber determines if the value is a float or an integer
// It returns a property reference with the number type.
// TODO - should this just return a string?
func processNumber(v reflect.Value) *Property {
	prop := &Property{}
	if math.Trunc(v.Float()) == v.Float() {
		prop.Type = "integer"
	} else {
		prop.Type = "number"
	}
	return prop
}

// buildSliceProperty parses array values passed through sample data.
// The function ranges over the values in the array, introspects the first
// element, determines its type, and traverses deeper into the model tree if
// the first element is a model type.
// It returns an error if something blows up.
func (prop *Property) buildSliceProperty(name string, i *ItemsRef,
	v reflect.Value,
	m map[string]*Model) (err error) {

	prop.Type = "array"
	for it := 0; it < v.Len(); it++ {
		datatype, sanitized := translateKind(v.Index(it))
		switch datatype {
		case "model":
			buildModel(name, m, sanitized)
			appendSubtype(name, m)
			prop.Items = &ItemsRef{}
			prop.Items.Ref = strings.Title(name)
			break
		default:
			prop.Type = "array"
			prop.Items = i
			i.Type = datatype
			break
		}
	}
	return
}

// introspectFormat examines a string value from sample data to determine
// if it is one of several pre-determined formats, such as email address, date
// or UUID.
// The format is added to the Swagger spec for string fields.
// It returns a format name (or blank string) or an error value.
func introspectFormat(str string) (string, error) {
	var outerr error

	// this is voodoo magic
	// basically this is a map of functions to iterate over
	// if the value matches one of them, it returns the format
	flist := []func(string) (bool, string, error){checkEmail, checkUUID, checkDateTime, checkDate}

	for _, fnc := range flist {
		// ugh shadowing...
		ok, format, err := fnc(str)
		if err != nil {
			return "", err
		}
		if ok {
			return format, outerr
		}
	}
	return "", outerr
}

// checkUUID checks to see if the string is formatted as a UUID.
// It returns a boolean if the format matches, the format name, and an error
// if the regular expression fails.
func checkUUID(str string) (ok bool, format string, err error) {
	re := "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
	ok, err = regexp.MatchString(re, str)
	format = "uuid"
	return
}

// checkEmail checks to see if the string is formatted as an email address.
// It returns a boolean if the format matches, the format name, and an error
// if the regular expression fails.
func checkEmail(str string) (ok bool, format string, err error) {
	re := "(?i)[A-Z0-9._%+-]+@[A-Z0-9.-]+\\.[A-Z]{2,4}"
	format = "email"
	ok, err = regexp.MatchString(re, str)
	return
}

// checkDate checks to see if the string is formatted as a date (YYYY-MM-DD).
// It returns a boolean if the format matches, the format name, and an error
// if the regular expression fails.
func checkDate(str string) (ok bool, format string, err error) {
	_, dateErr := time.Parse("2006-01-02", str)
	ok = (dateErr == nil)
	format = "date"
	return
}

// checkDateTime checks to see if the string is formatted as a datetime value
// using RFC3339
// It returns a boolean if the format matches, the format name, and an error
// if the regular expression fails.
func checkDateTime(str string) (ok bool, format string, err error) {
	_, dateErr := time.Parse(time.RFC3339, str)
	ok = (dateErr == nil)
	format = "date-time"
	return
}

// translateKind takes a value from sample data and translates it into one of
// the accepted Swagger types
// (see https://github.com/swagger-api/swagger-spec/blob/master/versions/1.2.md)
// It returns the type name and a dereferenced value (if the value is an
// interface or pointer).
func translateKind(v reflect.Value) (datatype string, sanitized reflect.Value) {
	sanitized = v

	switch v.Kind() {
	case reflect.Interface:
		datatype, sanitized = translateKind(v.Elem())
		break
	case reflect.String:
		datatype = "string"
		break
	case reflect.Bool:
		datatype = "boolean"
		break
	case reflect.Float64:
		datatype = "number"
		break
	case reflect.Map:
		datatype = "model"
		break
	case reflect.Slice:
		datatype = "array"
		break
	}
	return
}
