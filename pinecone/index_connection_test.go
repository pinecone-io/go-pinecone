package pinecone

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"os"
	"testing"
)

type IndexConnectionTests struct {
	suite.Suite
	host      string
	apiKey    string
	idxConn   *IndexConnection
	vectorIds []string
}

// Runs the test suite with `go test`
func TestIndexConnection(t *testing.T) {
	apiKey := os.Getenv("API_KEY")

	podTestSuite := new(IndexConnectionTests)
	podTestSuite.host = os.Getenv("POD_INDEX_HOST")
	podTestSuite.apiKey = apiKey

	serverlessTestSuite := new(IndexConnectionTests)
	serverlessTestSuite.host = os.Getenv("SERVERLESS_INDEX_HOST")
	serverlessTestSuite.apiKey = apiKey

	suite.Run(t, podTestSuite)
	suite.Run(t, serverlessTestSuite)
}

func (ts *IndexConnectionTests) SetupSuite() {
	assert.NotEmptyf(ts.T(), ts.host, "HOST env variable not set")
	assert.NotEmptyf(ts.T(), ts.apiKey, "API_KEY env variable not set")

	namespace, err := uuid.NewV7()
	assert.NoError(ts.T(), err)

	idxConn, err := newIndexConnection(ts.apiKey, ts.host, namespace.String())
	assert.NoError(ts.T(), err)
	ts.idxConn = idxConn

	ts.loadData()
}

func (ts *IndexConnectionTests) TearDownSuite() {
	ts.truncateData()

	err := ts.idxConn.Close()
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) TestFetchVectors() {
	ctx := context.Background()
	res, err := ts.idxConn.FetchVectors(&ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryByVector() {
	req := &QueryByVectorValuesRequest{
		Vector: []float32{0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		TopK:   5,
	}

	ctx := context.Background()
	res, err := ts.idxConn.QueryByVectorValues(&ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryById() {
	req := &QueryByVectorIdRequest{
		VectorId: ts.vectorIds[0],
		TopK:     5,
	}

	ctx := context.Background()
	res, err := ts.idxConn.QueryByVectorId(&ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestDeleteVectorsById() {
	ctx := context.Background()
	err := ts.idxConn.DeleteVectorsById(&ctx, ts.vectorIds)
	assert.NoError(ts.T(), err)

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDeleteVectorsByFilter() {
	ctx := context.Background()
	err := ts.idxConn.DeleteVectorsByFilter(&ctx, &Filter{})
	assert.NoError(ts.T(), err)

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDeleteAllVectorsInNamespace() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(&ctx)
	assert.NoError(ts.T(), err)

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDescribeIndexStats() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStats(&ctx)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestDescribeIndexStatsFiltered() {
	ctx := context.Background()
	res, err := ts.idxConn.DescribeIndexStatsFiltered(&ctx, &Filter{})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestListVectors() {
	ts.T().Skip()
	req := &ListVectorsRequest{}

	ctx := context.Background()
	res, err := ts.idxConn.ListVectors(&ctx, req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) loadData() {
	vectors := []*Vector{
		&Vector{
			Id:     "vec-1",
			Values: []float32{0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		},
		&Vector{
			Id:     "vec-2",
			Values: []float32{0.02, 0.02, 0.02, 0.02, 0.02, 0.02, 0.02, 0.02, 0.02, 0.02},
		},
	}

	ts.vectorIds = []string{"vec-1", "vec-2"}

	ctx := context.Background()
	_, err := ts.idxConn.UpsertVectors(&ctx, vectors)
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) truncateData() {
	ctx := context.Background()
	err := ts.idxConn.DeleteAllVectorsInNamespace(&ctx)
	assert.NoError(ts.T(), err)
}
