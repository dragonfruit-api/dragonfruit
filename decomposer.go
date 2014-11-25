package dragonfruit

import (
	"encoding/json"
	//"fmt"
	"github.com/gedex/inflector"
	"io/ioutil"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func Decompose(sampledata []byte, baseType string) (m map[string]*Model, err error) {

	var (
		val interface{}
	)

	baseType = strings.Title(baseType)
	//	container := new(Model)

	//	metalist := new(Model)

	byt, err := ioutil.ReadFile("containermodels.json")
	basecontaners := make([]*Model, 2)

	err = json.Unmarshal(byt, &basecontaners)

	m = make(map[string]*Model)
	m[ContainerName] = basecontaners[0]
	m["Metalist"] = basecontaners[1]

	appendSubtype(baseType, m)

	err = json.Unmarshal(sampledata, &val)
	if err != nil {
		panic("invalid json")
	}

	v := reflect.ValueOf(val)
	buildModel(baseType, m, v)

	return

}

func appendSubtype(baseSubtype string, m map[string]*Model) string {

	subtype := strings.Title(baseSubtype + strings.Title(ContainerName))

	//ugh
	for _, v := range m[ContainerName].SubTypes {
		if subtype == v {
			return subtype
		}
	}

	m[ContainerName].SubTypes = append(m[ContainerName].SubTypes, subtype)
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
	m[subtype].Properties["type"] = &Property{
		Type: "string",
	}
	return subtype
}

func buildModel(baseType string, m map[string]*Model, v reflect.Value) (err error) {
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

func buildProperty(propName string,
	modelName string,
	models map[string]*Model,
	v reflect.Value,
) (err error) {
	datatype, sanitized := translateKind(v)

	switch datatype {
	case "model":
		prop := &Property{
			Ref: propName,
		}
		buildModel(propName, models, sanitized)
		models[modelName].Properties[propName] = prop
		break

	case "array":
		iref := new(ItemsRef)

		prop := &Property{}
		prop.buildSliceProperty(inflector.Singularize(propName), iref, sanitized, models)

		/*prop := &Property{
			Type:  "array",
			Items: iref,
		}*/
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

func processString(v reflect.Value) *Property {
	//var err error
	prop := &Property{Type: "string"}

	tst := v.String()

	if strings.Contains(tst, "|") {
		prop.processSplit(tst)
	} else {
		prop.Format, _ = introspectFormat(tst)
	}

	return prop

}

func (prop *Property) processSplit(str string) {
	split := strings.Split(str, "|")

	// test for max and min values
	if len(split) == 2 {
		// if the string parses as an int or a float
		// ... set the min and max and the type to number
		// this is kind of ugly...
		_, interr1 := strconv.ParseInt(split[0], 10, 0)
		_, interr2 := strconv.ParseInt(split[1], 10, 0)
		fltval1, flterr1 := strconv.ParseFloat(split[0], 64)
		fltval2, flterr2 := strconv.ParseFloat(split[1], 64)
		if (interr1 == nil) && (interr2 == nil) {
			prop.Type = "integer"
			prop.Minimum = math.Trunc(math.Min(fltval1, fltval2))
			prop.Maximum = math.Trunc(math.Max(fltval1, fltval2))
		} else if (flterr1 == nil) && (flterr2 == nil) {
			prop.Type = "number"
			prop.Minimum = math.Min(fltval1, fltval2)
			prop.Maximum = math.Max(fltval1, fltval2)
		} else {
			prop.Type = "string"
			prop.Enum = split
		}

	} else {
		// same thing for a float

		prop.Enum = split
		prop.Type = "string"
	}

}

func processNumber(v reflect.Value) *Property {
	prop := &Property{}
	if math.Trunc(v.Float()) == v.Float() {
		prop.Type = "integer"
	} else {
		prop.Type = "number"
	}
	return prop
}

func (prop *Property) buildSliceProperty(name string, i *ItemsRef, v reflect.Value, m map[string]*Model) (err error) {
	for it := 0; it < v.Len(); it++ {
		datatype, sanitized := translateKind(v.Index(it))
		switch datatype {
		case "model":
			prop.Type = "array"
			buildModel(name, m, sanitized)
			appendSubtype(name, m)
			prop.Items = &ItemsRef{} //appendSubtype(name, m)
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

func introspectFormat(str string) (string, error) {
	var outerr error

	// this is voodoo magic
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

func checkUUID(str string) (ok bool, format string, err error) {
	re := "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
	ok, err = regexp.MatchString(re, str)
	format = "uuid"
	return
}

func checkEmail(str string) (ok bool, format string, err error) {
	re := "(?i)[A-Z0-9._%+-]+@[A-Z0-9.-]+\\.[A-Z]{2,4}"
	format = "email"
	ok, err = regexp.MatchString(re, str)
	return
}

func checkDate(str string) (ok bool, format string, err error) {
	_, dateErr := time.Parse("2006-01-02", str)
	ok = (dateErr == nil)
	format = "date"
	return
}

func checkDateTime(str string) (ok bool, format string, err error) {
	_, dateErr := time.Parse(time.RFC3339, str)
	ok = (dateErr == nil)
	format = "date-time"
	return
}

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
