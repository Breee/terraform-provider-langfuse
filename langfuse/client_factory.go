package langfuse

import "github.com/langfuse/terraform-provider-langfuse/langfuse/publicapi"

func newPublicAPIClient(host, publicKey, privateKey string) *publicapi.ClientWithResponses {
	api, err := publicapi.NewClientWithResponses(host, publicapi.WithRequestEditorFn(basicAuth(publicKey, privateKey)))
	if err != nil {
		// Unreachable: only client options can fail, and WithRequestEditorFn never does.
		panic(err)
	}
	return api
}

type clientFactoryImpl struct {
	host        string
	adminApiKey string
}

type ClientFactory interface {
	NewAdminClient() AdminClient
	NewOrganizationClient(publicKey, privateKey string) OrganizationClient
	NewLlmConnectionsClient(publicKey, privateKey string) LlmConnectionsClient
}

func NewClientFactory(host, adminApiKey string) ClientFactory {
	return &clientFactoryImpl{
		host:        host,
		adminApiKey: adminApiKey,
	}
}

func (cf *clientFactoryImpl) NewAdminClient() AdminClient {
	return NewAdminClient(cf.host, cf.adminApiKey)
}

func (cf *clientFactoryImpl) NewOrganizationClient(publicKey, privateKey string) OrganizationClient {
	return NewOrganizationClient(cf.host, publicKey, privateKey)
}

func (cf *clientFactoryImpl) NewLlmConnectionsClient(publicKey, privateKey string) LlmConnectionsClient {
	return NewLlmConnectionsClient(cf.host, publicKey, privateKey)
}
