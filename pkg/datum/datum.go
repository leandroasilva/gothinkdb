// Package datum provides the fundamental data types for ReQL (RethinkDB Query Language).
// This package is equivalent to the datum_t type in RethinkDB's C++ implementation.
package datum

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

// Type represents the type of a Datum
type Type int

const (
	// MinVal is the smallest possible value in RethinkDB ordering
	MinVal Type = iota
	// MaxVal is the largest possible value in RethinkDB ordering
	MaxVal
	// Null represents a null value
	Null
	// Bool represents a boolean value
	Bool
	// Num represents a numeric value (float64)
	Num
	// Str represents a string value
	Str
	// Array represents an array of datums
	Array
	// Object represents an object/map of datums
	Object
	// Binary represents binary data (pseudo-type)
	Binary
	// Time represents a time value (pseudo-type)
	Time
	// Geometry represents a geospatial value (pseudo-type)
	Geometry
)

// String returns the string representation of a Type
func (t Type) String() string {
	switch t {
	case MinVal:
		return "MINVAL"
	case MaxVal:
		return "MAXVAL"
	case Null:
		return "NULL"
	case Bool:
		return "BOOL"
	case Num:
		return "NUM"
	case Str:
		return "STR"
	case Array:
		return "ARRAY"
	case Object:
		return "OBJECT"
	case Binary:
		return "BINARY"
	case Time:
		return "TIME"
	case Geometry:
		return "GEOMETRY"
	default:
		return "UNKNOWN"
	}
}

// Datum represents a ReQL value. It's the fundamental data type used throughout
// the ReQL query system, equivalent to datum_t in RethinkDB's C++ implementation.
type Datum struct {
	typ Type

	// Value fields (only one is used based on typ)
	boolVal   bool
	numVal    float64
	strVal    string
	arrayVal  []Datum
	objectVal *ObjectData
	binaryVal []byte
	timeVal   time.Time
	geometry  *GeometryData
}

// Constructor functions

// NewMinVal creates a MinVal datum
func NewMinVal() Datum {
	return Datum{typ: MinVal}
}

// NewMaxVal creates a MaxVal datum
func NewMaxVal() Datum {
	return Datum{typ: MaxVal}
}

// NewNull creates a Null datum
func NewNull() Datum {
	return Datum{typ: Null}
}

// NewBool creates a Bool datum
func NewBool(val bool) Datum {
	return Datum{typ: Bool, boolVal: val}
}

// NewNum creates a Num datum
func NewNum(val float64) Datum {
	return Datum{typ: Num, numVal: val}
}

// NewInt creates a Num datum from an int
func NewInt(val int) Datum {
	return Datum{typ: Num, numVal: float64(val)}
}

// NewStr creates a Str datum
func NewStr(val string) Datum {
	return Datum{typ: Str, strVal: val}
}

// NewArray creates an Array datum
func NewArray(vals []Datum) Datum {
	return Datum{typ: Array, arrayVal: vals}
}

// NewObject creates an Object datum
func NewObject(obj *ObjectData) Datum {
	return Datum{typ: Object, objectVal: obj}
}

// NewObjectFromMap creates an Object datum from a map
func NewObjectFromMap(m map[string]Datum) Datum {
	return Datum{typ: Object, objectVal: NewObjectDataFromMap(m)}
}

// NewBinary creates a Binary datum
func NewBinary(val []byte) Datum {
	return Datum{typ: Binary, binaryVal: val}
}

// NewTime creates a Time datum
func NewTime(val time.Time) Datum {
	return Datum{typ: Time, timeVal: val}
}

// NewGeometry creates a Geometry datum
func NewGeometry(geom *GeometryData) Datum {
	return Datum{typ: Geometry, geometry: geom}
}

// Type returns the type of the datum
func (d Datum) Type() Type {
	return d.typ
}

// TypeString returns the string representation of the datum's type
func (d Datum) TypeString() string {
	return d.typ.String()
}

// Accessor methods

// Bool returns the boolean value. Panics if not a Bool.
func (d Datum) Bool() bool {
	if d.typ != Bool {
		panic(fmt.Sprintf("datum is not a Bool, got %s", d.typ))
	}
	return d.boolVal
}

// Num returns the numeric value. Panics if not a Num.
func (d Datum) Num() float64 {
	if d.typ != Num {
		panic(fmt.Sprintf("datum is not a Num, got %s", d.typ))
	}
	return d.numVal
}

// Int returns the numeric value as int. Panics if not a Num.
func (d Datum) Int() int {
	if d.typ != Num {
		panic(fmt.Sprintf("datum is not a Num, got %s", d.typ))
	}
	return int(d.numVal)
}

