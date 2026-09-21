package logdata

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestLogEntryDecodesStringData(t *testing.T) {
	id := primitive.NewObjectID()
	cases := []struct {
		name string
		data any
		want string
	}{
		{"object", bson.M{"actionType": "Interact"}, "Interact"},
		{"json string", `{"actionType":"Interact"}`, "Interact"},
		{"plain string", "oops", ""},
	}
	for _, c := range cases {
		raw, err := bson.Marshal(bson.M{"_id": id, "game": "mhs", "user_id": "u", "eventKey": "k", "data": c.data})
		if err != nil {
			t.Fatal(err)
		}
		var e LogEntry
		if err := bson.Unmarshal(raw, &e); err != nil {
			t.Fatalf("%s: decode failed: %v", c.name, err)
		}
		if e.EventKey != "k" || e.ID != id {
			t.Fatalf("%s: key fields lost: %+v", c.name, e)
		}
		got, _ := e.Data["actionType"].(string)
		if got != c.want {
			t.Fatalf("%s: actionType = %q, want %q (data %v)", c.name, got, c.want, e.Data)
		}
		if c.name == "plain string" && e.Data["_raw"] != "oops" {
			t.Fatalf("plain string payload not kept: %v", e.Data)
		}
	}
}
