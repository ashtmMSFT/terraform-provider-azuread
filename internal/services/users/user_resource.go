package users

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/manicminer/hamilton/msgraph"

	"github.com/hashicorp/terraform-provider-azuread/internal/clients"
	"github.com/hashicorp/terraform-provider-azuread/internal/tf"
	"github.com/hashicorp/terraform-provider-azuread/internal/utils"
	"github.com/hashicorp/terraform-provider-azuread/internal/validate"
)

func userResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: userResourceCreate,
		ReadContext:   userResourceRead,
		UpdateContext: userResourceUpdate,
		DeleteContext: userResourceDelete,

		CustomizeDiff: userResourceCustomizeDiff,

		Importer: tf.ValidateResourceIDPriorToImport(func(id string) error {
			if _, err := uuid.ParseUUID(id); err != nil {
				return fmt.Errorf("specified ID (%q) is not valid: %s", id, err)
			}
			return nil
		}),

		Schema: map[string]*schema.Schema{
			"user_principal_name": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateDiagFunc: validate.StringIsEmailAddress,
			},

			"display_name": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateDiagFunc: validate.NoEmptyStrings,
			},

			"account_enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},

			"city": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"company_name": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"country": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"department": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"force_password_change": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},

			"given_name": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"job_title": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"mail": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"mail_nickname": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"mobile_phone": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"office_location": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"onpremises_immutable_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"onpremises_sam_account_name": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"onpremises_user_principal_name": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"password": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				Sensitive:    true,
				ValidateFunc: validation.StringLenBetween(1, 256), // Currently the max length for AAD passwords is 256
			},

			"postal_code": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"street_address": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"state": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"surname": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"usage_location": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"object_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"user_type": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func userResourceCustomizeDiff(ctx context.Context, diff *schema.ResourceDiff, meta interface{}) error {
	if diff.Id() == "" && diff.Get("password").(string) == "" {
		return fmt.Errorf("`password` is required when creating a new user")
	}
	return nil
}

func userResourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Users.UsersClient

	upn := d.Get("user_principal_name").(string)
	mailNickName := d.Get("mail_nickname").(string)

	// Default mail nickname to the first part of the UPN (matches the portal)
	if mailNickName == "" {
		mailNickName = strings.Split(upn, "@")[0]
	}

	properties := msgraph.User{
		AccountEnabled:    utils.Bool(d.Get("account_enabled").(bool)),
		City:              utils.NullableString(d.Get("city").(string)),
		CompanyName:       utils.NullableString(d.Get("company_name").(string)),
		Country:           utils.NullableString(d.Get("country").(string)),
		Department:        utils.NullableString(d.Get("department").(string)),
		DisplayName:       utils.String(d.Get("display_name").(string)),
		GivenName:         utils.NullableString(d.Get("given_name").(string)),
		JobTitle:          utils.NullableString(d.Get("job_title").(string)),
		MailNickname:      utils.String(mailNickName),
		MobilePhone:       utils.NullableString(d.Get("mobile_phone").(string)),
		OfficeLocation:    utils.NullableString(d.Get("office_location").(string)),
		PostalCode:        utils.NullableString(d.Get("postal_code").(string)),
		State:             utils.NullableString(d.Get("state").(string)),
		StreetAddress:     utils.NullableString(d.Get("street_address").(string)),
		Surname:           utils.NullableString(d.Get("surname").(string)),
		UsageLocation:     utils.NullableString(d.Get("usage_location").(string)),
		UserPrincipalName: utils.String(upn),

		PasswordProfile: &msgraph.UserPasswordProfile{
			ForceChangePasswordNextSignIn: utils.Bool(d.Get("force_password_change").(bool)),
			Password:                      utils.String(d.Get("password").(string)),
		},
	}

	if v, ok := d.GetOk("onpremises_immutable_id"); ok {
		properties.OnPremisesImmutableId = utils.String(v.(string))
	}

	user, _, err := client.Create(ctx, properties)
	if err != nil {
		return tf.ErrorDiagF(err, "Creating user %q", upn)
	}

	if user.ID == nil || *user.ID == "" {
		return tf.ErrorDiagF(errors.New("API returned group with nil object ID"), "Bad API Response")
	}

	d.SetId(*user.ID)

	return userResourceRead(ctx, d, meta)
}

func userResourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Users.UsersClient

	properties := msgraph.User{
		ID:             utils.String(d.Id()),
		AccountEnabled: utils.Bool(d.Get("account_enabled").(bool)),
		City:           utils.NullableString(d.Get("city").(string)),
		CompanyName:    utils.NullableString(d.Get("company_name").(string)),
		Country:        utils.NullableString(d.Get("country").(string)),
		Department:     utils.NullableString(d.Get("department").(string)),
		DisplayName:    utils.String(d.Get("display_name").(string)),
		GivenName:      utils.NullableString(d.Get("given_name").(string)),
		JobTitle:       utils.NullableString(d.Get("job_title").(string)),
		MailNickname:   utils.String(d.Get("mail_nickname").(string)),
		MobilePhone:    utils.NullableString(d.Get("mobile_phone").(string)),
		OfficeLocation: utils.NullableString(d.Get("office_location").(string)),
		PostalCode:     utils.NullableString(d.Get("postal_code").(string)),
		State:          utils.NullableString(d.Get("state").(string)),
		StreetAddress:  utils.NullableString(d.Get("street_address").(string)),
		Surname:        utils.NullableString(d.Get("surname").(string)),
		UsageLocation:  utils.NullableString(d.Get("usage_location").(string)),
	}

	if d.HasChange("password") {
		properties.PasswordProfile = &msgraph.UserPasswordProfile{
			ForceChangePasswordNextSignIn: utils.Bool(d.Get("force_password_change").(bool)),
			Password:                      utils.String(d.Get("password").(string)),
		}
	}

	if d.HasChange("onpremises_immutable_id") {
		properties.OnPremisesImmutableId = utils.String(d.Get("onpremises_immutable_id").(string))
	}

	if _, err := client.Update(ctx, properties); err != nil {
		return tf.ErrorDiagF(err, "Could not update user with ID: %q", d.Id())
	}

	return userResourceRead(ctx, d, meta)
}

func userResourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Users.UsersClient

	objectId := d.Id()

	user, status, err := client.Get(ctx, objectId)
	if err != nil {
		if status == http.StatusNotFound {
			log.Printf("[DEBUG] User with Object ID %q was not found - removing from state!", objectId)
			d.SetId("")
			return nil
		}
		return tf.ErrorDiagF(err, "Retrieving user with object ID: %q", objectId)
	}

	tf.Set(d, "account_enabled", user.AccountEnabled)
	tf.Set(d, "city", user.City)
	tf.Set(d, "company_name", user.CompanyName)
	tf.Set(d, "country", user.Country)
	tf.Set(d, "department", user.Department)
	tf.Set(d, "display_name", user.DisplayName)
	tf.Set(d, "given_name", user.GivenName)
	tf.Set(d, "job_title", user.JobTitle)
	tf.Set(d, "mail", user.Mail)
	tf.Set(d, "mail_nickname", user.MailNickname)
	tf.Set(d, "mobile_phone", user.MobilePhone)
	tf.Set(d, "object_id", user.ID)
	tf.Set(d, "office_location", user.OfficeLocation)
	tf.Set(d, "onpremises_immutable_id", user.OnPremisesImmutableId)
	tf.Set(d, "onpremises_sam_account_name", user.OnPremisesSamAccountName)
	tf.Set(d, "onpremises_user_principal_name", user.OnPremisesUserPrincipalName)
	tf.Set(d, "postal_code", user.PostalCode)
	tf.Set(d, "state", user.State)
	tf.Set(d, "street_address", user.StreetAddress)
	tf.Set(d, "surname", user.Surname)
	tf.Set(d, "usage_location", user.UsageLocation)
	tf.Set(d, "user_principal_name", user.UserPrincipalName)
	tf.Set(d, "user_type", user.UserType)

	return nil
}

func userResourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Users.UsersClient

	_, status, err := client.Get(ctx, d.Id())
	if err != nil {
		if status == http.StatusNotFound {
			return tf.ErrorDiagPathF(fmt.Errorf("User was not found"), "id", "Retrieving user with object ID %q", d.Id())
		}

		return tf.ErrorDiagPathF(err, "id", "Retrieving user with object ID %q", d.Id())
	}

	status, err = client.Delete(ctx, d.Id())
	if err != nil {
		return tf.ErrorDiagPathF(err, "id", "Deleting user with object ID %q, got status %d", d.Id(), status)
	}

	return nil
}
