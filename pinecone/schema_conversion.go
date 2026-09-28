package pinecone

import (
	"encoding/json"
	"fmt"

	db_control "github.com/pinecone-io/go-pinecone/v7/internal/gen/db_control"
)

// Reserved schema field names the vector operations address vector data by. A schema whose fields
// are exactly these names (and nothing else) creates a vector index used with the vector operations —
// the same index earlier API versions created from dimension/metric/vector_type.
const (
	reservedDenseFieldName  = "_values"
	reservedSparseFieldName = "_sparse_values"
)

// classicVectorSchema builds the schema equivalent of the legacy dimension/metric/vector_type
// index declaration, addressing the vector by the reserved field name for the given vector type.
func classicVectorSchema(vectorType string, dimension *int32, metric *IndexMetric) (db_control.CreateIndexSchema, error) {
	if vectorType == "sparse" {
		// A sparse field takes no dimension and no metric; both were implied by vector_type="sparse".
		return toDbCreateIndexSchema(IndexSchema{Fields: map[string]IndexSchemaField{
			reservedSparseFieldName: {SparseVector: &SparseVectorField{}},
		}})
	}

	if dimension == nil {
		return db_control.CreateIndexSchema{}, fmt.Errorf("dimension is required for dense indexes")
	}
	return toDbCreateIndexSchema(IndexSchema{Fields: map[string]IndexSchemaField{
		reservedDenseFieldName: {DenseVector: &DenseVectorField{Dimension: *dimension, Metric: derefOrDefault(metric, IndexMetricCosine)}},
	}})
}

// rawSchemaField builds a CreateIndexSchemaField union value from the JSON encoding of value. Callers
// pass the public field types rather than the generated ones because the generated types send unset
// optional properties (e.g. description) as null instead of omitting them.
func rawSchemaField(value any) (db_control.CreateIndexSchemaField, error) {
	var field db_control.CreateIndexSchemaField
	raw, err := json.Marshal(value)
	if err != nil {
		return field, fmt.Errorf("failed to marshal schema field: %w", err)
	}
	if err := field.UnmarshalJSON(raw); err != nil {
		return field, fmt.Errorf("failed to build schema field: %w", err)
	}
	return field, nil
}

// toDbCreateIndexSchema converts a public IndexSchema into the generated create-request schema.
// Only dense_vector, sparse_vector, and full-text-search string fields may be declared at creation
// time; other field types are rejected before any request is sent.
func toDbCreateIndexSchema(schema IndexSchema) (db_control.CreateIndexSchema, error) {
	fields := make(map[string]db_control.CreateIndexSchemaField, len(schema.Fields))
	for name, field := range schema.Fields {
		set := countSet(field.DenseVector != nil, field.SparseVector != nil, field.SemanticText != nil,
			field.String != nil, field.StringList != nil, field.Boolean != nil, field.Float != nil,
			field.Integer != nil, field.LegacyMetadata != nil)
		if set != 1 {
			return db_control.CreateIndexSchema{}, fmt.Errorf("field %q: exactly one field type must be set on IndexSchemaField", name)
		}

		var wire any
		switch {
		case field.DenseVector != nil:
			wire = struct {
				Type string `json:"type"`
				*DenseVectorField
			}{"dense_vector", field.DenseVector}
		case field.SparseVector != nil:
			wire = struct {
				Type string `json:"type"`
				*SparseVectorField
			}{"sparse_vector", field.SparseVector}
		case field.String != nil:
			if field.String.FullTextSearch == nil {
				return db_control.CreateIndexSchema{}, fmt.Errorf("field %q: a StringField can only be declared with FullTextSearch set; other metadata fields are indexed automatically when you upsert data", name)
			}
			str := *field.String
			str.Filterable = nil // Filterable is only reported in responses.
			wire = struct {
				Type string `json:"type"`
				*StringField
			}{"string", &str}
		case field.SemanticText != nil:
			return db_control.CreateIndexSchema{}, fmt.Errorf("field %q: SemanticText fields can't be declared with CreateIndex; use CreateIndexForModel to create an index with integrated embedding", name)
		default:
			return db_control.CreateIndexSchema{}, fmt.Errorf("field %q: metadata fields don't need to be declared; they are indexed automatically when you upsert data", name)
		}

		dbField, err := rawSchemaField(wire)
		if err != nil {
			return db_control.CreateIndexSchema{}, err
		}
		fields[name] = dbField
	}
	return db_control.CreateIndexSchema{Fields: fields}, nil
}

