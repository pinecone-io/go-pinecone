package pinecone

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
	suite.Run(t, new(ClientTests))
}

func (ts *ClientTests) SetupSuite() {
	apiKey := os.Getenv("API_KEY")
	require.NotEmpty(ts.T(), apiKey, "API_KEY env variable not set")

	ts.podIndex = os.Getenv("POD_INDEX_NAME")
	require.NotEmpty(ts.T(), ts.podIndex, "POD_INDEX_NAME env variable not set")

	ts.serverlessIndex = os.Getenv("SERVERLESS_INDEX_NAME")
	require.NotEmpty(ts.T(), ts.serverlessIndex, "SERVERLESS_INDEX_NAME env variable not set")

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
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(indexes), 0, "Expected at least one index to exist")
}

func (ts *ClientTests) TestCreatePodIndex() {
	name := uuid.New().String()

	defer func(ts *ClientTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreatePodIndex(context.Background(), &CreatePodIndexRequest{
		Name:        name,
		Dimension:   10,
		Metric:      Cosine,
		Environment: "us-east1-gcp",
		PodType:     "p1.x1",
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, idx.Name, "Index name does not match")
}

func (ts *ClientTests) TestCreateServerlessIndex() {
	name := uuid.New().String()

	defer func(ts *ClientTests, name string) {
		err := ts.deleteIndex(name)
		require.NoError(ts.T(), err)
	}(ts, name)

	idx, err := ts.client.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:      name,
		Dimension: 10,
		Metric:    Cosine,
		Cloud:     Aws,
		Region:    "us-west-2",
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, idx.Name, "Index name does not match")
}

func (ts *ClientTests) TestDescribeServerlessIndex() {
	index, err := ts.client.DescribeIndex(context.Background(), ts.serverlessIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.serverlessIndex, index.Name, "Index name does not match")
}

func (ts *ClientTests) TestDescribePodIndex() {
	index, err := ts.client.DescribeIndex(context.Background(), ts.podIndex)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), ts.podIndex, index.Name, "Index name does not match")
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
			require.NoError(ts.T(), err, "Error deleting collection")
		}
	}(ts, collectionNames)

	for _, name := range collectionNames {
		_, err := ts.client.CreateCollection(ctx, &CreateCollectionRequest{
			Name:   name,
			Source: ts.podIndex,
		})
		require.NoError(ts.T(), err, "Error creating collection")
	}

	// Call the method under test to list all collections
	collections, err := ts.client.ListCollections(ctx)
	require.NoError(ts.T(), err)
	require.Greater(ts.T(), len(collections), 2, "Expected at least three collections to exist")

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
	require.Equal(ts.T(), len(collectionNames), found, "Not all created collections were listed")
}

func (ts *ClientTests) TestDescribeCollection() {
	ctx := context.Background()
	collectionName := uuid.New().String()

	defer func(client *Client, ctx context.Context, collectionName string) {
		err := client.DeleteCollection(ctx, collectionName)
		require.NoError(ts.T(), err)
	}(&ts.client, ctx, collectionName)

	_, err := ts.client.CreateCollection(ctx, &CreateCollectionRequest{
		Name:   collectionName,
		Source: ts.podIndex,
	})
	require.NoError(ts.T(), err)

	collection, err := ts.client.DescribeCollection(ctx, collectionName)
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), collectionName, collection.Name, "Collection name does not match")
}

func (ts *ClientTests) TestCreateCollection() {
	name := uuid.New().String()
	sourceIndex := ts.podIndex

	defer func() {
		err := ts.client.DeleteCollection(context.Background(), name)
		require.NoError(ts.T(), err)
	}()

	collection, err := ts.client.CreateCollection(context.Background(), &CreateCollectionRequest{
		Name:   name,
		Source: sourceIndex,
	})
	require.NoError(ts.T(), err)
	require.Equal(ts.T(), name, collection.Name, "Collection name does not match")
}

func (ts *ClientTests) TestDeleteCollection() {
	collectionName := uuid.New().String()
	_, err := ts.client.CreateCollection(context.Background(), &CreateCollectionRequest{
		Name:   collectionName,
		Source: ts.podIndex,
	})
	require.NoError(ts.T(), err)

	err = ts.client.DeleteCollection(context.Background(), collectionName)
	require.NoError(ts.T(), err)
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
