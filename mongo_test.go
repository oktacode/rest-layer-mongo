package mongo

import (
	"context"
	"testing"
	"time"

	"github.com/oktacode/rest-layer/resource"
	"github.com/oktacode/rest-layer/schema/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/mgo.v2"
)

// Mongo doesn't support nanoseconds
var now = time.Now().Round(time.Millisecond)

// cleanup deletes a database immediately and on defer when call as:
//
//   defer cleanup(c, "database")()
func cleanup(s *mgo.Session, db string) func() {
	s.DB(db).DropDatabase()
	return func() {
		s.DB(db).DropDatabase()
	}
}

// asserts that the items in a collection matches the provided list of IDs.
func assertCollectionIDs(t testing.TB, c *mgo.Collection, expect []string) {
	var ids []string
	assert.NoError(t, c.Find(nil).Distinct("_id", &ids))
	assert.EqualValues(t, expect, ids)
}

func TestInsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	s, err := mgo.Dial("")
	if !assert.NoError(t, err) {
		return
	}
	defer cleanup(s, "testinsert")()
	h := NewHandler(s, "testinsert", "test")
	items := []*resource.Item{
		{
			ID:      "1234",
			ETag:    "etag",
			Updated: now,
			Payload: map[string]interface{}{
				"id":  "1234",
				"foo": "bar",
			},
		},
	}
	err = h.Insert(context.Background(), items)
	assert.NoError(t, err)
	d := map[string]interface{}{}
	err = s.DB("testinsert").C("test").FindId("1234").One(&d)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, map[string]interface{}{"foo": "bar", "_id": "1234", "_etag": "etag", "_updated": now}, d)

	// Inserting same item twice should return a conflict error
	err = h.Insert(context.Background(), items)
	assert.Equal(t, resource.ErrConflict, err)
}

func TestUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	s, err := mgo.Dial("")
	if !assert.NoError(t, err) {
		return
	}
	defer cleanup(s, "testupdate")()
	h := NewHandler(s, "testupdate", "test")
	oldItem := &resource.Item{
		ID:      "1234",
		ETag:    "etag1",
		Updated: now,
		Payload: map[string]interface{}{
			"id":  "1234",
			"foo": "bar",
		},
	}
	newItem := &resource.Item{
		ID:      "1234",
		ETag:    "etag2",
		Updated: now,
		Payload: map[string]interface{}{
			"id":  "1234",
			"foo": "baz",
		},
	}

	// Can't update a non existing item
	err = h.Update(context.Background(), newItem, oldItem)
	assert.Equal(t, resource.ErrNotFound, err)

	err = h.Insert(context.Background(), []*resource.Item{oldItem})
	assert.NoError(t, err)
	err = h.Update(context.Background(), newItem, oldItem)
	assert.NoError(t, err)

	// Update refused if original item's etag doesn't match stored one
	err = h.Update(context.Background(), newItem, oldItem)
	assert.Equal(t, resource.ErrConflict, err)

	c := s.DB("testupdate").C("testEtag")
	// Add an item without _etag field
	c.Insert(map[string]interface{}{"foo": "bar", "_id": "1234", "_updated": now})
	h2 := NewHandler(s, "testupdate", "testEtag")
	// A item without _etag field, is extracted with ETag in "p-[id]" format
	originalItem := &resource.Item{
		ID:      "1234",
		ETag:    "p-1234",
		Updated: now,
		Payload: map[string]interface{}{
			"id":  "1234",
			"foo": "baz",
		},
	}
	item := &resource.Item{
		ID:      "1234",
		ETag:    "etag",
		Updated: now,
		Payload: map[string]interface{}{
			"id":  "1234",
			"foo": "baz",
		},
	}
	// Update an original item with Etag over item in DB without _etag
	err = h2.Update(context.Background(), item, originalItem)
	assert.NoError(t, err)

	d := map[string]interface{}{}
	err = s.DB("testupdate").C("testEtag").FindId("1234").One(&d)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, map[string]interface{}{"foo": "baz", "_id": "1234", "_etag": "etag", "_updated": now}, d)

	// Update an original item with ETag over item in DB with _etag,
	// fails because _etag is present
	err = h.Update(context.Background(), item, originalItem)
	assert.Equal(t, resource.ErrConflict, err)
}

func TestDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	s, err := mgo.Dial("")
	if !assert.NoError(t, err) {
		return
	}
	defer cleanup(s, "testupdate")()
	h := NewHandler(s, "testupdate", "test")
	item := &resource.Item{
		ID:      "1234",
		ETag:    "etag1",
		Updated: now,
		Payload: map[string]interface{}{
			"id":  "1234",
			"foo": "bar",
		},
	}

	// Can't delete a non existing item
	err = h.Delete(context.Background(), item)
	assert.Equal(t, resource.ErrNotFound, err)

	err = h.Insert(context.Background(), []*resource.Item{item})
	assert.NoError(t, err)
	err = h.Delete(context.Background(), item)
	assert.NoError(t, err)

	// Update refused if original item's etag doesn't match stored one
	err = h.Insert(context.Background(), []*resource.Item{item})
	assert.NoError(t, err)
	item.ETag = "etag2"
	err = h.Delete(context.Background(), item)
	assert.Equal(t, resource.ErrConflict, err)

	c := s.DB("testupdate").C("testEtag")
	// Add an item without _etag field
	c.Insert(map[string]interface{}{"foo": "bar", "_id": "1234", "_updated": now})
	c.Insert(map[string]interface{}{"foo": "bar", "_id": "12345", "_etag": "etag", "_updated": now})
	h2 := NewHandler(s, "testupdate", "testEtag")
	// A item without _etag field, is extracted with ETag in "p-[id]" format
	originalItem := &resource.Item{
		ID:      "1234",
		ETag:    "p-1234",
		Updated: now,
		Payload: map[string]interface{}{
			"id":  "1234",
			"foo": "baz",
		},
	}
	// Delete an original item with Etag over item in DB without _etag
	err = h2.Delete(context.Background(), originalItem)
	assert.NoError(t, err)

	originalItem.ID = "12345"
	// Delete an original item with Etag over item in DB with _etag
	// fails because _etag is present
	err = h2.Delete(context.Background(), originalItem)
	assert.Equal(t, resource.ErrConflict, err)
}

func TestClear(t *testing.T) {
	const (
		dbName = "testclearlimit"
		cName  = "test"
	)

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	s, err := mgo.Dial("")
	if !assert.NoError(t, err) {
		return
	}
	defer cleanup(s, dbName)()
	h := NewHandler(s, dbName, dbName)
	items := []*resource.Item{
		{ID: "1", Payload: map[string]interface{}{"id": "1", "name": "a"}},
		{ID: "2", Payload: map[string]interface{}{"id": "2", "name": "b"}},
		{ID: "3", Payload: map[string]interface{}{"id": "3", "name": "c"}},
		{ID: "4", Payload: map[string]interface{}{"id": "4", "name": "d"}},
	}

	err = h.Insert(context.Background(), items)
	assert.NoError(t, err)

	q, err := query.New("", `{name:{$in:["c","d"]}}`, "", nil)
	if assert.NoError(t, err) {
		deleted, err := h.Clear(context.Background(), q)
		assert.NoError(t, err)
		assert.Equal(t, 2, deleted)
	}
	assertCollectionIDs(t, s.DB(dbName).C(dbName), []string{"1", "2"})

	q, err = query.New("", `{id:"2"}`, "", nil)
	if assert.NoError(t, err) {
		deleted, err := h.Clear(context.Background(), q)
		assert.NoError(t, err)
		assert.Equal(t, 1, deleted)
	}
	assertCollectionIDs(t, s.DB(dbName).C(dbName), []string{"1"})
}
func TestClearLimit(t *testing.T) {
	const (
		dbName = "testclearlimit"
		cName  = "test"
	)

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	s, err := mgo.Dial("")
	if !assert.NoError(t, err) {
		return
	}
	defer cleanup(s, dbName)()
	h := NewHandler(s, dbName, cName)
	items := []*resource.Item{
		{ID: "1", Payload: map[string]interface{}{"id": "1", "name": "a"}},
		{ID: "2", Payload: map[string]interface{}{"id": "2", "name": "b"}},
		{ID: "3", Payload: map[string]interface{}{"id": "3", "name": "d"}}, // should be sorted after 4
		{ID: "4", Payload: map[string]interface{}{"id": "4", "name": "c"}}, // should be removed
	}

	err = h.Insert(context.Background(), items)
	require.NoError(t, err)

	q, err := query.New("", `{name:{$in:["c","d"]}}`, "name", &query.Window{Limit: 1})
	if assert.NoError(t, err) {
		deleted, err := h.Clear(context.Background(), q)
		assert.NoError(t, err)
		assert.Equal(t, 1, deleted)
	}
	assertCollectionIDs(t, s.DB(dbName).C(cName), []string{"1", "2", "3"})
}

