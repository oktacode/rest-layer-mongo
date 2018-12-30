package mongo

import (
	"reflect"
	"testing"

	"github.com/oktacode/rest-layer/resource"
	"github.com/oktacode/rest-layer/schema"
	"github.com/oktacode/rest-layer/schema/query"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

type UnsupportedExpression struct{}

func (u UnsupportedExpression) Match(p map[string]interface{}) bool {
	return false
}

func (u UnsupportedExpression) Prepare(v schema.Validator) error {
	return nil
}

func (u UnsupportedExpression) String() string {
	return ""
}

func TestTranslateAggregate(t *testing.T) {
	cases := []struct {
		aggregate string
		err       error
		want      bson.M
	}{
		{`{f:{$group:true}}`, nil, bson.M{"total": bson.M{"$sum": 1}, "_id": "$f"}},
	}
	for i := range cases {
		tc := cases[i]
		t.Run(tc.aggregate, func(t *testing.T) {
			got, err := translateAggregate(query.MustParseAggregate(tc.aggregate))
			if !reflect.DeepEqual(err, tc.err) {
				t.Errorf("translateAggregate error:\ngot:  %v\nwant: %v", err, tc.err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("translateAggregate:\ngot:  %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}

func TestTranslatePredicate(t *testing.T) {
	cases := []struct {
		predicate string
		err       error
		want      bson.M
	}{
		{`{id:"foo"}`, nil, bson.M{"_id": "foo"}},
		{`{f:"foo"}`, nil, bson.M{"f": "foo"}},
		{`{f:{$ne:"foo"}}`, nil, bson.M{"f": bson.M{"$ne": "foo"}}},
		{`{f:{$exists:true}}`, nil, bson.M{"f": bson.M{"$exists": true}}},
		{`{f:{$exists:false}}`, nil, bson.M{"f": bson.M{"$exists": false}}},
		{`{f:{$gt:1}}`, nil, bson.M{"f": bson.M{"$gt": float64(1)}}},
		{`{f:{$gte:1}}`, nil, bson.M{"f": bson.M{"$gte": float64(1)}}},
		{`{f:{$lt:1}}`, nil, bson.M{"f": bson.M{"$lt": float64(1)}}},
		{`{f:{$lte:1}}`, nil, bson.M{"f": bson.M{"$lte": float64(1)}}},
		{`{f:{$in:["foo","bar"]}}`, nil, bson.M{"f": bson.M{"$in": []interface{}{"foo", "bar"}}}},
		{`{f:{$nin:["foo","bar"]}}`, nil, bson.M{"f": bson.M{"$nin": []interface{}{"foo", "bar"}}}},
		{`{f:{$regex:"fo[o]{1}.+is.+some"}}`, nil, bson.M{"f": bson.M{"$regex": "fo[o]{1}.+is.+some"}}},
		{`{$and:[{f:"foo"},{f:"bar"}]}`, nil, bson.M{"$and": []bson.M{bson.M{"f": "foo"}, bson.M{"f": "bar"}}}},
		{`{$or:[{f:"foo"},{f:"bar"}]}`, nil, bson.M{"$or": []bson.M{bson.M{"f": "foo"}, bson.M{"f": "bar"}}}},
	}
	for i := range cases {
		tc := cases[i]
		t.Run(tc.predicate, func(t *testing.T) {
			got, err := translatePredicate(query.MustParsePredicate(tc.predicate))
			if !reflect.DeepEqual(err, tc.err) {
				t.Errorf("translatePredicate error:\ngot:  %v\nwant: %v", err, tc.err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("translatePredicate:\ngot:  %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}

func TestTranslatePredicateInvalid(t *testing.T) {
	var err error
	_, err = translatePredicate(query.Predicate{UnsupportedExpression{}})
	assert.Equal(t, resource.ErrNotImplemented, err)
	_, err = translatePredicate(query.Predicate{&query.And{UnsupportedExpression{}}})
	assert.Equal(t, resource.ErrNotImplemented, err)
	_, err = translatePredicate(query.Predicate{&query.Or{UnsupportedExpression{}}})
	assert.Equal(t, resource.ErrNotImplemented, err)
}

func TestGetSort(t *testing.T) {
	var s []string
	s = getSort(&query.Query{Sort: query.Sort{}})
	assert.Equal(t, []string{"_id"}, s)
	s = getSort(&query.Query{Sort: query.Sort{{Name: "id"}}})
	assert.Equal(t, []string{"_id"}, s)
	s = getSort(&query.Query{Sort: query.Sort{{Name: "f"}}})
	assert.Equal(t, []string{"f"}, s)
	s = getSort(&query.Query{Sort: query.Sort{{Name: "f", Reversed: true}}})
	assert.Equal(t, []string{"-f"}, s)
	s = getSort(&query.Query{Sort: query.Sort{{Name: "f"}, {Name: "f", Reversed: true}}})
	assert.Equal(t, []string{"f", "-f"}, s)
}