// Str returns the string value. Panics if not a Str.
func (d Datum) Str() string {
	if d.typ != Str {
		panic(fmt.Sprintf("datum is not a Str, got %s", d.typ))
	}
	return d.strVal
}

// Array returns the array value. Panics if not an Array.
func (d Datum) Array() []Datum {
	if d.typ != Array {
		panic(fmt.Sprintf("datum is not an Array, got %s", d.typ))
	}
	return d.arrayVal
}

// Object returns the object value. Panics if not an Object.
func (d Datum) Object() *ObjectData {
	if d.typ != Object {
		panic(fmt.Sprintf("datum is not an Object, got %s", d.typ))
	}
	return d.objectVal
}

// Binary returns the binary value. Panics if not Binary.
func (d Datum) Binary() []byte {
	if d.typ != Binary {
		panic(fmt.Sprintf("datum is not Binary, got %s", d.typ))
	}
	return d.binaryVal
}

// Time returns the time value. Panics if not a Time.
func (d Datum) Time() time.Time {
	if d.typ != Time {
		panic(fmt.Sprintf("datum is not a Time, got %s", d.typ))
	}
	return d.timeVal
}

// Geometry returns the geometry value. Panics if not a Geometry.
func (d Datum) Geometry() *GeometryData {
	if d.typ != Geometry {
		panic(fmt.Sprintf("datum is not a Geometry, got %s", d.typ))
	}
	return d.geometry
}

// IsNull returns true if the datum is Null
func (d Datum) IsNull() bool {
	return d.typ == Null
}

// IsMinVal returns true if the datum is MinVal
func (d Datum) IsMinVal() bool {
	return d.typ == MinVal
}

// IsMaxVal returns true if the datum is MaxVal
func (d Datum) IsMaxVal() bool {
	return d.typ == MaxVal
}

// Compare compares two datums using RethinkDB's total ordering.
// Returns -1 if d < other, 0 if d == other, 1 if d > other.
//
// RethinkDB ordering (from smallest to largest):
// MinVal < Null < Bool < Num < Str < Array < Object < Binary < Time < Geometry < MaxVal
func (d Datum) Compare(other Datum) int {
	// Different types: compare by type order
	if d.typ != other.typ {
		return compareTypes(d.typ, other.typ)
	}

	// Same type: compare values
	switch d.typ {
	case MinVal, MaxVal, Null:
		return 0 // These types are singletons

	case Bool:
		if d.boolVal == other.boolVal {
			return 0
		}
		if !d.boolVal {
			return -1 // false < true
		}
		return 1

	case Num:
		if d.numVal < other.numVal {
			return -1
		}
		if d.numVal > other.numVal {
			return 1
		}
		// Handle NaN
		if math.IsNaN(d.numVal) && math.IsNaN(other.numVal) {
			return 0
		}
		if math.IsNaN(d.numVal) {
			return -1 // NaN < any number
		}
		if math.IsNaN(other.numVal) {
			return 1
		}
		return 0

	case Str:
		if d.strVal < other.strVal {
			return -1
		}
		if d.strVal > other.strVal {
			return 1
		}
		return 0

	case Array:
		return compareArrays(d.arrayVal, other.arrayVal)

	case Object:
		return d.objectVal.Compare(other.objectVal)

	case Binary:
		return compareBytes(d.binaryVal, other.binaryVal)

	case Time:
		if d.timeVal.Before(other.timeVal) {
			return -1
		}
		if d.timeVal.After(other.timeVal) {
			return 1
		}
		return 0

	case Geometry:
		// Geometry comparison is complex, for now compare by type then coordinates
		return d.geometry.Compare(other.geometry)
	}

	return 0
}

// Equal returns true if two datums are equal
func (d Datum) Equal(other Datum) bool {
	return d.Compare(other) == 0
}

// Less returns true if d < other
func (d Datum) Less(other Datum) bool {
	return d.Compare(other) < 0
}

// compareTypes compares two types according to RethinkDB ordering
func compareTypes(a, b Type) int {
	order := map[Type]int{
		MinVal:   0,
		Null:     1,
		Bool:     2,
		Num:      3,
		Str:      4,
		Array:    5,
		Object:   6,
		Binary:   7,
		Time:     8,
		Geometry: 9,
		MaxVal:   10,
	}

	if order[a] < order[b] {
		return -1
	}
	if order[a] > order[b] {
		return 1
	}
	return 0
}

// compareArrays compares two arrays lexicographically
func compareArrays(a, b []Datum) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if cmp := a[i].Compare(b[i]); cmp != 0 {
			return cmp
		}
	}

	// All elements equal, shorter array is smaller
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// compareBytes compares two byte slices lexicographically
func compareBytes(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}

	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// String returns a string representation of the datum
