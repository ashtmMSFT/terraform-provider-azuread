package domains

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-provider-azuread/internal/clients"
	"github.com/hashicorp/terraform-provider-azuread/internal/tf"
)

func domainsDataSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: domainsDataSourceRead,

		Schema: map[string]*schema.Schema{
			"admin_managed": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"include_unverified": {
				Type:          schema.TypeBool,
				Optional:      true,
				ConflictsWith: []string{"only_default", "only_initial"}, // default or initial domains have to be verified
			},

			"only_default": {
				Type:          schema.TypeBool,
				Optional:      true,
				ConflictsWith: []string{"only_initial", "only_root"},
			},

			"only_initial": {
				Type:          schema.TypeBool,
				Optional:      true,
				ConflictsWith: []string{"only_default", "only_root"},
			},

			"only_root": {
				Type:          schema.TypeBool,
				Optional:      true,
				ConflictsWith: []string{"only_default", "only_initial"},
			},

			"supports_services": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"domains": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"domain_name": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"authentication_type": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"admin_managed": {
							Type:     schema.TypeBool,
							Computed: true,
						},

						"default": {
							Type:     schema.TypeBool,
							Computed: true,
						},

						"initial": {
							Type:     schema.TypeBool,
							Computed: true,
						},

						"root": {
							Type:     schema.TypeBool,
							Computed: true,
						},

						"verified": {
							Type:     schema.TypeBool,
							Computed: true,
						},

						"supported_services": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
		},
	}
}

func domainsDataSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client).Domains.DomainsClient

	result, _, err := client.List(ctx)
	if err != nil {
		return tf.ErrorDiagF(err, "Could not list domains")
	}

	adminManaged := d.Get("admin_managed").(bool)
	onlyDefault := d.Get("only_default").(bool)
	onlyInitial := d.Get("only_initial").(bool)
	onlyRoot := d.Get("only_root").(bool)
	includeUnverified := d.Get("include_unverified").(bool)
	supportsServices := d.Get("supports_services").([]interface{})

	var domains []interface{}
	var domainNames []string
	if result != nil {
		for _, v := range *result {
			if adminManaged && v.IsAdminManaged != nil && !*v.IsAdminManaged {
				continue
			}
			if onlyDefault && v.IsDefault != nil && !*v.IsDefault {
				continue
			}
			if onlyInitial && v.IsInitial != nil && !*v.IsInitial {
				continue
			}
			if onlyRoot && v.IsRoot != nil && !*v.IsRoot {
				continue
			}
			if !includeUnverified && v.IsVerified != nil && !*v.IsVerified {
				continue
			}
			if len(supportsServices) > 0 && v.SupportedServices != nil {
				supported := 0
				for _, serviceNeeded := range supportsServices {
					for _, serviceSupported := range *v.SupportedServices {
						if serviceNeeded.(string) == serviceSupported {
							supported++
							break
						}
					}
				}
				if supported < len(supportsServices) {
					continue
				}
			}

			if v.ID != nil {
				domainNames = append(domainNames, *v.ID)

				domains = append(domains, map[string]interface{}{
					"admin_managed":       v.IsAdminManaged,
					"authentication_type": v.AuthenticationType,
					"default":             v.IsDefault,
					"domain_name":         v.ID,
					"initial":             v.IsInitial,
					"root":                v.IsRoot,
					"supported_services":  v.SupportedServices,
					"verified":            v.IsVerified,
				})
			}
		}
	}

	if len(domains) == 0 {
		return tf.ErrorDiagF(err, "No domains found for the provided filters")
	}

	// Generate a unique ID based on result
	h := sha1.New()
	if _, err := h.Write([]byte(strings.Join(domainNames, "/"))); err != nil {
		return tf.ErrorDiagF(err, "Unable to compute hash for domain names")
	}

	d.SetId(fmt.Sprintf("domains#%s#%s", client.BaseClient.TenantId, base64.URLEncoding.EncodeToString(h.Sum(nil))))
	tf.Set(d, "domains", domains)

	return nil
}
