package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/deepmap/oapi-codegen/v2/pkg/securityprovider"
	"github.com/pinecone-io/go-pinecone/internal/gen/control"
	"github.com/pinecone-io/go-pinecone/internal/provider"
	"github.com/pinecone-io/go-pinecone/internal/useragent"
)

type Client struct {
	apiKey     string
	restClient *control.Client
	sourceTag  string
	headers    map[string]string
}

type NewClientParams struct {
	ApiKey     string            // optional unless no Authorization header provided
	SourceTag  string            // optional
	Headers    map[string]string // optional
	RestClient *http.Client      // optional
}

func NewClient(in NewClientParams) (*Client, error) {
	clientOptions, err := buildClientOptions(in)
	if err != nil {
		return nil, err
	}

	client, err := control.NewClient("https://api.pinecone.io", clientOptions...)
	if err != nil {
		return nil, err
	}

	c := Client{apiKey: in.ApiKey, restClient: client, sourceTag: in.SourceTag, headers: in.Headers}
	return &c, nil
}

func (c *Client) Index(host string) (*IndexConnection, error) {
	return c.IndexWithAdditionalMetadata(host, "", nil)
}

func (c *Client) IndexWithNamespace(host string, namespace string) (*IndexConnection, error) {
	return c.IndexWithAdditionalMetadata(host, namespace, nil)
}

func (c *Client) IndexWithAdditionalMetadata(host string, namespace string, additionalMetadata map[string]string) (*IndexConnection, error) {
	idx, err := newIndexConnection(newIndexParameters{apiKey: c.apiKey, host: host, namespace: namespace, sourceTag: c.sourceTag, additionalMetadata: additionalMetadata})
	if err != nil {
		return nil, err
	}
	return idx, nil
}

func (c *Client) ListIndexes(ctx context.Context) ([]*Index, error) {
	res, err := c.restClient.ListIndexes(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var indexList control.IndexList
	err = json.NewDecoder(res.Body).Decode(&indexList)
	if err != nil {
		return nil, err
	}

	indexes := make([]*Index, len(*indexList.Indexes))
	for i, idx := range *indexList.Indexes {
		indexes[i] = toIndex(&idx)
	}

	return indexes, nil
}

type CreatePodIndexRequest struct {
	Name             string
	Dimension        int32
	Metric           IndexMetric
	Environment      string
	PodType          string
	Shards           int32
	Replicas         int32
	SourceCollection *string
	MetadataConfig   *PodSpecMetadataConfig
}

func (req CreatePodIndexRequest) ReplicaCount() *int32 {
	x := minOne(req.Replicas)
	return &x
}

func (req CreatePodIndexRequest) ShardCount() *int32 {
	x := minOne(req.Shards)
	return &x
}

func (req CreatePodIndexRequest) TotalCount() *int {
	x := int(*req.ReplicaCount() * *req.ShardCount())
	return &x
}

func (c *Client) CreatePodIndex(ctx context.Context, in *CreatePodIndexRequest) (*Index, error) {
	metric := control.IndexMetric(in.Metric)
	req := control.CreateIndexRequest{
		Name:      in.Name,
		Dimension: in.Dimension,
		Metric:    &metric,
	}

	//add the spec to req.
	//because this is defined as an anon struct in the generated code, it must match exactly here.
	req.Spec = control.CreateIndexRequest_Spec{
		Pod: &struct {
			Environment    string `json:"environment"`
			MetadataConfig *struct {
				Indexed *[]string `json:"indexed,omitempty"`
			} `json:"metadata_config,omitempty"`
			PodType          control.PodSpecPodType   `json:"pod_type"`
			Pods             *int                     `json:"pods,omitempty"`
			Replicas         *control.PodSpecReplicas `json:"replicas,omitempty"`
			Shards           *control.PodSpecShards   `json:"shards,omitempty"`
			SourceCollection *string                  `json:"source_collection,omitempty"`
		}{
			Environment:      in.Environment,
			PodType:          in.PodType,
			Pods:             in.TotalCount(),
			Replicas:         in.ReplicaCount(),
			Shards:           in.ShardCount(),
			SourceCollection: in.SourceCollection,
		},
	}
	if in.MetadataConfig != nil {
		req.Spec.Pod.MetadataConfig = &struct {
			Indexed *[]string `json:"indexed,omitempty"`
		}{
			Indexed: in.MetadataConfig.Indexed,
		}
	}

	res, err := c.restClient.CreateIndex(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		var errResp control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errResp)
		if err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		return nil, fmt.Errorf("failed to create index: %s", errResp.Error.Message)
	}

	return decodeIndex(res.Body)
}