func (d Datum) String() string {
	switch d.typ {
	case MinVal:
		return "MINVAL"
	case MaxVal:
		return "MAXVAL"
	case Null:
		return "null"
	case Bool:
		if d.boolVal {
			return "true"
		}
		return "false"
	case Num:
		return fmt.Sprintf("%v", d.numVal)
	case Str:
		return fmt.Sprintf("%q", d.strVal)
	case Array:
		return fmt.Sprintf("Array(%d elements)", len(d.arrayVal))
	case Object:
		return fmt.Sprintf("Object(%d fields)", d.objectVal.Len())
	case Binary:
		return fmt.Sprintf("Binary(%d bytes)", len(d.binaryVal))
	case Time:
		return d.timeVal.Format(time.RFC3339Nano)
	case Geometry:
		return fmt.Sprintf("Geometry(%s)", d.geometry.Type())
	default:
		return "UNKNOWN"
	}
}

// MarshalJSON implements json.Marshaler
func (d Datum) MarshalJSON() ([]byte, error) {
	switch d.typ {
	case MinVal, MaxVal:
		return nil, fmt.Errorf("cannot marshal %s to JSON", d.typ)
	case Null:
		return json.Marshal(nil)
	case Bool:
		return json.Marshal(d.boolVal)
	case Num:
		return json.Marshal(d.numVal)
	case Str:
		return json.Marshal(d.strVal)
	case Array:
		return json.Marshal(d.arrayVal)
	case Object:
		return json.Marshal(d.objectVal)
	case Binary:
		// Pseudo-type: {"$reql_type$": "BINARY", "data": "<base64>"}
		return json.Marshal(map[string]interface{}{
			"$reql_type$": "BINARY",
			"data":        d.binaryVal,
		})
	case Time:
		// Pseudo-type: {"$reql_type$": "TIME", "epoch_time": <float>, "timezone": "<string>"}
		epochTime := float64(d.timeVal.UnixNano()) / 1e9
		_, offset := d.timeVal.Zone()
		timezone := fmt.Sprintf("%+03d:00", offset/3600)
		return json.Marshal(map[string]interface{}{
			"$reql_type$": "TIME",
			"epoch_time":  epochTime,
			"timezone":    timezone,
		})
	case Geometry:
		// Pseudo-type: GeoJSON format
		return json.Marshal(d.geometry)
	default:
		return nil, fmt.Errorf("unknown datum type: %s", d.typ)
	}
}

// UnmarshalJSON implements json.Unmarshaler
func (d *Datum) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as different types
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	return d.FromInterface(raw)
}

// FromInterface converts a Go interface{} to a Datum
func (d *Datum) FromInterface(val interface{}) error {
	if val == nil {
		*d = NewNull()
		return nil
	}

	switch v := val.(type) {
	case Datum:
		*d = v
	case *Datum:
		*d = *v
	case bool:
		*d = NewBool(v)
	case float64:
		*d = NewNum(v)
	case float32:
		*d = NewNum(float64(v))
	case int:
		*d = NewInt(v)
	case int64:
		*d = NewNum(float64(v))
	case string:
		*d = NewStr(v)
	case []interface{}:
		arr := make([]Datum, len(v))
		for i, elem := range v {
			if err := arr[i].FromInterface(elem); err != nil {
				return err
			}
		}
		*d = NewArray(arr)
	case map[string]interface{}:
		// Check for pseudo-types
		if reqlType, ok := v["$reql_type$"]; ok {
			return d.unmarshalPseudoType(reqlType.(string), v)
		}
		// Regular object
		obj := NewObjectDataFromMap(nil)
		for key, value := range v {
			var datum Datum
			if err := datum.FromInterface(value); err != nil {
				return err
			}
			obj.Set(key, datum)
		}
		*d = NewObject(obj)
	default:
		return fmt.Errorf("cannot convert %T to Datum", val)
	}

	return nil
}

