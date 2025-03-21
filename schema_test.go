package analysis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"testing"

	"github.com/go-openapi/analysis/internal/antest"
	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var knownSchemas = []*spec.Schema{
	spec.BoolProperty(),                  // 0
	spec.StringProperty(),                // 1
	spec.Int8Property(),                  // 2
	spec.Int16Property(),                 // 3
	spec.Int32Property(),                 // 4
	spec.Int64Property(),                 // 5
	spec.Float32Property(),               // 6
	spec.Float64Property(),               // 7
	spec.DateProperty(),                  // 8
	spec.DateTimeProperty(),              // 9
	(&spec.Schema{}),                     // 10
	(&spec.Schema{}).Typed("object", ""), // 11
	(&spec.Schema{}).Typed("", ""),       // 12
	(&spec.Schema{}).Typed("", "uuid"),   // 13
}

func TestSchemaAnalysis_KnownTypes(t *testing.T) {
	for i, v := range knownSchemas {
		sch, err := Schema(SchemaOpts{Schema: v})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Truef(t, sch.IsKnownType, "item at %d should be a known type", i)
	}

	for i, v := range complexSchemas {
		sch, err := Schema(SchemaOpts{Schema: v})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Falsef(t, sch.IsKnownType, "item at %d should not be a known type", i)
	}

	serv := refServer()
	defer serv.Close()

	for i, ref := range knownRefs(serv.URL) {
		sch, err := Schema(SchemaOpts{Schema: refSchema(ref)})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Truef(t, sch.IsKnownType, "item at %d should be a known type", i)
	}

	for i, ref := range complexRefs(serv.URL) {
		sch, err := Schema(SchemaOpts{Schema: refSchema(ref)})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Falsef(t, sch.IsKnownType, "item at %d should not be a known type", i)
	}
}

func TestSchemaAnalysis_Array(t *testing.T) {
	for i, v := range append(knownSchemas, (&spec.Schema{}).Typed("array", "")) {
		sch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(v)})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Truef(t, sch.IsArray, "item at %d should be an array type", i)
		assert.Truef(t, sch.IsSimpleArray, "item at %d should be a simple array type", i)
	}

	for i, v := range complexSchemas {
		sch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(v)})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Truef(t, sch.IsArray, "item at %d should be an array type", i)
		assert.Falsef(t, sch.IsSimpleArray, "item at %d should not be a simple array type", i)
	}

	serv := refServer()
	defer serv.Close()

	for i, ref := range knownRefs(serv.URL) {
		sch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(refSchema(ref))})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Truef(t, sch.IsArray, "item at %d should be an array type", i)
		assert.Truef(t, sch.IsSimpleArray, "item at %d should be a simple array type", i)
	}

	for i, ref := range complexRefs(serv.URL) {
		sch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(refSchema(ref))})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Falsef(t, sch.IsKnownType, "item at %d should not be a known type", i)
		assert.Truef(t, sch.IsArray, "item at %d should be an array type", i)
		assert.Falsef(t, sch.IsSimpleArray, "item at %d should not be a simple array type", i)
	}

	// edge case: unrestricted array (beyond Swagger)
	at := spec.ArrayProperty(nil)
	at.Items = nil
	sch, err := Schema(SchemaOpts{Schema: at})
	require.NoError(t, err)
	assert.True(t, sch.IsArray)
	assert.False(t, sch.IsTuple)
	assert.False(t, sch.IsKnownType)
	assert.True(t, sch.IsSimpleSchema)

	// unrestricted array with explicit empty schema
	at = spec.ArrayProperty(nil)
	at.Items = &spec.SchemaOrArray{}
	sch, err = Schema(SchemaOpts{Schema: at})
	require.NoError(t, err)
	assert.True(t, sch.IsArray)
	assert.False(t, sch.IsTuple)
	assert.False(t, sch.IsKnownType)
	assert.True(t, sch.IsSimpleSchema)
}