type CreateServerlessIndexRequest struct {
	Name      string
	Dimension int32
	Metric    IndexMetric
	Cloud     Cloud
	Region    string
}

func (c *Client) CreateServerlessIndex(ctx context.Context, in *CreateServerlessIndexRequest) (*Index, error) {
	metric := control.IndexMetric(in.Metric)
	req := control.CreateIndexRequest{
		Name:      in.Name,
		Dimension: in.Dimension,
		Metric:    &metric,
		Spec: control.CreateIndexRequest_Spec{
			Serverless: &control.ServerlessSpec{
				Cloud:  control.ServerlessSpecCloud(in.Cloud),
				Region: in.Region,
			},
		},
	}

	res, err := c.restClient.CreateIndex(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		var errResp control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errResp)
		if err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		return nil, fmt.Errorf("failed to create index: %s", errResp.Error.Message)
	}

	return decodeIndex(res.Body)
}

func (c *Client) DescribeIndex(ctx context.Context, idxName string) (*Index, error) {
	res, err := c.restClient.DescribeIndex(ctx, idxName)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var errResp control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errResp)
		if err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		return nil, fmt.Errorf("failed to describe idx: %s", errResp.Error.Message)
	}

	return decodeIndex(res.Body)
}

func (c *Client) DeleteIndex(ctx context.Context, idxName string) error {
	res, err := c.restClient.DeleteIndex(ctx, idxName)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		var errResp control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errResp)
		if err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}
		return fmt.Errorf("failed to delete index: %s", errResp.Error.Message)
	}

	return nil
}

func (c *Client) ListCollections(ctx context.Context) ([]*Collection, error) {
	res, err := c.restClient.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var collectionsResponse control.CollectionList
	if err := json.NewDecoder(res.Body).Decode(&collectionsResponse); err != nil {
		return nil, err
	}

	var collections []*Collection
	for _, collectionModel := range *collectionsResponse.Collections {
		collections = append(collections, toCollection(&collectionModel))
	}

	return collections, nil
}

func (c *Client) DescribeCollection(ctx context.Context, collectionName string) (*Collection, error) {
	res, err := c.restClient.DescribeCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return decodeCollection(res.Body)
}

type CreateCollectionRequest struct {
	Name   string
	Source string
}

func (c *Client) CreateCollection(ctx context.Context, in *CreateCollectionRequest) (*Collection, error) {
	req := control.CreateCollectionRequest{
		Name:   in.Name,
		Source: in.Source,
	}
	res, err := c.restClient.CreateCollection(ctx, req)

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		var errorResponse control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errorResponse)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("failed to create collection: %s", errorResponse.Error.Message)
	}

	return decodeCollection(res.Body)
}

func (c *Client) DeleteCollection(ctx context.Context, collectionName string) error {
	res, err := c.restClient.DeleteCollection(ctx, collectionName)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Check for successful response, consider successful HTTP codes like 200 or 204 as successful deletion
	if res.StatusCode != http.StatusAccepted {
		var errResp control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errResp)
		if err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}
		return fmt.Errorf("failed to delete collection '%s': %s", collectionName, errResp.Error.Message)
	}

	return nil
}

