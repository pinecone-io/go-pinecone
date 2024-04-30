package pinecone

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type IndexConnectionTests struct {
	suite.Suite
	host             string
	dimension        int32
	apiKey           string
	idxConn          *IndexConnection
	sourceTag        string
	metadata         map[string]string
	idxConnSourceTag *IndexConnection
	vectorIds        []string
}

// Runs the test suite with `go test`
func TestIndexConnection(t *testing.T) {
	apiKey := os.Getenv("API_KEY")
	assert.NotEmptyf(t, apiKey, "API_KEY env variable not set")

	client, err := NewClient(NewClientParams{ApiKey: apiKey})
	if err != nil {
		t.FailNow()
	}

	podIndexName := os.Getenv("TEST_POD_INDEX_NAME")
	assert.NotEmptyf(t, podIndexName, "TEST_POD_INDEX_NAME env variable not set")

	podIdx, err := client.DescribeIndex(context.Background(), podIndexName)
	if err != nil {
		t.FailNow()
	}

	podTestSuite := new(IndexConnectionTests)
	podTestSuite.host = podIdx.Host
	podTestSuite.dimension = podIdx.Dimension
	podTestSuite.apiKey = apiKey

	serverlessIndexName := os.Getenv("TEST_SERVERLESS_INDEX_NAME")
	assert.NotEmptyf(t, serverlessIndexName, "TEST_SERVERLESS_INDEX_NAME env variable not set")

	serverlessIdx, err := client.DescribeIndex(context.Background(), serverlessIndexName)
	if err != nil {
		t.FailNow()
	}

	serverlessTestSuite := new(IndexConnectionTests)
	serverlessTestSuite.host = serverlessIdx.Host
	serverlessTestSuite.dimension = serverlessIdx.Dimension
	serverlessTestSuite.apiKey = apiKey

	suite.Run(t, podTestSuite)
	suite.Run(t, serverlessTestSuite)
}

func (ts *IndexConnectionTests) SetupSuite() {
	assert.NotEmptyf(ts.T(), ts.host, "HOST env variable not set")
	assert.NotEmptyf(ts.T(), ts.apiKey, "API_KEY env variable not set")

	namespace, err := uuid.NewV7()
	assert.NoError(ts.T(), err)

	idxConn, err := newIndexConnection(ts.apiKey, ts.host, namespace.String(), "", nil)
	assert.NoError(ts.T(), err)
	ts.idxConn = idxConn

	ts.sourceTag = "test_source_tag"
	idxConnSourceTag, err := newIndexConnection(ts.apiKey, ts.host, namespace.String(), ts.sourceTag, nil)
	assert.NoError(ts.T(), err)
	ts.idxConnSourceTag = idxConnSourceTag

	ts.loadData()
}

func (ts *IndexConnectionTests) TearDownSuite() {
	ts.truncateData()

	err := ts.idxConn.Close()
	assert.NoError(ts.T(), err)

	err = ts.idxConnSourceTag.Close()
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) TestNewIndexConnection() {
	apiKey := "test-api-key"
	namespace := ""
	sourceTag := ""
	idxConn, err := newIndexConnection(apiKey, ts.host, namespace, sourceTag, nil)
	assert.NoError(ts.T(), err)

	if idxConn.apiKey != apiKey {
		ts.FailNow(fmt.Sprintf("Expected idxConn to have apiKey '%s', but got '%s'", apiKey, idxConn.apiKey))
	}
	if idxConn.Namespace != "" {
		ts.FailNow(fmt.Sprintf("Expected idxConn to have empty namespace, but got '%s'", idxConn.Namespace))
	}
	if idxConn.additionalMetadata != nil {
		ts.FailNow(fmt.Sprintf("Expected idxConn to have nil additionalMetadata, but got '%+v'", idxConn.additionalMetadata))
	}
	if idxConn.dataClient == nil {
		ts.FailNow("Expected idxConn to have non-nil dataClient")
	}
	if idxConn.grpcConn == nil {
		ts.FailNow("Expected idxConn to have non-nil grpcConn")
	}
	if idxConn.additionalMetadata != nil {
		ts.FailNow("Expected idxConn to have nil additionalMetadata")
	}
}

func (ts *IndexConnectionTests) TestNewIndexConnectionNamespace() {
	apiKey := "test-api-key"
	namespace := "test-namespace"
	sourceTag := "test-source-tag"
	idxConn, err := newIndexConnection(apiKey, ts.host, namespace, sourceTag, nil)
	assert.NoError(ts.T(), err)

	if idxConn.apiKey != apiKey {
		ts.FailNow(fmt.Sprintf("Expected idxConn to have apiKey '%s', but got '%s'", apiKey, idxConn.apiKey))
	}
	if idxConn.Namespace != namespace {
		ts.FailNow(fmt.Sprintf("Expected idxConn to have namespace '%s', but got '%s'", namespace, idxConn.Namespace))
	}
	if idxConn.dataClient == nil {
		ts.FailNow("Expected idxConn to have non-nil dataClient")
	}
	if idxConn.grpcConn == nil {
		ts.FailNow("Expected idxConn to have non-nil grpcConn")
	}
}

