// Package config provides typesafe, cloud native configuration binding from environment variables or files to structs.
//
// Configuration can be done in as little as two lines:
//     var c MyConfig
//     config.FromEnv().To(&c)
//
// A field's type determines what https://golang.org/pkg/strconv/ function is called.
//
// All string conversion rules are as defined in the https://golang.org/pkg/strconv/ package.
//
// If chaining multiple data sources, data sets are merged.
//
// Later values override previous values.
//   config.From("dev.config").FromEnv().To(&c)
//
// Unset values remain as their native zero value: https://tour.golang.org/basics/12.
//
// Nested structs/subconfigs are delimited with double underscore.
//   PARENT__CHILD
//
// Env vars map to struct fields case insensitively.
// NOTE: Also true when using struct tags.
package config

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	structTagKey         = "config"
	structDelim          = "__"
	sliceDelim           = " "
	structTagIgnoreField = "-"
)

// ValuePreProcessor is an interface for pre-processing values
type ValuePreProcessor interface {
	// PreProcessValue pre-processes a key/value pair for the config.
	PreProcessValue(key, value string) string
}

// Builder contains the current configuration state.
type Builder struct {
	structDelim, sliceDelim string
	configMap               map[string]string
	valuePreProcessor       ValuePreProcessor
}

// WithValuePreProcessor creates  a new builder with a ValuePreProcessor.
// This will be called when adding values to the builder's configMap.
// An example use would be retrieving values from AWS secret manager.
func WithValuePreProcessor(p ValuePreProcessor) *Builder {
	return newBuilder().WithValuePreProcessor(p)
}

// WithValuePreProcessor adds a ValuePreProcessor to the builder.
// This will be called when adding values to the builder's configMap.
// An example use would be retrieving values from AWS secret manager.
func (c *Builder) WithValuePreProcessor(p ValuePreProcessor) *Builder {
	c.valuePreProcessor = p
	return c
}

func newBuilder() *Builder {
	return &Builder{
		configMap:   make(map[string]string),
		structDelim: structDelim,
		sliceDelim:  sliceDelim,
	}
}

// To accepts a struct pointer, and populates it with the current config state.
// Supported fields:
//     * all int, uint, float variants
//     * bool, struct, string
//     * slice of any of the above, except for []struct{}
// It panics under the following circumstances:
//     * target is not a struct pointer
//     * struct contains unsupported fields (pointers, maps, slice of structs, channels, arrays, funcs, interfaces, complex)
func (c *Builder) To(target interface{}) {
	c.populateStructRecursively(target, "")
}

// From returns a new Builder, populated with the values from file.
// It panics if unable to open the file.
func From(file string) *Builder {
	return newBuilder().From(file)
}

