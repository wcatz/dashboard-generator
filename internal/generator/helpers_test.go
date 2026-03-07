package generator

import (
	"testing"
)

func TestGetString(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		def  string
		want string
	}{
		{"present", map[string]interface{}{"k": "val"}, "k", "d", "val"},
		{"missing", map[string]interface{}{}, "k", "d", "d"},
		{"non-string", map[string]interface{}{"k": 42}, "k", "d", "d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getString(tt.m, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		def  int
		want int
	}{
		{"int", map[string]interface{}{"k": 10}, "k", 0, 10},
		{"int64", map[string]interface{}{"k": int64(20)}, "k", 0, 20},
		{"float64", map[string]interface{}{"k": float64(30)}, "k", 0, 30},
		{"missing", map[string]interface{}{}, "k", 5, 5},
		{"non-numeric", map[string]interface{}{"k": "abc"}, "k", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getInt(tt.m, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetFloat(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		def  float64
		want float64
	}{
		{"float64", map[string]interface{}{"k": 1.5}, "k", 0, 1.5},
		{"int", map[string]interface{}{"k": 3}, "k", 0, 3.0},
		{"int64", map[string]interface{}{"k": int64(7)}, "k", 0, 7.0},
		{"missing", map[string]interface{}{}, "k", 9.9, 9.9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getFloat(tt.m, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getFloat() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestGetNumber(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		def  float64
		want interface{}
	}{
		{"float64 whole", map[string]interface{}{"k": float64(5)}, "k", 0, int(5)},
		{"float64 decimal", map[string]interface{}{"k": 3.14}, "k", 0, 3.14},
		{"int", map[string]interface{}{"k": 42}, "k", 0, 42},
		{"int64", map[string]interface{}{"k": int64(99)}, "k", 0, int(99)},
		{"missing whole default", map[string]interface{}{}, "k", 7.0, int(7)},
		{"missing decimal default", map[string]interface{}{}, "k", 2.5, 2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNumber(tt.m, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getNumber() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		def  bool
		want bool
	}{
		{"present true", map[string]interface{}{"k": true}, "k", false, true},
		{"present false", map[string]interface{}{"k": false}, "k", true, false},
		{"missing", map[string]interface{}{}, "k", true, true},
		{"non-bool", map[string]interface{}{"k": "yes"}, "k", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBool(tt.m, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStringSlice(t *testing.T) {
	t.Run("[]interface{}", func(t *testing.T) {
		m := map[string]interface{}{"k": []interface{}{"a", "b"}}
		got := getStringSlice(m, "k", nil)
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("got %v, want [a b]", got)
		}
	})

	t.Run("[]string", func(t *testing.T) {
		m := map[string]interface{}{"k": []string{"x", "y"}}
		got := getStringSlice(m, "k", nil)
		if len(got) != 2 || got[0] != "x" || got[1] != "y" {
			t.Errorf("got %v, want [x y]", got)
		}
	})

	t.Run("missing nil default", func(t *testing.T) {
		m := map[string]interface{}{}
		got := getStringSlice(m, "k", nil)
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("missing non-nil default", func(t *testing.T) {
		m := map[string]interface{}{}
		got := getStringSlice(m, "k", []string{"d"})
		if len(got) != 1 || got[0] != "d" {
			t.Errorf("got %v, want [d]", got)
		}
	})
}

func TestGetStringSliceAsStrings(t *testing.T) {
	t.Run("[]interface{} strings", func(t *testing.T) {
		m := map[string]interface{}{"k": []interface{}{"a", "b"}}
		got := getStringSliceAsStrings(m, "k")
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("got %v, want [a b]", got)
		}
	})

	t.Run("[]string", func(t *testing.T) {
		m := map[string]interface{}{"k": []string{"x"}}
		got := getStringSliceAsStrings(m, "k")
		if len(got) != 1 || got[0] != "x" {
			t.Errorf("got %v, want [x]", got)
		}
	})

	t.Run("missing", func(t *testing.T) {
		m := map[string]interface{}{}
		got := getStringSliceAsStrings(m, "k")
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("mixed types in slice", func(t *testing.T) {
		m := map[string]interface{}{"k": []interface{}{"a", 42, "b"}}
		got := getStringSliceAsStrings(m, "k")
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("got %v, want [a b] (non-strings skipped)", got)
		}
	})
}

func TestContains(t *testing.T) {
	if !contains("hello world", "world") {
		t.Error("expected true for substring match")
	}
	if contains("hello world", "xyz") {
		t.Error("expected false for no match")
	}
}
