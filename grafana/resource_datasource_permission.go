package grafana

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	gapi "github.com/grafana/grafana-api-golang-client"
)

func ResourceDatasourcePermission() *schema.Resource {
	return &schema.Resource{

		Description: `
* [HTTP API](https://grafana.com/docs/grafana/latest/http_api/datasource_permissions/)
`,

		CreateContext: UpdateDatasourcePermissions,
		ReadContext:   ReadDatasourcePermissions,
		UpdateContext: UpdateDatasourcePermissions,
		DeleteContext: DeleteDatasourcePermissions,

		Schema: map[string]*schema.Schema{
			"datasource_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the datasource to apply permissions to.",
			},
			"permissions": {
				Type:        schema.TypeSet,
				Required:    true,
				Description: "The permission items to add/update. Items that are omitted from the list will be removed.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"team_id": {
							Type:        schema.TypeInt,
							Optional:    true,
							Default:     0,
							Description: "ID of the team to manage permissions for.",
						},
						"user_id": {
							Type:        schema.TypeInt,
							Optional:    true,
							Default:     0,
							Description: "ID of the user to manage permissions for.",
						},
						"permission": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{"Query"}, false),
							Description:  "Permission to associate with item. Must be `Query`.",
						},
					},
				},
			},
		},
	}
}

func UpdateDatasourcePermissions(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*client).gapi

	v, ok := d.GetOk("permissions")
	if !ok {
		return nil
	}
	datasourceID := int64(d.Get("datasource_id").(int))

	for _, permission := range v.(*schema.Set).List() {
		permission := permission.(map[string]interface{})
		permissionItem := gapi.DatasourcePermissionAddPayload{}
		if permission["team_id"].(int) != -1 {
			permissionItem.TeamID = int64(permission["team_id"].(int))
		}
		if permission["user_id"].(int) != -1 {
			permissionItem.UserID = int64(permission["user_id"].(int))
		}
		permissionItem.Permission = mapDatasourcePermissionStringToInt64(permission["permission"].(string))

		err := client.AddDatasourcePermission(datasourceID, &permissionItem)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(strconv.FormatInt(datasourceID, 10))

	return ReadDatasourcePermissions(ctx, d, meta)
}

func ReadDatasourcePermissions(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*client).gapi

	datasourceID := int64(d.Get("datasource_id").(int))

	response, err := client.DatasourcePermissions(datasourceID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "status: 404") {
			log.Printf("[WARN] removing datasource permissions %d from state because it no longer exists in grafana", datasourceID)
			d.SetId("")
			return nil
		}

		return diag.FromErr(err)
	}

	permissionItems := make([]interface{}, len(response.Permissions))
	count := 0
	for _, permission := range response.Permissions {
		permissionItem := make(map[string]interface{})
		permissionItem["team_id"] = permission.TeamID
		permissionItem["user_id"] = permission.UserID
		permissionItem["permission"] = mapDatasourcePermissionInt64ToString(permission.Permission)

		permissionItems[count] = permissionItem
		count++
	}

	d.Set("permissions", permissionItems)

	return nil
}

func DeleteDatasourcePermissions(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*client).gapi

	datasourceID := int64(d.Get("datasource_id").(int))

	response, err := client.DatasourcePermissions(datasourceID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "status: 404") {
			log.Printf("[WARN] removing datasource permissions %d from state because it no longer exists in grafana", datasourceID)
			d.SetId("")
			return nil
		}

		return diag.FromErr(err)
	}

	for _, permission := range response.Permissions {
		err := client.RemoveDatasourcePermission(datasourceID, permission.Permission)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func mapDatasourcePermissionStringToInt64(permission string) int64 {
	permissionInt := int64(-1)
	switch permission {
	case "Query":
		permissionInt = int64(1)
	}
	return permissionInt
}

func mapDatasourcePermissionInt64ToString(permission int64) string {
	permissionString := "-1"
	switch permission {
	case 1:
		permissionString = "Query"
	}
	return permissionString
}
