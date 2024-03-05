package pinecone

type Client struct {
	apiKey string
}

func NewClient(apiKey string) *Client {
	c := Client{apiKey: apiKey}
	return &c
}

func (c *Client) Index(host string) (*IndexConnection, error) {
	return c.IndexWithNamespace(host, "")
}

func (c *Client) IndexWithNamespace(host string, namespace string) (*IndexConnection, error) {
	idx, err := newIndexConnection(c.apiKey, host, namespace)
	if err != nil {
		return nil, err
	}
	return idx, nil
}
