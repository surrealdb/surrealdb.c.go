package surrealdbc

import "testing"

func TestObjectOperations(t *testing.T) {
	obj := newObject()
	defer obj.Free()

	obj.insertStr("name", "test")
	obj.insertInt("count", 42)
	obj.insertFloat("ratio", 3.14)
	obj.insertBool("active", true)

	if obj.len() != 4 {
		t.Fatalf("expected 4 keys, got %d", obj.len())
	}

	keys := obj.keys()
	if len(keys) != 4 {
		t.Fatalf("expected 4 keys, got %d", len(keys))
	}

	expected := map[string]bool{"name": false, "count": false, "ratio": false, "active": false}
	for _, k := range keys {
		if _, ok := expected[k]; !ok {
			t.Errorf("unexpected key: %s", k)
		}
		expected[k] = true
	}
	for k, found := range expected {
		if !found {
			t.Errorf("missing key: %s", k)
		}
	}
}