func toIndex(idx *control.IndexModel) *Index {
	if idx == nil {
		return nil
	}

	spec := &IndexSpec{}
	if idx.Spec.Pod != nil {
		spec.Pod = &PodSpec{
			Environment:      idx.Spec.Pod.Environment,
			PodType:          idx.Spec.Pod.PodType,
			PodCount:         int32(idx.Spec.Pod.Pods),
			Replicas:         idx.Spec.Pod.Replicas,
			ShardCount:       idx.Spec.Pod.Shards,
			SourceCollection: idx.Spec.Pod.SourceCollection,
		}
		if idx.Spec.Pod.MetadataConfig != nil {
			spec.Pod.MetadataConfig = &PodSpecMetadataConfig{Indexed: idx.Spec.Pod.MetadataConfig.Indexed}
		}
	}
	if idx.Spec.Serverless != nil {
		spec.Serverless = &ServerlessSpec{
			Cloud:  Cloud(idx.Spec.Serverless.Cloud),
			Region: idx.Spec.Serverless.Region,
		}
	}
	status := &IndexStatus{
		Ready: idx.Status.Ready,
		State: IndexStatusState(idx.Status.State),
	}
	return &Index{
		Name:      idx.Name,
		Dimension: idx.Dimension,
		Host:      idx.Host,
		Metric:    IndexMetric(idx.Metric),
		Spec:      spec,
		Status:    status,
	}
}

func decodeIndex(resBody io.ReadCloser) (*Index, error) {
	var idx control.IndexModel
	err := json.NewDecoder(resBody).Decode(&idx)
	if err != nil {
		return nil, fmt.Errorf("failed to decode idx response: %w", err)
	}

	return toIndex(&idx), nil
}

func toCollection(cm *control.CollectionModel) *Collection {
	if cm == nil {
		return nil
	}
	return &Collection{
		Name:        cm.Name,
		Size:        cm.Size,
		Status:      CollectionStatus(cm.Status),
		Dimension:   cm.Dimension,
		VectorCount: cm.VectorCount,
		Environment: cm.Environment,
	}
}

func decodeCollection(resBody io.ReadCloser) (*Collection, error) {
	var collectionModel control.CollectionModel
	err := json.NewDecoder(resBody).Decode(&collectionModel)
	if err != nil {
		return nil, fmt.Errorf("failed to decode collection response: %w", err)
	}

	return toCollection(&collectionModel), nil
}

func minOne(x int32) int32 {
	if x < 1 {
		return 1
	}
	return x
}

func buildClientOptions(in NewClientParams) ([]control.ClientOption, error) {
	clientOptions := []control.ClientOption{}
	hasAuthorizationHeader := false
	hasApiKey := in.ApiKey != ""

	userAgentProvider := provider.NewHeaderProvider("User-Agent", useragent.BuildUserAgent(in.SourceTag))
	clientOptions = append(clientOptions, control.WithRequestEditorFn(userAgentProvider.Intercept))

	for key, value := range in.Headers {
		headerProvider := provider.NewHeaderProvider(key, value)

		if strings.Contains(key, "Authorization") {
			hasAuthorizationHeader = true
		}

		clientOptions = append(clientOptions, control.WithRequestEditorFn(headerProvider.Intercept))
	}

	if !hasAuthorizationHeader {
		apiKeyProvider, err := securityprovider.NewSecurityProviderApiKey("header", "Api-Key", in.ApiKey)
		if err != nil {
			return nil, err
		}
		clientOptions = append(clientOptions, control.WithRequestEditorFn(apiKeyProvider.Intercept))
	}

	if !hasAuthorizationHeader && !hasApiKey {
		return nil, fmt.Errorf("no API key provided, please pass an API key for authorization")
	}

	if in.RestClient != nil {
		clientOptions = append(clientOptions, control.WithHTTPClient(in.RestClient))
	}

	return clientOptions, nil
}
