package pinecone

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"os"
	"testing"
)

type ClientTests struct {
	suite.Suite
	client          Client
	podIndex        string
	serverlessIndex string
}

func TestClient(t *testing.T) {
	testSuite := new(ClientTests)
	suite.Run(t, testSuite)
}

func (ts *ClientTests) SetupSuite() {
	apiKey := os.Getenv("API_KEY")
	assert.NotEmptyf(ts.T(), apiKey, "API_KEY env variable not set")

	client, err := NewClient(apiKey)
	if err != nil {
		ts.FailNow(err.Error())
	}
	ts.client = *client

	ts.podIndex = uuid.New().String()
	_, err = ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        ts.podIndex,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x1",
	})
	assert.NoError(ts.T(), err)

	ts.serverlessIndex = uuid.New().String()
	_, err = ts.client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:      ts.serverlessIndex,
		Dimension: 10,
		Metric:    Cosine,
		Cloud:     Aws,
		Region:    "us-west-2",
	})
	assert.NoError(ts.T(), err)
}

func (ts *ClientTests) TeardownSuite() {
	ts.deleteIndex(ts.podIndex)
	ts.deleteIndex(ts.serverlessIndex)
}

func (ts *ClientTests) TestListIndexes() {
	indexes, err := ts.client.ListIndexes(context.Background())
	ts.Require().NoError(err)
	ts.Require().NotNil(indexes)
}

func (ts *ClientTests) TestCreatePodIndex() {
	id, err := uuid.NewV7()
	assert.NoError(ts.T(), err)

	name := id.String()

	idx, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x1",
	})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), idx)

	err = ts.deleteIndex(name)
	assert.NoError(ts.T(), err)
}

func (ts *ClientTests) TestCreateServerlessIndex() {
	id, err := uuid.NewV7()
	assert.NoError(ts.T(), err)

	name := id.String()

	idx, err := ts.client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:      name,
		Dimension: 10,
		Metric:    Cosine,
		Cloud:     Aws,
		Region:    "us-west-2",
	})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), idx)

	err = ts.deleteIndex(name)
	assert.NoError(ts.T(), err)
}

func (ts *ClientTests) TestDescribeServerlessIndex() {
	index, err := ts.client.DescribeIndex(context.Background(), ts.serverlessIndex)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), index)
}

func (ts *ClientTests) TestDescribePodIndex() {
	index, err := ts.client.DescribeIndex(context.Background(), ts.podIndex)
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), index)
}

func (ts *ClientTests) deleteIndex(name string) error {
	return ts.client.DeleteIndex(context.Background(), name)
}
