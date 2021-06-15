package users

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/manicminer/hamilton/msgraph"

	"github.com/hashicorp/terraform-provider-azuread/internal/clients"
	"github.com/hashicorp/terraform-provider-azuread/internal/tf"
	"github.com/hashicorp/terraform-provider-azuread/internal/validate"
)

func userDataSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: userDataSourceRead,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"mail_nickname": {
				Type:             schema.TypeString,
				Optional:         true,
				ExactlyOneOf:     []string{"mail_nickname", "object_id", "user_principal_name"},
				Computed:         true,
				ValidateDiagFunc: validate.NoEmptyStrings,
			},

			"object_id": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				ExactlyOneOf:     []string{"mail_nickname", "object_id", "user_principal_name"},
				ValidateDiagFunc: validate.UUID,
			},

			"user_principal_name": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				ExactlyOneOf:     []string{"mail_nickname", "object_id", "user_principal_name"},
				ValidateDiagFunc: validate.NoEmptyStrings,
			},

			"account_enabled": {
				Type:     schema.TypeBool,
				Computed: true,
			},

			"city": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"company_name": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"country": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"department": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"display_name": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"given_name": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"job_title": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"mail": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"mobile_phone": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"office_location": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"onpremises_immutable_id": {
				Type:     schema.TypeString,
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

			"postal_code": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"state": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"street_address": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"surname": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"usage_location": {
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

func userDataSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Users.UsersClient

	var user msgraph.User

	if upn, ok := d.Get("user_principal_name").(string); ok && upn != "" {
		filter := fmt.Sprintf("userPrincipalName eq '%s'", upn)
		users, _, err := client.List(ctx, filter)
		if err != nil {
			return tf.ErrorDiagF(err, "Finding user with UPN: %q", upn)
		}
		if users == nil {
			return tf.ErrorDiagF(errors.New("API returned nil result"), "Bad API Response")
		}
		count := len(*users)
		if count > 1 {
			return tf.ErrorDiagPathF(nil, "user_principal_name", "More than one user found with UPN: %q", upn)
		} else if count == 0 {
			return tf.ErrorDiagPathF(err, "user_principal_name", "User with UPN %q was not found", upn)
		}
		user = (*users)[0]
	} else if objectId, ok := d.Get("object_id").(string); ok && objectId != "" {
		u, status, err := client.Get(ctx, objectId)
		if err != nil {
			if status == http.StatusNotFound {
				return tf.ErrorDiagPathF(nil, "object_id", "User not found with object ID: %q", objectId)
			}
			return tf.ErrorDiagF(err, "Retrieving user with object ID: %q", objectId)
		}
		if u == nil {
			return tf.ErrorDiagPathF(nil, "object_id", "User not found with object ID: %q", objectId)
		}
		user = *u
	} else if mailNickname, ok := d.Get("mail_nickname").(string); ok && mailNickname != "" {
		filter := fmt.Sprintf("mailNickname eq '%s'", mailNickname)
		users, _, err := client.List(ctx, filter)
		if err != nil {
			return tf.ErrorDiagF(err, "Finding user with email alias: %q", mailNickname)
		}
		if users == nil {
			return tf.ErrorDiagF(errors.New("API returned nil result"), "Bad API Response")
		}
		count := len(*users)
		if count > 1 {
			return tf.ErrorDiagPathF(nil, "mail_nickname", "More than one user found with email alias: %q", upn)
		} else if count == 0 {
			return tf.ErrorDiagPathF(err, "mail_nickname", "User not found with email alias: %q", upn)
		}
		user = (*users)[0]
	} else {
		return tf.ErrorDiagF(nil, "One of `object_id`, `user_principal_name` or `mail_nickname` must be supplied")
	}

	if user.ID == nil {
		return tf.ErrorDiagF(errors.New("API returned user with nil object ID"), "Bad API Response")
	}

	d.SetId(*user.ID)

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