func TestSchemaAnalysis_Map(t *testing.T) {
	for i, v := range append(knownSchemas, spec.MapProperty(nil)) {
		sch, err := Schema(SchemaOpts{Schema: spec.MapProperty(v)})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Truef(t, sch.IsMap, "item at %d should be a map type", i)
		assert.Truef(t, sch.IsSimpleMap, "item at %d should be a simple map type", i)
	}

	for i, v := range complexSchemas {
		sch, err := Schema(SchemaOpts{Schema: spec.MapProperty(v)})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Truef(t, sch.IsMap, "item at %d should be a map type", i)
		assert.Falsef(t, sch.IsSimpleMap, "item at %d should not be a simple map type", i)
	}
}

func TestSchemaAnalysis_ExtendedObject(t *testing.T) {
	for i, v := range knownSchemas {
		wex := spec.MapProperty(v).SetProperty("name", *spec.StringProperty())
		sch, err := Schema(SchemaOpts{Schema: wex})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Truef(t, sch.IsExtendedObject, "item at %d should be an extended map object type", i)
		assert.Falsef(t, sch.IsMap, "item at %d should not be a map type", i)
		assert.Falsef(t, sch.IsSimpleMap, "item at %d should not be a simple map type", i)
	}
}

func TestSchemaAnalysis_Tuple(t *testing.T) {
	at := spec.ArrayProperty(nil)
	at.Items = &spec.SchemaOrArray{}
	at.Items.Schemas = append(at.Items.Schemas, *spec.StringProperty(), *spec.Int64Property())

	sch, err := Schema(SchemaOpts{Schema: at})
	require.NoError(t, err)
	assert.True(t, sch.IsTuple)
	assert.False(t, sch.IsTupleWithExtra)
	assert.False(t, sch.IsKnownType)
	assert.False(t, sch.IsSimpleSchema)

	// edge case: tuple with a single element
	at.Items = &spec.SchemaOrArray{}
	at.Items.Schemas = append(at.Items.Schemas, *spec.StringProperty())
	sch, err = Schema(SchemaOpts{Schema: at})
	require.NoError(t, err)
	assert.True(t, sch.IsTuple)
	assert.False(t, sch.IsTupleWithExtra)
	assert.False(t, sch.IsKnownType)
	assert.False(t, sch.IsSimpleSchema)
}

func TestSchemaAnalysis_TupleWithExtra(t *testing.T) {
	at := spec.ArrayProperty(nil)
	at.Items = &spec.SchemaOrArray{}
	at.Items.Schemas = append(at.Items.Schemas, *spec.StringProperty(), *spec.Int64Property())
	at.AdditionalItems = &spec.SchemaOrBool{Allows: true}
	at.AdditionalItems.Schema = spec.Int32Property()

	sch, err := Schema(SchemaOpts{Schema: at})
	require.NoError(t, err)
	assert.False(t, sch.IsTuple)
	assert.True(t, sch.IsTupleWithExtra)
	assert.False(t, sch.IsKnownType)
	assert.False(t, sch.IsSimpleSchema)
}

func TestSchemaAnalysis_BaseType(t *testing.T) {
	cl := (&spec.Schema{}).Typed("object", "").SetProperty("type", *spec.StringProperty()).WithDiscriminator("type")

	sch, err := Schema(SchemaOpts{Schema: cl})
	require.NoError(t, err)
	assert.True(t, sch.IsBaseType)
	assert.False(t, sch.IsKnownType)
	assert.False(t, sch.IsSimpleSchema)
}

func TestSchemaAnalysis_SimpleSchema(t *testing.T) {
	for i, v := range append(knownSchemas, spec.ArrayProperty(nil), spec.MapProperty(nil)) {
		sch, err := Schema(SchemaOpts{Schema: v})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Truef(t, sch.IsSimpleSchema, "item at %d should be a simple schema", i)

		asch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(v)})
		require.NoErrorf(t, err, "failed to analyze array schema at %d: %v", i, err)
		assert.Truef(t, asch.IsSimpleSchema, "array item at %d should be a simple schema", i)

		msch, err := Schema(SchemaOpts{Schema: spec.MapProperty(v)})
		require.NoErrorf(t, err, "failed to analyze map schema at %d: %v", i, err)
		assert.Truef(t, msch.IsSimpleSchema, "map item at %d should be a simple schema", i)
	}

	for i, v := range complexSchemas {
		sch, err := Schema(SchemaOpts{Schema: v})
		require.NoErrorf(t, err, "failed to analyze schema at %d: %v", i, err)
		assert.Falsef(t, sch.IsSimpleSchema, "item at %d should not be a simple schema", i)
	}
}

