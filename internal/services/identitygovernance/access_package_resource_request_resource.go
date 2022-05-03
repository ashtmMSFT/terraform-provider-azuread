package identitygovernance

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-azuread/internal/clients"
	"github.com/hashicorp/terraform-provider-azuread/internal/tf"
	"github.com/hashicorp/terraform-provider-azuread/internal/utils"
	"github.com/hashicorp/terraform-provider-azuread/internal/validate"
	"github.com/manicminer/hamilton/msgraph"
)

func accessPackageResourceRequestResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: accessPackageResourceRequestResourceCreate,
		ReadContext:   accessPackageResourceRequestResourceRead,
		//UpdateContext: accessPackageResourceRequestResourceUpdate,
		DeleteContext: accessPackageResourceRequestResourceDelete,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(5 * time.Minute),
			Read:   schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(5 * time.Minute),
			Delete: schema.DefaultTimeout(5 * time.Minute),
		},
		Importer: tf.ValidateResourceIDPriorToImport(func(id string) error {
			if _, err := uuid.ParseUUID(id); err != nil {
				return fmt.Errorf("specified ID (%q) is not valid: %s", id, err)
			}
			return nil
		}),
		// https://docs.microsoft.com/en-us/graph/api/resources/accesspackageresourcerequest?view=graph-rest-beta
		Schema: map[string]*schema.Schema{
			"catalog_id": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateDiagFunc: validate.NoEmptyStrings,
				ForceNew:         true,
			},

			"expiration_date_time": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.IsRFC3339Time,
				ForceNew:     true,
			},

			// TODO: this property doesn't actually appear to be supported by the API despite
			// being listed as a property in documentation and in Graph's metadata endpoint
			// "is_validation_only": {
			// 	Type:     schema.TypeBool,
			// 	Optional: true,
			// 	Default:  false,
			// 	ForceNew: true,
			// },

			"justification": {
				Type:     schema.TypeString,
				Optional: true,
				// TODO: validate needed?
				ValidateFunc: validation.StringIsNotEmpty,
				ForceNew:     true,
			},

			"request_state": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"request_status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"request_type": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					msgraph.AccessPackageResourceRequestTypeAdminAdd,
					msgraph.AccessPackageResourceRequestTypeAdminRemove,
				}, false),
				ForceNew: true,
			},

			// TODO:: ONLY USED ON CREATE CALLS
			"access_package_resource": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Default:  nil,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"added_by": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"added_on": {
							Type:     schema.TypeString,
							Computed: true,
							// Optional:     true,
							// ValidateFunc: validation.IsRFC3339Time,
						},
						"description": {
							Type:     schema.TypeString,
							Optional: true,
							// TODO: validate needed?
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"display_name": {
							Type:     schema.TypeString,
							Optional: true,
							// TODO: validate needed?
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"is_pending_onboarding": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"origin_id": {
							Type:             schema.TypeString,
							Required:         true,
							ValidateDiagFunc: validate.NoEmptyStrings,
						},
						"origin_system": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{
								msgraph.AccessPackageResourceOriginSystemAadApplication,
								msgraph.AccessPackageResourceOriginSystemAadGroup,
								msgraph.AccessPackageResourceOriginSystemSharePointOnline,
							}, false),
						},
						"resource_type": {
							Type:     schema.TypeString,
							Optional: true,
							// TODO: why????
							ValidateFunc: validation.StringInSlice([]string{
								msgraph.AccessPackageResourceTypeApplication,
								msgraph.AccessPackageResourceTypeSharePointOnlineSite,
							}, false),
						},
						"url": {
							Type:             schema.TypeString,
							Optional:         true,
							ValidateDiagFunc: validate.NoEmptyStrings,
						},
					},
				},
			},
		},
	}
}
func accessPackageResourceRequestResourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).IdentityGovernance.AccessPackageResourceRequestClient

	properties := msgraph.AccessPackageResourceRequest{
		CatalogId:          utils.String(d.Get("catalog_id").(string)),
		ExpirationDateTime: nil,
		ID:                 nil,
		// IsValidationOnly:      utils.Bool(d.Get("is_validation_only").(bool)),
		Justification:         utils.String(d.Get("justification").(string)),
		RequestState:          utils.String(d.Get("request_state").(msgraph.AccessPackageResourceRequestState)),
		RequestStatus:         utils.String(d.Get("request_status").(string)),
		RequestType:           utils.String(d.Get("request_type").(msgraph.AccessPackageResourceRequestType)),
		AccessPackageResource: expandAccessPackageResourcePtr(d.Get("access_package_resource").([]interface{})),
		// ExecuteImmediately:    nil,
	}

	accessPackageResourceRequest, _, err := client.Create(ctx, properties, true)
	if err != nil {
		return tf.ErrorDiagF(err, "Could not create accessPackageResourceRequest")
	}
	if accessPackageResourceRequest.ID == nil || *accessPackageResourceRequest.ID == "" {
		return tf.ErrorDiagF(errors.New("Bad API response"), "Object ID returned for AP Resource Request is nil/empty")
	}
	d.SetId(*accessPackageResourceRequest.ID)
	return accessPackageResourceRead(ctx, d, meta)
}
func accessPackageResourceRequestResourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).IdentityGovernance.AccessPackageResourceRequestClient
	accessPackageResourceRequest, status, err := client.Get(ctx, d.Id())
	// accessPackage, status, err := client.Get(ctx, d.Id(), odata.Query{
	// 	Expand: odata.Expand{
	// 		Relationship: "accessPackageResource",
	// 	},
	// })
	if err != nil {
		if status == http.StatusNotFound {
			log.Printf("[DEBUG] AP ResourceRequest with Object ID %q was not found - removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return tf.ErrorDiagPathF(err, "id", "Retrieving AP ResourceRequest with object ID %q", d.Id())
	}

	tf.Set(d, "catalog_id", accessPackageResourceRequest.CatalogId)
	tf.Set(d, "expiration_date_time", accessPackageResourceRequest.ExpirationDateTime)
	// tf.Set(d, "is_validation_only", accessPackageResourceRequest.IsValidationOnly)
	tf.Set(d, "justification", accessPackageResourceRequest.Justification)
	tf.Set(d, "request_state", accessPackageResourceRequest.RequestState)
	tf.Set(d, "request_status", accessPackageResourceRequest.RequestStatus)
	tf.Set(d, "request_type", accessPackageResourceRequest.RequestType)
	return nil
}
func accessPackageResourceRequestResourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).IdentityGovernance.AccessPackageResourceRequestClient
	accessPackageResourceRequest, status, err := client.Get(ctx, d.Id())
	if err != nil {
		if status == http.StatusNotFound {
			log.Printf("[DEBUG] AP ResourceRequest with ID %q already deleted", d.Id())
			return nil
		}
		return tf.ErrorDiagPathF(err, "id", "Retrieving AP ResourceRequest with ID %q", d.Id())
	}
	status, err = client.Delete(ctx, *accessPackageResourceRequest)
	if err != nil {
		return tf.ErrorDiagPathF(err, "id", "Deleting AP ResourceRequest with ID %q, got status %d", d.Id(), status)
	}

	return nil
}

func expandAccessPackageResourcePtr(input []interface{}) *msgraph.AccessPackageResource {
	if len(input) == 0 || input[0] == nil {
		return nil
	}

	b := input[0].(map[string]interface{})

	output := &msgraph.AccessPackageResource{
		AccessPackageResourceEnvironment: nil,
		AddedBy:                          utils.String(b["added_by"].(string)),
		AddedOn:                          nil,
		Description:                      nil, // TODO: description is marked as a bool in Hamilton but Graph metadata says string :(
		DisplayName:                      utils.String(b["display_name"].(string)),
		ID:                               nil,
		IsPendingOnboarding:              utils.Bool(b["is_pending_onboarding"].(bool)),
		OriginId:                         utils.String(b["origin_id"].(string)),
		OriginSystem:                     *utils.String(b["origin_system"].(msgraph.AccessPackageResourceOriginSystem)),
		ResourceType:                     utils.String(b["resource_type"].(msgraph.AccessPackageResourceType)),
		Url:                              utils.String(b["url"].(string)),
	}

	return output
}
