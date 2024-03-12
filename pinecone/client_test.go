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

	ts.podIndex = os.Getenv("POD_INDEX_NAME")
	assert.NotEmptyf(ts.T(), ts.podIndex, "POD_INDEX_NAME env variable not set")

	ts.serverlessIndex = os.Getenv("SERVERLESS_INDEX_NAME")
	assert.NotEmptyf(ts.T(), ts.serverlessIndex, "SERVERLESS_INDEX_NAME env variable not set")

	client, err := NewClient(apiKey)
	if err != nil {
		ts.FailNow(err.Error())
	}
	ts.client = *client

	// this will clean up the project deleting all indexes and collections that are
	// named a UUID. Generally not needed as all tests are cleaning up after themselves
	// Left here as a convenience during active development.
	//deleteUUIDNamedResources(context.Background(), &ts.client)
}

func (ts *ClientTests) TestListIndexes() {
	indexes, err := ts.client.ListIndexes(context.Background())
	ts.Require().NoError(err)
	ts.Require().NotNil(indexes)
}

func (ts *ClientTests) TestCreatePodIndex() {
	name := uuid.New().String()

	defer func(ts *ClientTests, name string) {
		err := ts.deleteIndex(name)
		assert.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x1",
	})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), idx)
}

func (ts *ClientTests) TestCreateServerlessIndex() {
	name := uuid.New().String()

	defer func(ts *ClientTests, name string) {
		err := ts.deleteIndex(name)
		assert.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:      name,
		Dimension: 10,
		Metric:    Cosine,
		Cloud:     Aws,
		Region:    "us-west-2",
	})
	assert.NoError(ts.T(), err)
	assert.NotNil(ts.T(), idx)
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

func (ts *ClientTests) TestListCollections() {
	ctx := context.Background()

	var collectionNames []string
	for i := 0; i < 3; i++ {
		collectionName := uuid.New().String()
		collectionNames = append(collectionNames, collectionName)
	}

	defer func(ts *ClientTests, collectionNames []string) {
		for _, name := range collectionNames {
			err := ts.client.DeleteCollection(ctx, name)
			ts.Require().NoError(err)
		}
	}(ts, collectionNames)

	for _, name := range collectionNames {
		_, err := ts.client.CreateCollection(ctx, &CreateCollectionRequest{
			Name:   name,
			Source: ts.podIndex,
		})
		ts.Require().NoError(err)
	}

	// Call the method under test to list all collections
	collections, err := ts.client.ListCollections(ctx)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(collections)

	// Check that the created collections are in the returned list
	found := 0
	for _, collection := range collections {
		for _, name := range collectionNames {
			if collection.Name == name {
				found++
				break
			}
		}
	}
	ts.Require().Equal(len(collectionNames), found, "Not all created collections were listed")
}

func (ts *ClientTests) TestDescribeCollection() {
	ctx := context.Background()
	collectionName := uuid.New().String()

	defer func(ts *ClientTests, ctx context.Context, collectionName string) {
		err := ts.client.DeleteCollection(ctx, collectionName)
		ts.Require().NoError(err)
	}(ts, ctx, collectionName)

	_, err := ts.client.CreateCollection(ctx, &CreateCollectionRequest{
		Name:   collectionName,
		Source: ts.podIndex,
	})
	ts.Require().NoError(err)

	collection, err := ts.client.DescribeCollection(ctx, collectionName)
	ts.Require().NoError(err)
	ts.Require().NotNil(collection)
	ts.Require().Equal(collectionName, collection.Name)
}

func (ts *ClientTests) TestCreateCollection() {
	name := uuid.New().String()
	sourceIndex := ts.podIndex

	defer func(client *Client, ctx context.Context, collectionName string) {
		err := client.DeleteCollection(ctx, collectionName)
		if err != nil {

		}
	}(&ts.client, context.Background(), name)

	collection, err := ts.client.CreateCollection(context.Background(), &CreateCollectionRequest{
		Name:   name,
		Source: sourceIndex,
	})
	ts.Require().NoError(err)
	ts.Require().NotNil(collection)

	ts.Require().Equal(name, collection.Name)
}

func (ts *ClientTests) TestDeleteCollection() {
	collectionName := uuid.New().String()
	_, err := ts.client.CreateCollection(context.Background(), &CreateCollectionRequest{
		Name:   collectionName,
		Source: ts.podIndex,
	})
	ts.Require().NoError(err)

	err = ts.client.DeleteCollection(context.Background(), collectionName)
	ts.Require().NoError(err)
}

func (ts *ClientTests) deleteIndex(name string) error {
	return ts.client.DeleteIndex(context.Background(), name)
}

// Helper function to check if a name is a valid UUID
func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func deleteUUIDNamedResources(ctx context.Context, c *Client) error {
	// Delete UUID-named indexes
	indexes, err := c.ListIndexes(ctx)
	if err != nil {
		return err
	}

	for _, index := range indexes {
		if isValidUUID(index.Name) {
			err := c.DeleteIndex(ctx, index.Name)
			if err != nil {
				return err
			}
		}
	}

	// Delete UUID-named collections
	collections, err := c.ListCollections(ctx)
	if err != nil {
		return err
	}

	for _, collection := range collections {
		if isValidUUID(collection.Name) {
			err := c.DeleteCollection(ctx, collection.Name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