// unmarshalPseudoType handles unmarshaling of pseudo-types
func (d *Datum) unmarshalPseudoType(reqlType string, m map[string]interface{}) error {
	switch reqlType {
	case "BINARY":
		data, ok := m["data"]
		if !ok {
			return fmt.Errorf("BINARY pseudo-type missing 'data' field")
		}
		if str, ok := data.(string); ok {
			*d = NewBinary([]byte(str))
		} else {
			return fmt.Errorf("BINARY pseudo-type 'data' must be a string")
		}

	case "TIME":
		epochTime, ok := m["epoch_time"]
		if !ok {
			return fmt.Errorf("TIME pseudo-type missing 'epoch_time' field")
		}
		timezone, _ := m["timezone"].(string)

		var epoch float64
		switch v := epochTime.(type) {
		case float64:
			epoch = v
		case int:
			epoch = float64(v)
		default:
			return fmt.Errorf("TIME pseudo-type 'epoch_time' must be numeric")
		}

		sec := int64(epoch)
		nsec := int64((epoch - float64(sec)) * 1e9)
		t := time.Unix(sec, nsec)

		// Parse timezone if provided
		if timezone != "" {
			loc, err := time.Parse("-07:00", timezone)
			if err == nil {
				t = t.In(loc.Location())
			}
		}

		*d = NewTime(t)

	case "GEOMETRY":
		// TODO: Implement geometry unmarshaling
		return fmt.Errorf("GEOMETRY pseudo-type not yet implemented")

	default:
		return fmt.Errorf("unknown pseudo-type: %s", reqlType)
	}

	return nil
}

// ObjectData represents a ReQL object with stable key ordering
type ObjectData struct {
	fields map[string]Datum
	keys   []string // Sorted keys for deterministic ordering
}

// NewObjectDataFromMap creates an ObjectData from a map
func NewObjectDataFromMap(m map[string]Datum) *ObjectData {
	obj := &ObjectData{
		fields: make(map[string]Datum),
		keys:   make([]string, 0),
	}

	if m != nil {
		for k, v := range m {
			obj.fields[k] = v
			obj.keys = append(obj.keys, k)
		}
		sort.Strings(obj.keys)
	}

	return obj
}

// Get returns the value for a key
func (o *ObjectData) Get(key string) (Datum, bool) {
	val, ok := o.fields[key]
	return val, ok
}

// Set sets a value for a key
func (o *ObjectData) Set(key string, val Datum) {
	if _, exists := o.fields[key]; !exists {
		o.keys = append(o.keys, key)
		sort.Strings(o.keys)
	}
	o.fields[key] = val
}

// Delete removes a key
func (o *ObjectData) Delete(key string) {
	if _, exists := o.fields[key]; exists {
		delete(o.fields, key)
		for i, k := range o.keys {
			if k == key {
				o.keys = append(o.keys[:i], o.keys[i+1:]...)
				break
			}
		}
	}
}

// Len returns the number of fields
func (o *ObjectData) Len() int {
	return len(o.fields)
}

// Keys returns the sorted keys
func (o *ObjectData) Keys() []string {
	return o.keys
}

// Values returns the values in key order
func (o *ObjectData) Values() []Datum {
	vals := make([]Datum, len(o.keys))
	for i, key := range o.keys {
		vals[i] = o.fields[key]
	}
	return vals
}

// Compare compares two objects
func (o *ObjectData) Compare(other *ObjectData) int {
	// Compare by keys first
	if len(o.keys) < len(other.keys) {
		return -1
	}
	if len(o.keys) > len(other.keys) {
		return 1
	}

	// Compare keys lexicographically
	for i := 0; i < len(o.keys); i++ {
		if o.keys[i] < other.keys[i] {
			return -1
		}
		if o.keys[i] > other.keys[i] {
			return 1
		}
	}

	// Keys are equal, compare values
	for _, key := range o.keys {
		if cmp := o.fields[key].Compare(other.fields[key]); cmp != 0 {
			return cmp
		}
	}

	return 0
}

// MarshalJSON implements json.Marshaler for ObjectData
func (o *ObjectData) MarshalJSON() ([]byte, error) {
	m := make(map[string]Datum, len(o.fields))
	for k, v := range o.fields {
		m[k] = v
	}
	return json.Marshal(m)
}

// GeometryData represents a geospatial value
type GeometryData struct {
	geomType    string // "Point", "LineString", "Polygon"
	coordinates interface{}
}

// NewPoint creates a Point geometry
func NewPoint(lon, lat float64) *GeometryData {
	return &GeometryData{
		geomType:    "Point",
		coordinates: [2]float64{lon, lat},
	}
}

// Type returns the geometry type
func (g *GeometryData) Type() string {
	return g.geomType
}

// Compare compares two geometries
func (g *GeometryData) Compare(other *GeometryData) int {
	// Simple comparison by type then coordinates
	if g.geomType < other.geomType {
		return -1
	}
	if g.geomType > other.geomType {
		return 1
	}

	// For now, just compare by type (full geometry comparison is complex)
	return 0
}

// MarshalJSON implements json.Marshaler for GeometryData
func (g *GeometryData) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"$reql_type$": "GEOMETRY",
		"type":        g.geomType,
		"coordinates": g.coordinates,
	})
}
