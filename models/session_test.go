package models

import (
	"testing"
)

func newSession(t *testing.T) (*Session, func(*testing.T)) {
	session := NewSession("mongodb://localhost:27021/?directConnection=true&serverSelectionTimeoutMS=2000", "test")
	session.ResetDB()
	return session, func(tb *testing.T) {
		defer session.Close()
		session.ResetDB()
	}
}

func TestCreate(t *testing.T) {
	want := &KV{"1", "2"}

	session, td := newSession(t)
	defer td(t)
	err := session.Create(want.Key, want.Value)
	if err != nil {
		t.Fatalf(`Create(%q): %v`, want.Key, err)
	}

	res, err := session.Get(want.Key)
	if err != nil {
		t.Fatalf(`Get(%q): %v`, want.Key, err)
	}

	if *res != *want {
		t.Fatalf(`Get(%q) = %v != %v`, want.Key, res, want)
	}
}

func TestMultipleCreate(t *testing.T) {
	want := &KV{"1", "2"}
	session, td := newSession(t)
	defer td(t)

	err := session.Create(want.Key, want.Value)
	if err != nil {
		t.Fatalf(`Create(%q): %v`, want.Key, err)
	}

	err = session.Create(want.Key, want.Value)
	if err != AlreadyExists {
		t.Fatalf(`Create(%q): %v`, want.Key, err)
	}
}

func TestDeleteUpdateCreate(t *testing.T) {
	want := &KV{"1", "2"}
	session, td := newSession(t)
	defer td(t)

	err := session.Create(want.Key, want.Value)
	if err != nil {
		t.Fatalf(`Create(%q): %v`, want.Key, err)
	}

	res, err := session.Get(want.Key)
	if err != nil {
		t.Fatalf(`Create(%q): %v`, want.Key, err)
	}

	if *res != *want {
		t.Fatalf(`Get(%q) = %v != %v`, want.Key, res, want)
	}

	err = session.Delete(want.Key)
	if err != nil {
		t.Fatalf(`Delete(%q): %v`, want.Key, err)
	}

	err = session.Update(want.Key, "42")
	if err != NotFound {
		t.Fatalf(`Update(%q, %q): %v`, want.Key, "42", err)
	}

	_, err = session.Get(want.Key)
	if err != NotFound {
		t.Fatalf(`Get(%q): %v`, want.Key, err)
	}

	err = session.Create(want.Key, want.Value)
	if err != nil {
		t.Errorf(`Create: did not detect repetition for (%q)`, want.Key)
	}
}

func TestUpdate(t *testing.T) {
	want := &KV{"1", "2"}
	session, td := newSession(t)
	defer td(t)

	err := session.Update(want.Key, "42")
	if err != NotFound {
		t.Fatalf(`Update: expected %q found %q`, NotFound, err)
	}

	err = session.Create(want.Key, want.Value)
	if err != nil {
		t.Fatalf(`Create(%q): %v`, want.Key, err)
	}

	err = session.Update(want.Key, "42")
	if err != nil {
		t.Fatalf(`Update(%q, %q): %v`, want.Key, "42", err)
	}

	var res *KV
	res, err = session.Get(want.Key)
	if err != nil {
		t.Fatalf(`Get(%q): %v`, want.Key, err)
	}

	if res.Value != "42" {
		t.Fatalf(`Wrong value %v, expected: %v `, res.Value, "42")
	}
}

func TestGetHistory(t *testing.T) {
	session, td := newSession(t)
	defer td(t)

	_ = session.Create("1", "1")
	_ = session.Create("1", "1") // ignored
	_ = session.Update("1", "2")

	_ = session.Delete("1")
	_ = session.Update("1", "10") // ignored

	_ = session.Create("1", "3")
	_ = session.Update("1", "4")

	evs, err := session.GetHistory("1", 10)
	if err != nil {
		t.Fatalf(`Get(%q): %v`, "1", err)
	}

	want := []struct {
		key   string
		value string
		op    string
	}{
		{"1", "4", "update"},
		{"1", "3", "create"},
		{"1", "2", "delete"},
		{"1", "2", "update"},
		{"1", "1", "create"},
	}

	for i := 0; i < len(evs); i++ {
		if evs[i].Kv.Key != want[i].key ||
			evs[i].Kv.Value != want[i].value ||
			evs[i].Op != want[i].op {
			t.Fatalf("Wrong event: %v, %v != %v", evs[i].Kv, evs[i].Timestamp, want[i])
		}
	}
}
