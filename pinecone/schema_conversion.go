package pinecone

import (
	"encoding/json"
	"fmt"
	"sort"

	db_control "github.com/pinecone-io/go-pinecone/v6/internal/gen/db_control"
)

// Reserved schema field names the 2026-07 vectors API addresses classic vector data by. A schema
// whose fields are exactly these names (and nothing else) creates a classic vector index served by
// the vectors API — the same index earlier API versions created from dimension/metric/vector_type.
const (
	reservedDenseFieldName  = "_values"
	reservedSparseFieldName = "_sparse_values"
)

// classicVectorSchema builds the 2026-07 schema equivalent of the legacy dimension/metric/vector_type
// index declaration, addressing the vector by the reserved field name for the given vector type.
func classicVectorSchema(vectorType string, dimension *int32, metric *IndexMetric) (db_control.CreateIndexSchema, error) {
	fields := map[string]db_control.CreateIndexSchemaField{}

	if vectorType == "sparse" {
		// A sparse field takes no dimension and no metric; both were implied by vector_type="sparse".
		field, err := rawSchemaField(map[string]interface{}{"type": "sparse_vector"})
		if err != nil {
			return db_control.CreateIndexSchema{}, err
		}
		fields[reservedSparseFieldName] = field
		return db_control.CreateIndexSchema{Fields: fields}, nil
	}

	if dimension == nil {
		return db_control.CreateIndexSchema{}, fmt.Errorf("dimension is required for dense indexes")
	}
	resolvedMetric := "cosine"
	if metric != nil {
		resolvedMetric = string(*metric)
	}
	field, err := rawSchemaField(map[string]interface{}{
		"type":      "dense_vector",
		"dimension": *dimension,
		"metric":    resolvedMetric,
	})
	if err != nil {
		return db_control.CreateIndexSchema{}, err
	}
	fields[reservedDenseFieldName] = field
	return db_control.CreateIndexSchema{Fields: fields}, nil
}