func TestSchemaAnalys_InvalidSchema(t *testing.T) {
	// explore error cases in schema analysis:
	// the only cause for failure is a wrong $ref at some place
	bp := filepath.Join("fixtures", "bugs", "1602", "other-invalid-pointers.yaml")
	sp := antest.LoadOrFail(t, bp)

	// invalid ref not detected (no digging further)
	def := sp.Definitions["invalidRefInObject"]
	_, err := Schema(SchemaOpts{Schema: &def, Root: sp, BasePath: bp})
	require.NoError(t, err, "did not expect an error here, in spite of the underlying invalid $ref")

	def = sp.Definitions["invalidRefInTuple"]
	_, err = Schema(SchemaOpts{Schema: &def, Root: sp, BasePath: bp})
	require.NoError(t, err, "did not expect an error here, in spite of the underlying invalid $ref")

	// invalid ref detected (digging)
	schema := refSchema(spec.MustCreateRef("#/definitions/noWhere"))
	_, err = Schema(SchemaOpts{Schema: schema, Root: sp, BasePath: bp})
	require.Error(t, err, "expected an error here")

	def = sp.Definitions["invalidRefInMap"]
	_, err = Schema(SchemaOpts{Schema: &def, Root: sp, BasePath: bp})
	require.Error(t, err, "expected an error here")

	def = sp.Definitions["invalidRefInArray"]
	_, err = Schema(SchemaOpts{Schema: &def, Root: sp, BasePath: bp})
	require.Error(t, err, "expected an error here")

	def = sp.Definitions["indirectToInvalidRef"]
	_, err = Schema(SchemaOpts{Schema: &def, Root: sp, BasePath: bp})
	require.Error(t, err, "expected an error here")
}

func TestSchemaAnalysis_EdgeCases(t *testing.T) {
	t.Parallel()

	_, err := Schema(SchemaOpts{Schema: nil})
	require.Error(t, err)
}

/* helpers for the Schema test suite */

func newCObj() *spec.Schema {
	return (&spec.Schema{}).Typed("object", "").SetProperty("id", *spec.Int64Property())
}

var complexObject = newCObj()

var complexSchemas = []*spec.Schema{
	complexObject,
	spec.ArrayProperty(complexObject),
	spec.MapProperty(complexObject),
}

func knownRefs(base string) []spec.Ref {
	urls := []string{"bool", "string", "integer", "float", "date", "object", "format"}

	result := make([]spec.Ref, 0, len(urls))
	for _, u := range urls {
		result = append(result, spec.MustCreateRef(fmt.Sprintf("%s/%s", base, path.Join("known", u))))
	}

	return result
}

func complexRefs(base string) []spec.Ref {
	urls := []string{"object", "array", "map"}

	result := make([]spec.Ref, 0, len(urls))
	for _, u := range urls {
		result = append(result, spec.MustCreateRef(fmt.Sprintf("%s/%s", base, path.Join("complex", u))))
	}

	return result
}

func refServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/known/bool", schemaHandler(knownSchemas[0]))
	mux.Handle("/known/string", schemaHandler(knownSchemas[1]))
	mux.Handle("/known/integer", schemaHandler(knownSchemas[5]))
	mux.Handle("/known/float", schemaHandler(knownSchemas[6]))
	mux.Handle("/known/date", schemaHandler(knownSchemas[8]))
	mux.Handle("/known/object", schemaHandler(knownSchemas[11]))
	mux.Handle("/known/format", schemaHandler(knownSchemas[13]))

	mux.Handle("/complex/object", schemaHandler(complexSchemas[0]))
	mux.Handle("/complex/array", schemaHandler(complexSchemas[1]))
	mux.Handle("/complex/map", schemaHandler(complexSchemas[2]))

	return httptest.NewServer(mux)
}

func refSchema(ref spec.Ref) *spec.Schema {
	return &spec.Schema{SchemaProps: spec.SchemaProps{Ref: ref}}
}

func schemaHandler(schema *spec.Schema) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, schema)
	})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)

	if err := enc.Encode(data); err != nil {
		panic(err)
	}
}
