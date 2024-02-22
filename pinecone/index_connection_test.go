package pinecone

import (
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
	namespace string
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

	idxConn, err := newIndexConnection(ts.apiKey, ts.host)
	assert.NoError(ts.T(), err)
	ts.idxConn = idxConn

	namespace, err := uuid.NewV7()
	assert.NoError(ts.T(), err)
	ts.namespace = namespace.String()

	ts.loadData()
}

func (ts *IndexConnectionTests) TearDownSuite() {
	ts.truncateData()

	err := ts.idxConn.Close()
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) TestFetchVectors() {
	req := &FetchVectorsRequest{
		Ids:       ts.vectorIds,
		Namespace: ts.namespace,
	}

	res, err := ts.idxConn.FetchVectors(req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryByVector() {
	req := &QueryByVectorRequest{
		Vector:    []float32{0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		Namespace: ts.namespace,
		TopK:      5,
	}

	res, err := ts.idxConn.QueryByVector(req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestQueryById() {
	req := &QueryByIdRequest{
		Id:        ts.vectorIds[0],
		Namespace: ts.namespace,
		TopK:      5,
	}

	res, err := ts.idxConn.QueryById(req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestDeleteVectors() {
	req := &DeleteVectorsRequest{
		Ids:       ts.vectorIds,
		Namespace: ts.namespace,
	}

	err := ts.idxConn.DeleteVectors(req)
	assert.NoError(ts.T(), err)

	ts.loadData() //reload deleted data
}

func (ts *IndexConnectionTests) TestDescribeIndexStats() {
	req := &DescribeIndexStatsRequest{}
	res, err := ts.idxConn.DescribeIndexStats(req)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), res)
}

func (ts *IndexConnectionTests) TestListVectors() {
	ts.T().Skip()
	req := &ListVectorsRequest{
		Namespace: ts.namespace,
	}

	res, err := ts.idxConn.ListVectors(req)
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

	req := &UpsertVectorsRequest{
		Vectors:   vectors,
		Namespace: ts.namespace,
	}
	_, err := ts.idxConn.UpsertVectors(req)
	assert.NoError(ts.T(), err)
}

func (ts *IndexConnectionTests) truncateData() {
	err := ts.idxConn.DeleteVectors(&DeleteVectorsRequest{
		DeleteAll: true,
		Namespace: ts.namespace,
	})
	assert.NoError(ts.T(), err)
}