// rawSchemaField builds a CreateIndexSchemaField union value from exactly the given keys, so unset
// optional properties (e.g. description) are omitted from the wire entirely.
func rawSchemaField(value map[string]interface{}) (db_control.CreateIndexSchemaField, error) {
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
		var (
			wire map[string]interface{}
			err  error
		)
		switch {
		case field.DenseVector != nil:
			wire = map[string]interface{}{
				"type":      "dense_vector",
				"dimension": field.DenseVector.Dimension,
				"metric":    string(field.DenseVector.Metric),
			}
			if field.DenseVector.Description != nil {
				wire["description"] = *field.DenseVector.Description
			}
		case field.SparseVector != nil:
			wire = map[string]interface{}{"type": "sparse_vector"}
			if field.SparseVector.Description != nil {
				wire["description"] = *field.SparseVector.Description
			}
		case field.String != nil:
			if field.String.FullTextSearch == nil {
				return db_control.CreateIndexSchema{}, fmt.Errorf("field %q: a string field is accepted at creation time only with a full_text_search configuration; metadata fields are indexed automatically at upsert", name)
			}
			fts := map[string]interface{}{}
			if field.String.FullTextSearch.Language != nil {
				fts["language"] = *field.String.FullTextSearch.Language
			}
			if field.String.FullTextSearch.Stemming != nil {
				fts["stemming"] = *field.String.FullTextSearch.Stemming
			}
			if field.String.FullTextSearch.StopWords != nil {
				fts["stop_words"] = *field.String.FullTextSearch.StopWords
			}
			if field.String.FullTextSearch.Ngram != nil {
				ngram := map[string]interface{}{
					"min_gram": field.String.FullTextSearch.Ngram.MinGram,
					"max_gram": field.String.FullTextSearch.Ngram.MaxGram,
				}
				if field.String.FullTextSearch.Ngram.PrefixOnly != nil {
					ngram["prefix_only"] = *field.String.FullTextSearch.Ngram.PrefixOnly
				}
				fts["ngram"] = ngram
			}
			wire = map[string]interface{}{"type": "string", "full_text_search": fts}
			if field.String.Description != nil {
				wire["description"] = *field.String.Description
			}
		case field.SemanticText != nil:
			return db_control.CreateIndexSchema{}, fmt.Errorf("field %q: semantic_text fields cannot be declared at creation time; use CreateIndexForModel to create an integrated-embedding index", name)
		case field.StringList != nil || field.Boolean != nil || field.Float != nil || field.Integer != nil || field.LegacyMetadata != nil:
			return db_control.CreateIndexSchema{}, fmt.Errorf("field %q: metadata fields are not declared in the schema; they are indexed automatically at upsert", name)
		default:
			return db_control.CreateIndexSchema{}, fmt.Errorf("field %q: exactly one field type must be set on IndexSchemaField", name)
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
// deployment union. Managed and BYOC are the spec'd request deployment types; pod deployments are
// injected for the legacy CreatePodIndex path.
func toDbDeploymentRequest(deployment *IndexDeployment) (*db_control.IndexDeploymentRequest, error) {
	if deployment == nil {
		return nil, nil
	}

	set := 0
	for _, isSet := range []bool{deployment.Managed != nil, deployment.Pod != nil, deployment.Byoc != nil} {
		if isSet {
			set++
		}
	}
	if set != 1 {
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
		raw, err := json.Marshal(db_control.PodDeployment{
			DeploymentType: "pod",
			Environment:    deployment.Pod.Environment,
			PodType:        deployment.Pod.PodType,
			Replicas:       deployment.Pod.Replicas,
			Shards:         deployment.Pod.Shards,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal pod deployment: %w", err)
		}
		if err := request.UnmarshalJSON(raw); err != nil {
			return nil, fmt.Errorf("failed to build pod deployment: %w", err)
		}
	}
	return &request, nil
}

// toIndexSchema converts the generated response schema into the public IndexSchema. Fields with an
// unknown type are skipped rather than failing the whole response, so the SDK keeps working when the
// API introduces new field types.
func toIndexSchema(schema *db_control.IndexSchema) *IndexSchema {
	if schema == nil {
		return nil
	}

	fields := make(map[string]IndexSchemaField, len(schema.Fields))
	for name, field := range schema.Fields {
		typed, err := field.AsTypedIndexSchemaField()
		if err != nil {
			continue
		}
		discriminator, err := typed.Discriminator()
		if err != nil || discriminator == "" {
			// No "type" tag: a legacy metadata field carrying only "filterable".
			if legacy, err := field.AsLegacyMetadataField(); err == nil {
				fields[name] = IndexSchemaField{LegacyMetadata: &LegacyMetadataField{Filterable: legacy.Filterable}}
			}
			continue
		}

		switch discriminator {
		case "dense_vector":
			if dense, err := typed.AsDenseVectorField(); err == nil {
				fields[name] = IndexSchemaField{DenseVector: &DenseVectorField{
					Dimension:   dense.Dimension,
					Metric:      IndexMetric(dense.Metric),
					Description: dense.Description,
				}}
			}
		case "sparse_vector":
			if sparse, err := typed.AsSparseVectorField(); err == nil {
				fields[name] = IndexSchemaField{SparseVector: &SparseVectorField{Description: sparse.Description}}
			}
		case "semantic_text":
			if semantic, err := typed.AsSemanticTextField(); err == nil {
				fields[name] = IndexSchemaField{SemanticText: &SemanticTextField{
					Model:           semantic.Model,
					Dimension:       semantic.Dimension,
					Metric:          (*IndexMetric)(semantic.Metric),
					ReadParameters:  semantic.ReadParameters,
					WriteParameters: semantic.WriteParameters,
					Description:     semantic.Description,
				}}
			}
		case "string":
			if str, err := typed.AsResponseStringField(); err == nil {
				publicField := StringField{
					Filterable:  str.Filterable,
					Description: str.Description,
				}
				if str.FullTextSearch != nil {
					fts := &FullTextSearchConfig{
						Language:  &str.FullTextSearch.Language,
						Stemming:  &str.FullTextSearch.Stemming,
						StopWords: &str.FullTextSearch.StopWords,
					}
					if str.FullTextSearch.Ngram != nil {
						fts.Ngram = &NgramConfig{
							MinGram:    str.FullTextSearch.Ngram.MinGram,
							MaxGram:    str.FullTextSearch.Ngram.MaxGram,
							PrefixOnly: &str.FullTextSearch.Ngram.PrefixOnly,
						}
					}
					publicField.FullTextSearch = fts
				}
				fields[name] = IndexSchemaField{String: &publicField}
			}
		case "string_list":
			if list, err := typed.AsStringListField(); err == nil {
				fields[name] = IndexSchemaField{StringList: &StringListField{Filterable: list.Filterable, Description: list.Description}}
			}
		case "boolean":
			if boolean, err := typed.AsBooleanField(); err == nil {
				fields[name] = IndexSchemaField{Boolean: &BooleanField{Filterable: boolean.Filterable, Description: boolean.Description}}
			}
		case "float":
			if float, err := typed.AsFloatField(); err == nil {
				fields[name] = IndexSchemaField{Float: &FloatField{Filterable: float.Filterable, Description: float.Description}}
			}
		case "integer":
			if integer, err := typed.AsIntegerField(); err == nil {
				fields[name] = IndexSchemaField{Integer: &IntegerField{Filterable: integer.Filterable, Description: integer.Description}}
			}
		}
	}
	return &IndexSchema{Fields: fields}
}

// toIndexDeployment converts the generated response deployment union into the public IndexDeployment.
func toIndexDeployment(deployment db_control.IndexDeployment) (*IndexDeployment, error) {
	discriminator, err := deployment.Discriminator()
	if err != nil || discriminator == "" {
		return nil, nil
	}

	switch discriminator {
	case "managed":
		managed, err := deployment.AsManagedDeployment()
		if err != nil {
			return nil, err
		}
		return &IndexDeployment{Managed: &ManagedDeployment{
			Cloud:       Cloud(managed.Cloud),
			Region:      managed.Region,
			Environment: managed.Environment,
		}}, nil
	case "pod":
		pod, err := deployment.AsPodDeployment()
		if err != nil {
			return nil, err
		}
		return &IndexDeployment{Pod: &PodDeployment{
			Environment: pod.Environment,
			PodType:     pod.PodType,
			Replicas:    pod.Replicas,
			Shards:      pod.Shards,
		}}, nil
	case "byoc":
		byoc, err := deployment.AsByocDeployment()
		if err != nil {
			return nil, err
		}
		return &IndexDeployment{Byoc: &ByocDeployment{Environment: byoc.Environment}}, nil
	default:
		// Be permissive about deployment types this SDK version doesn't know.
		return nil, nil
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

// semanticTextFieldForCompat returns the name and config of the schema's semantic_text field, if
// exactly one exists. Field names are visited in sorted order for determinism.
func semanticTextFieldForCompat(schema *IndexSchema) (string, *SemanticTextField) {
	if schema == nil {
		return "", nil
	}
	names := make([]string, 0, len(schema.Fields))
	for name := range schema.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if field := schema.Fields[name]; field.SemanticText != nil {
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
// Vector-type resolution: classic dense vectors-API indexes always report both "_values" and
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
		index.Metric = Dotproduct
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
