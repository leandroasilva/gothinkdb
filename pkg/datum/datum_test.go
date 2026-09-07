package datum

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestDatumTypes(t *testing.T) {
	tests := []struct {
		name string
		d    Datum
		typ  Type
	}{
		{"MinVal", NewMinVal(), MinVal},
		{"MaxVal", NewMaxVal(), MaxVal},
		{"Null", NewNull(), Null},
		{"Bool_true", NewBool(true), Bool},
		{"Bool_false", NewBool(false), Bool},
		{"Num", NewNum(42.5), Num},
		{"Int", NewInt(42), Num},
		{"Str", NewStr("hello"), Str},
		{"Array", NewArray([]Datum{NewInt(1), NewInt(2)}), Array},
		{"Object", NewObjectFromMap(map[string]Datum{"key": NewStr("value")}), Object},
		{"Binary", NewBinary([]byte{1, 2, 3}), Binary},
		{"Time", NewTime(time.Now()), Time},
		{"Geometry", NewGeometry(NewPoint(1.0, 2.0)), Geometry},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.d.Type() != tt.typ {
				t.Errorf("expected type %v, got %v", tt.typ, tt.d.Type())
			}
		})
	}
}

func TestDatumAccessors(t *testing.T) {
	t.Run("Bool", func(t *testing.T) {
		d := NewBool(true)
		if !d.Bool() {
			t.Error("expected true")
		}
	})

	t.Run("Num", func(t *testing.T) {
		d := NewNum(42.5)
		if d.Num() != 42.5 {
			t.Errorf("expected 42.5, got %v", d.Num())
		}
	})

	t.Run("Int", func(t *testing.T) {
		d := NewInt(42)
		if d.Int() != 42 {
			t.Errorf("expected 42, got %v", d.Int())
		}
	})

	t.Run("Str", func(t *testing.T) {
		d := NewStr("hello")
		if d.Str() != "hello" {
			t.Errorf("expected 'hello', got %v", d.Str())
		}
	})

	t.Run("Array", func(t *testing.T) {
		arr := []Datum{NewInt(1), NewInt(2), NewInt(3)}
		d := NewArray(arr)
		if len(d.Array()) != 3 {
			t.Errorf("expected 3 elements, got %v", len(d.Array()))
		}
	})

	t.Run("Object", func(t *testing.T) {
		obj := map[string]Datum{"key": NewStr("value")}
		d := NewObjectFromMap(obj)
		if d.Object().Len() != 1 {
			t.Errorf("expected 1 field, got %v", d.Object().Len())
		}
	})

	t.Run("Binary", func(t *testing.T) {
		data := []byte{1, 2, 3}
		d := NewBinary(data)
		if len(d.Binary()) != 3 {
			t.Errorf("expected 3 bytes, got %v", len(d.Binary()))
		}
	})

	t.Run("Time", func(t *testing.T) {
		now := time.Now()
		d := NewTime(now)
		if !d.Time().Equal(now) {
			t.Errorf("expected %v, got %v", now, d.Time())
		}
	})

	t.Run("Geometry", func(t *testing.T) {
		geom := NewPoint(1.0, 2.0)
		d := NewGeometry(geom)
		if d.Geometry().Type() != "Point" {
			t.Errorf("expected Point, got %v", d.Geometry().Type())
		}
	})
}