func (ts *IndexConnectionTests) TestNewIndexConnectionAdditionalMetadata() {
	apiKey := "test-api-key"
	namespace := "test-namespace"
	sourceTag := "test-source-tag"
	additionalMetadata := map[string]string{"test-header": "test-value"}
	idxConn, err := newIndexConnection(apiKey, ts.host, namespace, sourceTag, additionalMetadata)
	assert.NoError(ts.T(), err)

	if idxConn.apiKey != apiKey {
		ts.FailNow(fmt.Sprintf("Expected idxConn to have apiKey '%s', but got '%s'", apiKey, idxConn.apiKey))
	}
	if idxConn.Namespace != namespace {
		ts.FailNow(fmt.Sprintf("Expected idxConn to have namespace '%s', but got '%s'", namespace, idxConn.Namespace))
	}
	if  !reflect.DeepEqual(idxConn.additionalMetadata, additionalMetadata) {
		ts.FailNow(fmt.Sprintf("Expected idxConn to have additionalMetadata '%+v', but got '%+v'", additionalMetadata, idxConn.additionalMetadata))
	}	
	if idxConn.dataClient == nil {
		ts.FailNow("Expected idxConn to have non-nil dataClient")
	}
	if idxConn.grpcConn == nil {
		ts.FailNow("Expected idxConn to have non-nil grpcConn")
	}
}

func (ts *IndexConnectionTests) TestFetchVectors() {
	ctx := context.Background()
	res, err := ts.idxConn.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestFetchVectorsSourceTag() {
	ctx := context.Background()
	res, err := ts.idxConnSourceTag.FetchVectors(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryByVector() {
	vec := make([]float32, ts.dimension)
	for i := range vec {
		vec[i] = 0.01
	}

	req := &QueryByVectorValuesRequest{
		Vector: vec,
		TopK:   5,
	}

	ctx := context.Background()
	res, err := ts.idxConn.QueryByVectorValues(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryByVectorSourceTag() {
	vec := make([]float32, ts.dimension)
	for i := range vec {
		vec[i] = 0.01
	}

	req := &QueryByVectorValuesRequest{
		Vector: vec,
		TopK:   5,
	}

	ctx := context.Background()
	res, err := ts.idxConnSourceTag.QueryByVectorValues(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryById() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConn.QueryByVectorId(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryByIdSourceTag() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConnSourceTag.QueryByVectorId(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestDeleteVectorsById() {
	ctx := context.Background()
	err := ts.idxConn.DeleteVectorsById(ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDeleteVectorsByFilter() {
	ctx := context.Background()
	err := ts.idxConn.DeleteVectorsByFilter(ctx, &Filter{})
	assert.NoError(ts.T(), err)

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDeleteAllVectorsInNamespace() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDescribeIndexStats() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStats(ctx)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestDescribeIndexStatsFiltered() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStatsFiltered(ctx, &Filter{})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestListVectors() {
	ts.T().Skip()
	req := &ListVectorsRequest{}

	ctx := context.Background()
	res, err := ts.idxConn.ListVectors(ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) loadData() {
	vals := []float32{0.01, 0.02, 0.03, 0.04, 0.05}
	vectors := make([]*Vector, len(vals))
	ts.vectorIds = make([]string, len(vals))

	for i, val := range vals {
		vec := make([]float32, ts.dimension)
		for i := range vec {
			vec[i] = val
		}

		id := fmt.Sprintf("vec-%d", i+1)
		ts.vectorIds[i] = id

		vectors[i] = &Vector{
			Id:     id,
			Values: vec,
		}
	}

	ctx := context.Background()
	_, err := ts.idxConn.UpsertVectors(ctx, vectors)
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) loadDataSourceTag() {
	vals := []float32{0.01, 0.02, 0.03, 0.04, 0.05}
	vectors := make([]*Vector, len(vals))
	ts.vectorIds = make([]string, len(vals))

	for i, val := range vals {
		vec := make([]float32, ts.dimension)
		for i := range vec {
			vec[i] = val
		}

		id := fmt.Sprintf("vec-%d", i+1)
		ts.vectorIds[i] = id

		vectors[i] = &Vector{
			Id:     id,
			Values: vec,
		}
	}

	ctx := context.Background()
	_, err := ts.idxConnSourceTag.UpsertVectors(ctx, vectors)
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) truncateData() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(ctx)
	assert.NoError(ts.T(), err)
}