// From merges new values from file into the current config state, returning the Builder.
// It panics if unable to open the file.
func (c *Builder) From(file string) *Builder {
	f, err := os.Open(file)
	if err != nil {
		panic(fmt.Sprintf("oops!: %v", err))
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var ss []string
	for scanner.Scan() {
		ss = append(ss, scanner.Text())
	}
	c.mergeConfig(stringsToMap(ss))
	return c
}

// FromEnv returns a new Builder, populated with environment variables
func FromEnv() *Builder {
	return newBuilder().FromEnv()
}

// FromEnv merges new values from the environment into the current config state, returning the Builder.
func (c *Builder) FromEnv() *Builder {
	c.mergeConfig(stringsToMap(os.Environ()))
	return c
}

func (c *Builder) mergeConfig(in map[string]string) {
	for k, v := range in {
		if c.valuePreProcessor != nil {
			v = c.valuePreProcessor.PreProcessValue(k, v)
		}

		c.configMap[k] = v
	}
}

// stringsToMap builds a map from a string slice.
// The input strings are assumed to be environment variable in style e.g. KEY=VALUE
// Keys with no value are not added to the map.
func stringsToMap(ss []string) map[string]string {
	m := make(map[string]string)
	for _, s := range ss {
		if !strings.Contains(s, "=") {
			continue // ensures return is always of length 2
		}
		split := strings.SplitN(s, "=", 2)
		key, value := strings.ToLower(split[0]), split[1]
		if key != "" && value != "" {
			m[key] = value
		}
	}
	return m
}

// populateStructRecursively populates each field of the passed in struct.
// slices and values are set directly.
// nested structs recurse through this function.
// values are derived from the field name, prefixed with the field names of any parents.
func (c *Builder) populateStructRecursively(structPtr interface{}, prefix string) {
	structValue := reflect.ValueOf(structPtr).Elem()
	for i := 0; i < structValue.NumField(); i++ {
		fieldType := structValue.Type().Field(i)
		fieldPtr := structValue.Field(i).Addr().Interface()

		possibleKey := getKey(fieldType, prefix)
		if possibleKey == nil {
			continue
		}

		key := *possibleKey
		value := c.configMap[key]

		switch fieldType.Type.Kind() {
		case reflect.Struct:
			c.populateStructRecursively(fieldPtr, key+c.structDelim)
		case reflect.Slice:
			convertAndSetSlice(fieldPtr, stringToSlice(value, c.sliceDelim))
		default:
			convertAndSetValue(fieldPtr, value)
		}
	}
}

// getKey returns the string that represents this structField in the config map.
// If the structField has the appropriate structTag set, it is used.
// Otherwise, field's name is used.
func getKey(t reflect.StructField, prefix string) *string {
	name := t.Name
	if tag, exists := t.Tag.Lookup(structTagKey); exists {
		if tag = strings.TrimSpace(tag); tag == structTagIgnoreField {
			return nil
		} else if tag != "" {
			name = tag
		}
	}

	key := strings.ToLower(prefix + name)
	return &key
}

// stringToSlice converts a string to a slice of string, using delim.
// It strips surrounding whitespace of all entries.
// If the input string is empty or all whitespace, nil is returned.
func stringToSlice(s, delim string) []string {
	if delim == "" {
		panic("empty delimiter") // impossible or programmer error
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	split := strings.Split(s, delim)
	filtered := split[:0] // https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
	for _, v := range split {
		v = strings.TrimSpace(v)
		if v != "" {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// convertAndSetSlice builds a slice of a dynamic type.
// It converts each entry in "values" to the elemType of the passed in slice.
// The slice remains nil if "values" is empty.
func convertAndSetSlice(slicePtr interface{}, values []string) {
	sliceVal := reflect.ValueOf(slicePtr).Elem()
	elemType := sliceVal.Type().Elem()

	for _, s := range values {
		valuePtr := reflect.New(elemType)
		convertAndSetValue(valuePtr.Interface(), s)
		sliceVal.Set(reflect.Append(sliceVal, valuePtr.Elem()))
	}
}

// convertAndSetValue receives a settable of an arbitrary kind, and sets its value to s".
// It calls the matching strconv function on s, based on the settable's kind.
// All basic types (bool, int, float, string) are handled by this function.
// Slice and struct are handled elsewhere.
// Unhandled kinds panic.
// Errors in string conversion are ignored, and the settable remains a zero value.
func convertAndSetValue(settable interface{}, s string) {
	settableValue := reflect.ValueOf(settable).Elem()
	i := settableValue.Interface()

	switch i.(type) {
	case string:
		settableValue.SetString(s)
	case time.Duration:
		d, _ := time.ParseDuration(s)
		settableValue.Set(reflect.ValueOf(d))
	case int:
		val, _ := strconv.ParseInt(s, 10, 0)
		settableValue.SetInt(val)
	case int8:
		val, _ := strconv.ParseInt(s, 10, 8)
		settableValue.SetInt(val)
	case int16:
		val, _ := strconv.ParseInt(s, 10, 16)
		settableValue.SetInt(val)
	case int32:
		val, _ := strconv.ParseInt(s, 10, 32)
		settableValue.SetInt(val)
	case int64:
		val, _ := strconv.ParseInt(s, 10, 64)
		settableValue.SetInt(val)
	case uint:
		val, _ := strconv.ParseUint(s, 10, 0)
		settableValue.SetUint(val)
	case uint8:
		val, _ := strconv.ParseUint(s, 10, 8)
		settableValue.SetUint(val)
	case uint16:
		val, _ := strconv.ParseUint(s, 10, 16)
		settableValue.SetUint(val)
	case uint32:
		val, _ := strconv.ParseUint(s, 10, 32)
		settableValue.SetUint(val)
	case uint64:
		val, _ := strconv.ParseUint(s, 10, 64)
		settableValue.SetUint(val)
	case bool:
		val, _ := strconv.ParseBool(s)
		settableValue.SetBool(val)
	case float32:
		val, _ := strconv.ParseFloat(s, 32)
		settableValue.SetFloat(val)
	case float64:
		val, _ := strconv.ParseFloat(s, 64)
		settableValue.SetFloat(val)
	default:
		panic(fmt.Sprintf("cannot handle kind %v\n", settableValue.Type().Kind()))
	}
}
