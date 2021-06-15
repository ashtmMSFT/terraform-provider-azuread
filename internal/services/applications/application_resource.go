package applications

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
	"github.com/manicminer/hamilton/msgraph"

	"github.com/hashicorp/terraform-provider-azuread/internal/clients"
	applicationsValidate "github.com/hashicorp/terraform-provider-azuread/internal/services/applications/validate"
	"github.com/hashicorp/terraform-provider-azuread/internal/tf"
	"github.com/hashicorp/terraform-provider-azuread/internal/utils"
	"github.com/hashicorp/terraform-provider-azuread/internal/validate"
)

const applicationResourceName = "azuread_application"

func applicationResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: applicationResourceCreate,
		ReadContext:   applicationResourceRead,
		UpdateContext: applicationResourceUpdate,
		DeleteContext: applicationResourceDelete,

		CustomizeDiff: applicationResourceCustomizeDiff,

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

		Schema: map[string]*schema.Schema{
			"display_name": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateDiagFunc: validate.NoEmptyStrings,
			},

			"api": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						// TODO: v2.0 also consider another computed typemap attribute `oauth2_permission_scope_ids` for easier consumption
						"oauth2_permission_scope": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"id": {
										Type:     schema.TypeString,
										Required: true,
									},

									"admin_consent_description": {
										Type:             schema.TypeString,
										Optional:         true,
										ValidateDiagFunc: validate.NoEmptyStrings,
									},

									"admin_consent_display_name": {
										Type:             schema.TypeString,
										Optional:         true,
										ValidateDiagFunc: validate.NoEmptyStrings,
									},

									"enabled": {
										Type:     schema.TypeBool,
										Optional: true,
										Default:  true,
									},

									"type": {
										Type:     schema.TypeString,
										Optional: true,
										Default:  string(msgraph.PermissionScopeTypeUser),
										ValidateFunc: validation.StringInSlice([]string{
											string(msgraph.PermissionScopeTypeAdmin),
											string(msgraph.PermissionScopeTypeUser),
										}, false),
									},

									"user_consent_description": {
										Type:             schema.TypeString,
										Optional:         true,
										ValidateDiagFunc: validate.NoEmptyStrings,
									},

									"user_consent_display_name": {
										Type:             schema.TypeString,
										Optional:         true,
										ValidateDiagFunc: validate.NoEmptyStrings,
									},

									"value": {
										Type:             schema.TypeString,
										Optional:         true,
										ValidateDiagFunc: applicationsValidate.RoleScopeClaimValue,
									},
								},
							},
						},
					},
				},
			},

			// TODO: v2.0 consider another computed typemap attribute `app_role_ids` for easier consumption
			"app_role": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.IsUUID,
						},

						"allowed_member_types": {
							Type:     schema.TypeSet,
							Required: true,
							MinItems: 1,
							Elem: &schema.Schema{
								Type: schema.TypeString,
								ValidateFunc: validation.StringInSlice(
									[]string{
										string(msgraph.AppRoleAllowedMemberTypeApplication),
										string(msgraph.AppRoleAllowedMemberTypeUser),
									}, false,
								),
							},
						},

						"description": {
							Type:             schema.TypeString,
							Required:         true,
							ValidateDiagFunc: validate.NoEmptyStrings,
						},

						"display_name": {
							Type:             schema.TypeString,
							Required:         true,
							ValidateDiagFunc: validate.NoEmptyStrings,
						},

						"enabled": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},

						"value": {
							Type:             schema.TypeString,
							Optional:         true,
							ValidateDiagFunc: applicationsValidate.RoleScopeClaimValue,
						},
					},
				},
			},

			"fallback_public_client_enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},

			"group_membership_claims": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					ValidateFunc: validation.StringInSlice([]string{
						string(msgraph.GroupMembershipClaimAll),
						string(msgraph.GroupMembershipClaimNone),
						string(msgraph.GroupMembershipClaimApplicationGroup),
						string(msgraph.GroupMembershipClaimDirectoryRole),
						string(msgraph.GroupMembershipClaimSecurityGroup),
					}, false),
				},
			},

			"identifier_uris": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type:             schema.TypeString,
					ValidateDiagFunc: validate.IsAppURI,
				},
			},

			"optional_claims": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"access_token": schemaOptionalClaims(),
						"id_token":     schemaOptionalClaims(),
						"saml2_token":  schemaOptionalClaims(),
					},
				},
			},

			"owners": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type:             schema.TypeString,
					ValidateDiagFunc: validate.NoEmptyStrings,
				},
			},

			"required_resource_access": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"resource_app_id": {
							Type:     schema.TypeString,
							Required: true,
						},

						"resource_access": {
							Type:     schema.TypeList,
							Required: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"id": {
										Type:             schema.TypeString,
										Required:         true,
										ValidateDiagFunc: validate.UUID,
									},

									"type": {
										Type:     schema.TypeString,
										Required: true,
										ValidateFunc: validation.StringInSlice(
											[]string{
												string(msgraph.ResourceAccessTypeRole),
												string(msgraph.ResourceAccessTypeScope),
											},
											false, // force case sensitivity
										),
									},
								},
							},
						},
					},
				},
			},

			"sign_in_audience": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  string(msgraph.SignInAudienceAzureADMyOrg),
				ValidateFunc: validation.StringInSlice([]string{
					string(msgraph.SignInAudienceAzureADMyOrg),
					string(msgraph.SignInAudienceAzureADMultipleOrgs),
					string(msgraph.SignInAudienceAzureADandPersonalMicrosoftAccount),
					string(msgraph.SignInAudiencePersonalMicrosoftAccount),
				}, false),
			},

			"web": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"homepage_url": {
							Type:             schema.TypeString,
							Optional:         true,
							ValidateDiagFunc: validate.IsHTTPOrHTTPSURL,
						},

						"logout_url": {
							Type:             schema.TypeString,
							Optional:         true,
							ValidateDiagFunc: validate.IsHTTPOrHTTPSURL,
						},

						"redirect_uris": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem: &schema.Schema{
								Type:             schema.TypeString,
								ValidateDiagFunc: validate.NoEmptyStrings,
							},
						},

						"implicit_grant": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"access_token_issuance_enabled": {
										Type:     schema.TypeBool,
										Optional: true,
									},

									"id_token_issuance_enabled": {
										Type:     schema.TypeBool,
										Optional: true,
									},
								},
							},
						},
					},
				},
			},

			"application_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"object_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"prevent_duplicate_names": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func applicationResourceCustomizeDiff(ctx context.Context, diff *schema.ResourceDiff, meta interface{}) error {
	client := meta.(*clients.Client).Applications.ApplicationsClient
	oldDisplayName, newDisplayName := diff.GetChange("display_name")

	if diff.Get("prevent_duplicate_names").(bool) &&
		(oldDisplayName.(string) == "" || oldDisplayName.(string) != newDisplayName.(string)) {
		result, err := applicationFindByName(ctx, client, newDisplayName.(string))
		if err != nil {
			return fmt.Errorf("could not check for existing application(s): %+v", err)
		}
		if result != nil && len(*result) > 0 {
			for _, existingApp := range *result {
				if existingApp.ID == nil {
					return fmt.Errorf("API error: application returned with nil object ID during duplicate name check")
				}
				if diff.Id() == "" || diff.Id() == *existingApp.ID {
					return tf.ImportAsDuplicateError("azuread_application", *existingApp.ID, newDisplayName.(string))
				}
			}
		}
	}

	if err := applicationValidateRolesScopes(diff.Get("app_role").(*schema.Set).List(), diff.Get("api.0.oauth2_permission_scope").(*schema.Set).List()); err != nil {
		return fmt.Errorf("checking for duplicate app role / oauth2_permissions values: %v", err)
	}

	return nil
}

func applicationResourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Applications.ApplicationsClient
	displayName := d.Get("display_name").(string)

	// Perform this check at apply time to catch any duplicate names created during the same apply
	if d.Get("prevent_duplicate_names").(bool) {
		result, err := applicationFindByName(ctx, client, displayName)
		if err != nil {
			return tf.ErrorDiagPathF(err, "name", "Could not check for existing application(s)")
		}
		if result != nil && len(*result) > 0 {
			existingApp := (*result)[0]
			if existingApp.ID == nil {
				return tf.ErrorDiagF(errors.New("API returned application with nil object ID during duplicate name check"), "Bad API response")
			}
			return tf.ImportAsDuplicateDiag("azuread_application", *existingApp.ID, displayName)
		}
	}

	properties := msgraph.Application{
		Api:                    expandApplicationApi(d.Get("api").([]interface{})),
		AppRoles:               expandApplicationAppRoles(d.Get("app_role").(*schema.Set).List()),
		DisplayName:            utils.String(displayName),
		IsFallbackPublicClient: utils.Bool(d.Get("fallback_public_client_enabled").(bool)),
		GroupMembershipClaims:  expandApplicationGroupMembershipClaims(d.Get("group_membership_claims").(*schema.Set).List()),
		IdentifierUris:         tf.ExpandStringSlicePtr(d.Get("identifier_uris").([]interface{})),
		OptionalClaims:         expandApplicationOptionalClaims(d.Get("optional_claims").([]interface{})),
		RequiredResourceAccess: expandApplicationRequiredResourceAccess(d.Get("required_resource_access").(*schema.Set).List()),
		SignInAudience:         msgraph.SignInAudience(d.Get("sign_in_audience").(string)),
		Web:                    expandApplicationWeb(d.Get("web").([]interface{})),
	}

	app, _, err := client.Create(ctx, properties)
	if err != nil {
		return tf.ErrorDiagF(err, "Could not create application")
	}

	if app.ID == nil || *app.ID == "" {
		return tf.ErrorDiagF(errors.New("Bad API response"), "Object ID returned for application is nil/empty")
	}

	d.SetId(*app.ID)

	owners := *tf.ExpandStringSlicePtr(d.Get("owners").(*schema.Set).List())
	if err := applicationSetOwners(ctx, client, app, owners); err != nil {
		return tf.ErrorDiagPathF(err, "owners", "Could not set owners for application with object ID: %q", *app.ID)
	}

	return applicationResourceRead(ctx, d, meta)
}

func applicationResourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Applications.ApplicationsClient
	applicationId := d.Id()
	displayName := d.Get("display_name").(string)

	// Perform this check at apply time to catch any duplicate names created during the same apply
	if d.Get("prevent_duplicate_names").(bool) {
		result, err := applicationFindByName(ctx, client, displayName)
		if err != nil {
			return tf.ErrorDiagPathF(err, "display_name", "Could not check for existing application(s)")
		}
		if result != nil && len(*result) > 0 {
			for _, existingApp := range *result {
				if existingApp.ID == nil {
					return tf.ErrorDiagF(errors.New("API returned application with nil object ID during duplicate name check"), "Bad API response")
				}

				if *existingApp.ID != applicationId {
					return tf.ImportAsDuplicateDiag("azuread_application", *existingApp.ID, displayName)
				}
			}
		}
	}

	properties := msgraph.Application{
		ID:                     utils.String(applicationId),
		Api:                    expandApplicationApi(d.Get("api").([]interface{})),
		AppRoles:               expandApplicationAppRoles(d.Get("app_role").(*schema.Set).List()),
		DisplayName:            utils.String(displayName),
		IsFallbackPublicClient: utils.Bool(d.Get("fallback_public_client_enabled").(bool)),
		GroupMembershipClaims:  expandApplicationGroupMembershipClaims(d.Get("group_membership_claims").(*schema.Set).List()),
		IdentifierUris:         tf.ExpandStringSlicePtr(d.Get("identifier_uris").([]interface{})),
		OptionalClaims:         expandApplicationOptionalClaims(d.Get("optional_claims").([]interface{})),
		RequiredResourceAccess: expandApplicationRequiredResourceAccess(d.Get("required_resource_access").(*schema.Set).List()),
		SignInAudience:         msgraph.SignInAudience(d.Get("sign_in_audience").(string)),
		Web:                    expandApplicationWeb(d.Get("web").([]interface{})),
	}

	if err := applicationDisableAppRoles(ctx, client, &properties, expandApplicationAppRoles(d.Get("app_role").(*schema.Set).List())); err != nil {
		return tf.ErrorDiagPathF(err, "app_role", "Could not disable App Roles for application with object ID %q", d.Id())
	}

	if err := applicationDisableOauth2PermissionScopes(ctx, client, &properties, expandApplicationOAuth2PermissionScope(d.Get("api.0.oauth2_permission_scope").(*schema.Set).List())); err != nil {
		return tf.ErrorDiagPathF(err, "api.0.oauth2_permission_scope", "Could not disable OAuth2 Permission Scopes for application with object ID %q", d.Id())
	}

	if _, err := client.Update(ctx, properties); err != nil {
		return tf.ErrorDiagF(err, "Could not update application with ID: %q", d.Id())
	}

	owners := *tf.ExpandStringSlicePtr(d.Get("owners").(*schema.Set).List())
	if err := applicationSetOwners(ctx, client, &properties, owners); err != nil {
		return tf.ErrorDiagPathF(err, "owners", "Could not set owners for application with object ID: %q", d.Id())
	}

	return applicationResourceRead(ctx, d, meta)
}

func applicationResourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Applications.ApplicationsClient

	app, status, err := client.Get(ctx, d.Id())
	if err != nil {
		if status == http.StatusNotFound {
			log.Printf("[DEBUG] Application with Object ID %q was not found - removing from state", d.Id())
			d.SetId("")
			return nil
		}

		return tf.ErrorDiagPathF(err, "id", "Retrieving Application with object ID %q", d.Id())
	}

	tf.Set(d, "api", flattenApplicationApi(app.Api, false))
	tf.Set(d, "app_role", flattenApplicationAppRoles(app.AppRoles))
	tf.Set(d, "application_id", app.AppId)
	tf.Set(d, "display_name", app.DisplayName)
	tf.Set(d, "fallback_public_client_enabled", app.IsFallbackPublicClient)
	tf.Set(d, "group_membership_claims", flattenApplicationGroupMembershipClaims(app.GroupMembershipClaims))
	tf.Set(d, "identifier_uris", tf.FlattenStringSlicePtr(app.IdentifierUris))
	tf.Set(d, "object_id", app.ID)
	tf.Set(d, "optional_claims", flattenApplicationOptionalClaims(app.OptionalClaims))
	tf.Set(d, "required_resource_access", flattenApplicationRequiredResourceAccess(app.RequiredResourceAccess))
	tf.Set(d, "sign_in_audience", string(app.SignInAudience))
	tf.Set(d, "web", flattenApplicationWeb(app.Web, d.Get("web.#").(int) > 0, d.Get("web.0.implicit_grant.#").(int) > 0))

	preventDuplicates := false
	if v := d.Get("prevent_duplicate_names").(bool); v {
		preventDuplicates = v
	}
	tf.Set(d, "prevent_duplicate_names", preventDuplicates)

	owners, _, err := client.ListOwners(ctx, *app.ID)
	if err != nil {
		return tf.ErrorDiagPathF(err, "owners", "Could not retrieve owners for application with object ID %q", *app.ID)
	}
	tf.Set(d, "owners", owners)

	return nil
}

func applicationResourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Applications.ApplicationsClient

	_, status, err := client.Get(ctx, d.Id())
	if err != nil {
		if status == http.StatusNotFound {
			return tf.ErrorDiagPathF(fmt.Errorf("Application was not found"), "id", "Retrieving Application with object ID %q", d.Id())
		}

		return tf.ErrorDiagPathF(err, "id", "Retrieving application with object ID %q", d.Id())
	}

	status, err = client.Delete(ctx, d.Id())
	if err != nil {
		return tf.ErrorDiagPathF(err, "id", "Deleting application with object ID %q, got status %d", d.Id(), status)
	}

	return nil
}
