package client

import (
	"github.com/manicminer/hamilton/msgraph"

	"github.com/hashicorp/terraform-provider-azuread/internal/common"
)

type Client struct {
	AccessPackageAssignmentPolicyClient  *msgraph.AccessPackageAssignmentPolicyClient
	AccessPackageCatalogClient           *msgraph.AccessPackageCatalogClient
	AccessPackageClient                  *msgraph.AccessPackageClient
	AccessPackageResourceClient          *msgraph.AccessPackageResourceClient
	AccessPackageResourceRequestClient   *msgraph.AccessPackageResourceRequestClient
	AccessPackageResourceRoleScopeClient *msgraph.AccessPackageResourceRoleScopeClient
}

func NewClient(o *common.ClientOptions) *Client {
	// Note this must be beta for now as stable does not exist
	accessPackageAssignmentPolicyClient := msgraph.NewAccessPackageAssignmentPolicyClient(o.TenantID)
	o.ConfigureClient(&accessPackageAssignmentPolicyClient.BaseClient)

	accessPackageCatalogClient := msgraph.NewAccessPackageCatalogClient(o.TenantID)
	o.ConfigureClient(&accessPackageCatalogClient.BaseClient)

	accessPackageClient := msgraph.NewAccessPackageClient(o.TenantID)
	o.ConfigureClient(&accessPackageClient.BaseClient)

	accessPackageResourceClient := msgraph.NewAccessPackageResourceClient(o.TenantID)
	o.ConfigureClient(&accessPackageResourceClient.BaseClient)

	accessPackageResourceRequestClient := msgraph.NewAccessPackageResourceRequestClient(o.TenantID)
	o.ConfigureClient(&accessPackageResourceRequestClient.BaseClient)

	accessPackageResourceRoleScopeClient := msgraph.NewAccessPackageResourceRoleScopeClient(o.TenantID)
	o.ConfigureClient(&accessPackageResourceRoleScopeClient.BaseClient)

	return &Client{
		AccessPackageAssignmentPolicyClient:  accessPackageAssignmentPolicyClient,
		AccessPackageCatalogClient:           accessPackageCatalogClient,
		AccessPackageClient:                  accessPackageClient,
		AccessPackageResourceClient:          accessPackageResourceClient,
		AccessPackageResourceRequestClient:   accessPackageResourceRequestClient,
		AccessPackageResourceRoleScopeClient: accessPackageResourceRoleScopeClient,
	}
}
