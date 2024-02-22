package pinecone

type Client struct {
	apiKey string
}

func NewClient(apiKey string) *Client {
	c := Client{apiKey: apiKey}
	return &c
}

func (c *Client) Index(host string) (*IndexConnection, error) {
	idx, err := newIndexConnection(c.apiKey, host)
	if err != nil {
		return nil, err
	}
	return idx, nil
}