// toDbDeploymentRequest converts a public IndexDeployment into the generated create-request
// deployment union. Managed and BYOC are the only deployment types the 2026-07 create request
// accepts; the backend rejects pod creation on this API version.
func toDbDeploymentRequest(deployment *IndexDeployment) (*db_control.IndexDeploymentRequest, error) {
	if deployment == nil {
		return nil, nil
	}

	if countSet(deployment.Managed != nil, deployment.Pod != nil, deployment.Byoc != nil) != 1 {
		return nil, fmt.Errorf("exactly one of Managed, Pod, or Byoc must be set on IndexDeployment")
	}

	var request db_control.IndexDeploymentRequest
	switch {
	case deployment.Managed != nil:
		err := request.FromManagedDeployment(db_control.ManagedDeployment{
			DeploymentType: "managed",
			Cloud:          string(deployment.Managed.Cloud),
			Region:         deployment.Managed.Region,
		})
		if err != nil {
			return nil, err
		}
	case deployment.Byoc != nil:
		err := request.FromByocDeployment(db_control.ByocDeployment{
			DeploymentType: "byoc",
			Environment:    deployment.Byoc.Environment,
		})
		if err != nil {
			return nil, err
		}
	case deployment.Pod != nil:
		return nil, fmt.Errorf("creating pod-based indexes is not supported by Pinecone API version 2026-07; use a Managed or Byoc deployment instead. Existing pod-based indexes can still be used and scaled")
	}
	return &request, nil
}

// toIndexSchema converts the generated response schema into the public IndexSchema. Each field is
// decoded straight into the matching public type, whose JSON tags mirror the wire names. Fields with
// an unknown type are skipped rather than failing the whole response, so the SDK keeps working when
// the API introduces new field types.
func toIndexSchema(schema *db_control.IndexSchema) *IndexSchema {
	if schema == nil {
		return nil
	}

	fields := make(map[string]IndexSchemaField, len(schema.Fields))
	for name, field := range schema.Fields {
		raw, err := field.MarshalJSON()
		if err != nil {
			continue
		}
		var tagged struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &tagged); err != nil {
			continue
		}

		var publicField IndexSchemaField
		var target any
		switch tagged.Type {
		case "dense_vector":
			publicField.DenseVector = &DenseVectorField{}
			target = publicField.DenseVector
		case "sparse_vector":
			publicField.SparseVector = &SparseVectorField{}
			target = publicField.SparseVector
		case "semantic_text":
			publicField.SemanticText = &SemanticTextField{}
			target = publicField.SemanticText
		case "string":
			publicField.String = &StringField{}
			target = publicField.String
		case "string_list":
			publicField.StringList = &StringListField{}
			target = publicField.StringList
		case "boolean":
			publicField.Boolean = &BooleanField{}
			target = publicField.Boolean
		case "float":
			publicField.Float = &FloatField{}
			target = publicField.Float
		case "integer":
			publicField.Integer = &IntegerField{}
			target = publicField.Integer
		case "":
			// No "type" tag: a legacy metadata field carrying only "filterable".
			publicField.LegacyMetadata = &LegacyMetadataField{}
			target = publicField.LegacyMetadata
		default:
			continue
		}

		if err := json.Unmarshal(raw, target); err != nil {
			continue
		}
		fields[name] = publicField
	}
	return &IndexSchema{Fields: fields}
}

// toIndexDeployment converts the generated response deployment union into the public IndexDeployment.
// A deployment of an unknown type, or one that can't be decoded, is reported as nil rather than
// failing the whole response, matching how toIndexSchema treats fields it can't read.
func toIndexDeployment(deployment db_control.IndexDeployment) *IndexDeployment {
	discriminator, err := deployment.Discriminator()
	if err != nil {
		return nil
	}

	switch discriminator {
	case "managed":
		managed, err := deployment.AsManagedDeployment()
		if err != nil {
			return nil
		}
		return &IndexDeployment{Managed: &ManagedDeployment{
			Cloud:       Cloud(managed.Cloud),
			Region:      managed.Region,
			Environment: managed.Environment,
		}}
	case "pod":
		pod, err := deployment.AsPodDeployment()
		if err != nil {
			return nil
		}
		return &IndexDeployment{Pod: &PodDeployment{
			Environment: pod.Environment,
			PodType:     pod.PodType,
			Replicas:    pod.Replicas,
			Shards:      pod.Shards,
		}}
	case "byoc":
		byoc, err := deployment.AsByocDeployment()
		if err != nil {
			return nil
		}
		return &IndexDeployment{Byoc: &ByocDeployment{Environment: byoc.Environment}}
	default:
		return nil
	}
}