func TestDatumComparison(t *testing.T) {
	tests := []struct {
		name     string
		a, b     Datum
		expected int
	}{
		// Type ordering
		{"MinVal_MinVal", NewMinVal(), NewMinVal(), 0},
		{"MinVal_Null", NewMinVal(), NewNull(), -1},
		{"Null_Bool", NewNull(), NewBool(false), -1},
		{"Bool_Num", NewBool(true), NewNum(0), -1},
		{"Num_Str", NewNum(999), NewStr("a"), -1},
		{"Str_Array", NewStr("z"), NewArray([]Datum{}), -1},
		{"Array_Object", NewArray([]Datum{}), NewObjectFromMap(nil), -1},
		{"Object_Binary", NewObjectFromMap(nil), NewBinary(nil), -1},
		{"Binary_Time", NewBinary(nil), NewTime(time.Now()), -1},
		{"Time_Geometry", NewTime(time.Now()), NewGeometry(NewPoint(0, 0)), -1},
		{"Geometry_MaxVal", NewGeometry(NewPoint(0, 0)), NewMaxVal(), -1},

		// Bool comparison
		{"Bool_false_false", NewBool(false), NewBool(false), 0},
		{"Bool_true_true", NewBool(true), NewBool(true), 0},
		{"Bool_false_true", NewBool(false), NewBool(true), -1},
		{"Bool_true_false", NewBool(true), NewBool(false), 1},

		// Num comparison
		{"Num_equal", NewNum(42), NewNum(42), 0},
		{"Num_less", NewNum(1), NewNum(2), -1},
		{"Num_greater", NewNum(2), NewNum(1), 1},
		{"Num_NaN_NaN", NewNum(math.NaN()), NewNum(math.NaN()), 0},
		{"Num_NaN_num", NewNum(math.NaN()), NewNum(1), -1},

		// Str comparison
		{"Str_equal", NewStr("hello"), NewStr("hello"), 0},
		{"Str_less", NewStr("abc"), NewStr("def"), -1},
		{"Str_greater", NewStr("def"), NewStr("abc"), 1},

		// Array comparison
		{"Array_equal", NewArray([]Datum{NewInt(1), NewInt(2)}), NewArray([]Datum{NewInt(1), NewInt(2)}), 0},
		{"Array_less_shorter", NewArray([]Datum{NewInt(1)}), NewArray([]Datum{NewInt(1), NewInt(2)}), -1},
		{"Array_less_elem", NewArray([]Datum{NewInt(1), NewInt(1)}), NewArray([]Datum{NewInt(1), NewInt(2)}), -1},

		// Object comparison
		{"Object_equal", NewObjectFromMap(map[string]Datum{"a": NewInt(1)}), NewObjectFromMap(map[string]Datum{"a": NewInt(1)}), 0},
		{"Object_less_fewer_keys", NewObjectFromMap(map[string]Datum{"a": NewInt(1)}), NewObjectFromMap(map[string]Datum{"a": NewInt(1), "b": NewInt(2)}), -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.a.Compare(tt.b)
			if (tt.expected < 0 && result >= 0) || (tt.expected > 0 && result <= 0) || (tt.expected == 0 && result != 0) {
				t.Errorf("Compare(%v, %v) = %d, expected sign %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestDatumJSON(t *testing.T) {
	t.Run("Null", func(t *testing.T) {
		d := NewNull()
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if string(data) != "null" {
			t.Errorf("expected 'null', got %s", data)
		}

		var d2 Datum
		if err := json.Unmarshal(data, &d2); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if !d2.IsNull() {
			t.Error("expected Null")
		}
	})

	t.Run("Bool", func(t *testing.T) {
		d := NewBool(true)
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if string(data) != "true" {
			t.Errorf("expected 'true', got %s", data)
		}

		var d2 Datum
		if err := json.Unmarshal(data, &d2); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if !d2.Bool() {
			t.Error("expected true")
		}
	})

	t.Run("Num", func(t *testing.T) {
		d := NewNum(42.5)
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var d2 Datum
		if err := json.Unmarshal(data, &d2); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if d2.Num() != 42.5 {
			t.Errorf("expected 42.5, got %v", d2.Num())
		}
	})

	t.Run("Str", func(t *testing.T) {
		d := NewStr("hello")
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var d2 Datum
		if err := json.Unmarshal(data, &d2); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if d2.Str() != "hello" {
			t.Errorf("expected 'hello', got %v", d2.Str())
		}
	})

	t.Run("Array", func(t *testing.T) {
		d := NewArray([]Datum{NewInt(1), NewStr("two"), NewBool(true)})
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var d2 Datum
		if err := json.Unmarshal(data, &d2); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		arr := d2.Array()
		if len(arr) != 3 {
			t.Errorf("expected 3 elements, got %v", len(arr))
		}
	})

	t.Run("Object", func(t *testing.T) {
		d := NewObjectFromMap(map[string]Datum{
			"name": NewStr("John"),
			"age":  NewInt(30),
		})
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var d2 Datum
		if err := json.Unmarshal(data, &d2); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		obj := d2.Object()
		if obj.Len() != 2 {
			t.Errorf("expected 2 fields, got %v", obj.Len())
		}
	})

	t.Run("Time_pseudo_type", func(t *testing.T) {
		now := time.Now()
		d := NewTime(now)
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		// Should be a pseudo-type
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if raw["$reql_type$"] != "TIME" {
			t.Errorf("expected $reql_type$ = TIME, got %v", raw["$reql_type$"])
		}

		var d2 Datum
		if err := json.Unmarshal(data, &d2); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if d2.Type() != Time {
			t.Errorf("expected Time type, got %v", d2.Type())
		}
	})
}

func TestObjectData(t *testing.T) {
	t.Run("NewObjectDataFromMap", func(t *testing.T) {
		obj := NewObjectDataFromMap(map[string]Datum{
			"b": NewInt(2),
			"a": NewInt(1),
			"c": NewInt(3),
		})

		if obj.Len() != 3 {
			t.Errorf("expected 3 fields, got %v", obj.Len())
		}

		// Keys should be sorted
		keys := obj.Keys()
		if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
			t.Errorf("keys not sorted: %v", keys)
		}
	})

	t.Run("Get_Set_Delete", func(t *testing.T) {
		obj := NewObjectDataFromMap(nil)

		// Set
		obj.Set("key1", NewStr("value1"))
		obj.Set("key2", NewStr("value2"))

		if obj.Len() != 2 {
			t.Errorf("expected 2 fields, got %v", obj.Len())
		}

		// Get
		val, ok := obj.Get("key1")
		if !ok {
			t.Error("expected key1 to exist")
		}
		if val.Str() != "value1" {
			t.Errorf("expected 'value1', got %v", val.Str())
		}

		// Delete
		obj.Delete("key1")
		if obj.Len() != 1 {
			t.Errorf("expected 1 field after delete, got %v", obj.Len())
		}

		_, ok = obj.Get("key1")
		if ok {
			t.Error("expected key1 to be deleted")
		}
	})

	t.Run("Compare", func(t *testing.T) {
		obj1 := NewObjectDataFromMap(map[string]Datum{"a": NewInt(1)})
		obj2 := NewObjectDataFromMap(map[string]Datum{"a": NewInt(1)})
		obj3 := NewObjectDataFromMap(map[string]Datum{"a": NewInt(2)})

		if obj1.Compare(obj2) != 0 {
			t.Error("expected equal objects")
		}

		if obj1.Compare(obj3) >= 0 {
			t.Error("expected obj1 < obj3")
		}
	})
}

func TestFromInterface(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		typ   Type
	}{
		{"nil", nil, Null},
		{"bool", true, Bool},
		{"float64", 42.5, Num},
		{"int", 42, Num},
		{"string", "hello", Str},
		{"array", []interface{}{1, 2, 3}, Array},
		{"object", map[string]interface{}{"key": "value"}, Object},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Datum
			if err := d.FromInterface(tt.input); err != nil {
				t.Fatalf("FromInterface error: %v", err)
			}
			if d.Type() != tt.typ {
				t.Errorf("expected type %v, got %v", tt.typ, d.Type())
			}
		})
	}
}

func BenchmarkDatumCompare(b *testing.B) {
	d1 := NewStr("hello world")
	d2 := NewStr("hello world")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d1.Compare(d2)
	}
}

func BenchmarkObjectSet(b *testing.B) {
	obj := NewObjectDataFromMap(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj.Set("key", NewInt(i))
	}
}