func TestClearOffset(t *testing.T) {
	const (
		dbName = "testclearoffset"
		cName  = "test"
	)

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	s, err := mgo.Dial("")
	if !assert.NoError(t, err) {
		return
	}
	defer cleanup(s, dbName)()
	h := NewHandler(s, dbName, cName)
	items := []*resource.Item{
		{ID: "1", Payload: map[string]interface{}{"id": "1", "name": "a"}},
		{ID: "2", Payload: map[string]interface{}{"id": "2", "name": "b"}},
		{ID: "3", Payload: map[string]interface{}{"id": "3", "name": "d"}}, // should be sorted after 4, should be removed
		{ID: "4", Payload: map[string]interface{}{"id": "4", "name": "c"}}, // should be skipped
	}

	err = h.Insert(context.Background(), items)
	require.NoError(t, err)

	q, err := query.New("", `{name:{$in:["c","d"]}}`, "name", &query.Window{Offset: 1})
	if assert.NoError(t, err) {
		deleted, err := h.Clear(context.Background(), q)
		assert.NoError(t, err)
		assert.Equal(t, 1, deleted)
	}
	assertCollectionIDs(t, s.DB(dbName).C(cName), []string{"1", "2", "4"})
}

func TestFind(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	s, err := mgo.Dial("")
	if !assert.NoError(t, err) {
		return
	}
	defer cleanup(s, "testfind")()
	h := NewHandler(s, "testfind", "test")
	h2 := NewHandler(s, "testfind", "test2")
	items := []*resource.Item{
		{ID: "1", Payload: map[string]interface{}{"id": "1", "name": "a", "age": 1}},
		{ID: "2", Payload: map[string]interface{}{"id": "2", "name": "b", "age": 2}},
		{ID: "3", Payload: map[string]interface{}{"id": "3", "name": "c", "age": 3}},
		{ID: "4", Payload: map[string]interface{}{"id": "4", "name": "d", "age": 4}},
		{ID: "5", Payload: map[string]interface{}{"id": "5", "name": "rest-layer-regexp"}},
	}
	ctx := context.Background()
	assert.NoError(t, h.Insert(ctx, items))
	assert.NoError(t, h2.Insert(ctx, items))

	l, err := h.Find(ctx, &query.Query{})
	if assert.NoError(t, err) {
		assert.Equal(t, 5, l.Total)
		assert.Len(t, l.Items, 5)
		// Do not check result's content as its order is unpredictable
	}

	l, err = h.Find(ctx, &query.Query{Window: &query.Window{Limit: 0}})
	if assert.NoError(t, err) {
		assert.Equal(t, 5, l.Total)
		assert.Len(t, l.Items, 0)
	}

	l, err = h.Find(ctx, &query.Query{Window: &query.Window{Limit: -1, Offset: 2}})
	if assert.NoError(t, err) {
		assert.Equal(t, 5, l.Total)
		assert.Len(t, l.Items, 3)
	}

	l, err = h.Find(ctx, &query.Query{Window: &query.Window{Limit: -1, Offset: 5}})
	if assert.NoError(t, err) {
		assert.Equal(t, -1, l.Total)
		assert.Len(t, l.Items, 0)
	}

	l, err = h.Find(ctx, &query.Query{Window: &query.Window{Limit: -1, Offset: 6}})
	if assert.NoError(t, err) {
		assert.Equal(t, -1, l.Total)
		assert.Len(t, l.Items, 0)
	}

	q, err := query.New("", `{name:"c"}`, "", query.Page(1, 1, 0))
	if assert.NoError(t, err) {
		l, err = h.Find(ctx, q)
		if assert.NoError(t, err) {
			assert.Equal(t, -1, l.Total)
			if assert.Len(t, l.Items, 1) {
				item := l.Items[0]
				assert.Equal(t, "3", item.ID)
				assert.Equal(t, map[string]interface{}{"id": "3", "name": "c", "age": 3}, item.Payload)
				assert.Equal(t, "p-3", item.ETag)
			}
		}
	}

	q, err = query.New("", `{name:{$in:["c","d"]}}`, "name", query.Page(1, 100, 0))
	if assert.NoError(t, err) {
		l, err = h.Find(ctx, q)
		if assert.NoError(t, err) {
			assert.Equal(t, 2, l.Total)
			if assert.Len(t, l.Items, 2) {
				item := l.Items[0]
				assert.Equal(t, "3", item.ID)
				assert.Equal(t, map[string]interface{}{"id": "3", "name": "c", "age": 3}, item.Payload)
				item = l.Items[1]
				assert.Equal(t, "4", item.ID)
				assert.Equal(t, map[string]interface{}{"id": "4", "name": "d", "age": 4}, item.Payload)
			}
		}
	}

	q, err = query.New("", `{id:"3"}`, "", query.Page(1, 1, 0))
	if assert.NoError(t, err) {
		l, err = h.Find(ctx, q)
		if assert.NoError(t, err) {
			assert.Equal(t, -1, l.Total)
			if assert.Len(t, l.Items, 1) {
				item := l.Items[0]
				assert.Equal(t, "3", item.ID)
				assert.Equal(t, map[string]interface{}{"id": "3", "name": "c", "age": 3}, item.Payload)
			}
		}
	}

	q, err = query.New("", `{name:{$regex:"^re[s]{1}t-.+yer.+exp$"}}`, "", query.Page(1, 1, 0))
	if assert.NoError(t, err) {
		l, err = h.Find(ctx, q)
		if assert.NoError(t, err) {
			assert.Equal(t, -1, l.Total)
			if assert.Len(t, l.Items, 1) {
				item := l.Items[0]
				assert.Equal(t, "5", item.ID)
				assert.Equal(t, map[string]interface{}{"id": "5", "name": "rest-layer-regexp"}, item.Payload)
			}
		}
	}

	q, err = query.New("", `{id:"10"}`, "", query.Page(1, 1, 0))
	if assert.NoError(t, err) {
		l, err = h.Find(ctx, q)
		if assert.NoError(t, err) {
			assert.Equal(t, 0, l.Total)
			assert.Len(t, l.Items, 0)
		}
	}

	q, err = query.New("", `{id:{$in:["3","4","10"]}}`, "", nil)
	if assert.NoError(t, err) {
		l, err = h.Find(ctx, q)
		if assert.NoError(t, err) {
			assert.Equal(t, 2, l.Total)
			assert.Len(t, l.Items, 2)
		}
	}
}