// denseFieldForCompat picks the dense vector field the deprecated Dimension/Metric/VectorType
// accessors resolve to: the reserved "_values" field when present, otherwise a sole dense field.
func denseFieldForCompat(schema *IndexSchema) *DenseVectorField {
	if schema == nil {
		return nil
	}
	if field, ok := schema.Fields[reservedDenseFieldName]; ok && field.DenseVector != nil {
		return field.DenseVector
	}
	var found *DenseVectorField
	for _, field := range schema.Fields {
		if field.DenseVector != nil {
			if found != nil {
				return nil // ambiguous: more than one dense field, no single compat answer
			}
			found = field.DenseVector
		}
	}
	return found
}

// semanticTextFieldForCompat returns the name and config of the schema's semantic_text field, if any.
// An index has at most one: it's created by CreateIndexForModel with a single field_map entry.
func semanticTextFieldForCompat(schema *IndexSchema) (string, *SemanticTextField) {
	if schema == nil {
		return "", nil
	}
	for name, field := range schema.Fields {
		if field.SemanticText != nil {
			return name, field.SemanticText
		}
	}
	return "", nil
}

// hasSparseField reports whether the schema declares any sparse vector field.
func hasSparseField(schema *IndexSchema) bool {
	if schema == nil {
		return false
	}
	for _, field := range schema.Fields {
		if field.SparseVector != nil {
			return true
		}
	}
	return false
}

// applyIndexCompatFields populates the deprecated computed fields (Metric, VectorType, Dimension,
// Spec, Embed) on an Index from its 2026-07 Schema and Deployment.
//
// Vector-type resolution: dense vector indexes always report both "_values" and
// "_sparse_values" regardless of how they were created, so a dense field wins over a sparse one.
// A sparse-only schema resolves to "sparse" with the implied "dotproduct" metric. A full-text-search
// index with no vector field leaves all three unset.
func applyIndexCompatFields(index *Index) {
	semanticName, semantic := semanticTextFieldForCompat(index.Schema)
	if semantic != nil {
		index.Embed = &IndexEmbed{
			Model:           semantic.Model,
			Dimension:       semantic.Dimension,
			Metric:          semantic.Metric,
			FieldMap:        &map[string]interface{}{"text": semanticName},
			ReadParameters:  semantic.ReadParameters,
			WriteParameters: semantic.WriteParameters,
		}
	}

	if dense := denseFieldForCompat(index.Schema); dense != nil {
		index.VectorType = "dense"
		dimension := dense.Dimension
		index.Dimension = &dimension
		index.Metric = dense.Metric
	} else if hasSparseField(index.Schema) {
		index.VectorType = "sparse"
		index.Metric = IndexMetricDotproduct
	} else if semantic != nil {
		index.Dimension = semantic.Dimension
		if semantic.Metric != nil {
			index.Metric = *semantic.Metric
		}
		if semantic.Dimension != nil {
			index.VectorType = "dense"
		} else {
			index.VectorType = "sparse"
		}
	}

	if index.Deployment != nil {
		spec := &IndexSpec{}
		switch {
		case index.Deployment.Managed != nil:
			spec.Serverless = &ServerlessSpec{
				Cloud:            index.Deployment.Managed.Cloud,
				Region:           index.Deployment.Managed.Region,
				SourceCollection: index.SourceCollection,
				ReadCapacity:     index.ReadCapacity,
			}
		case index.Deployment.Pod != nil:
			replicas := derefOrDefault(index.Deployment.Pod.Replicas, 1)
			shards := derefOrDefault(index.Deployment.Pod.Shards, 1)
			spec.Pod = &PodSpec{
				Environment:      index.Deployment.Pod.Environment,
				PodType:          index.Deployment.Pod.PodType,
				PodCount:         int(replicas * shards),
				Replicas:         replicas,
				ShardCount:       shards,
				SourceCollection: index.SourceCollection,
			}
		case index.Deployment.Byoc != nil:
			spec.BYOC = &BYOCSpec{
				Environment:  index.Deployment.Byoc.Environment,
				ReadCapacity: index.ReadCapacity,
			}
		}
		index.Spec = spec
	}
}

func countSet(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}
