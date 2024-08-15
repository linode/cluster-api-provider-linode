package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linode/linodego"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/version"

	_ "embed"
)

//go:embed stackscript.sh
var stackscriptTemplate string

func EnsureStackscript(ctx context.Context, machineScope *scope.MachineScope) (int, error) {
	stackscriptName := fmt.Sprintf("CAPL-%s", version.GetVersion())
	listFilter := util.Filter{
		ID:    nil,
		Label: stackscriptName,
		Tags:  []string{},
	}
	filter, err := listFilter.String()
	if err != nil {
		return 0, err
	}
	stackscripts, err := machineScope.LinodeClient.ListStackscripts(ctx, &linodego.ListOptions{Filter: filter})
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		return 0, fmt.Errorf("failed to get stackscript with label %s: %w", stackscriptName, err)
	}
	if len(stackscripts) != 0 {
		return stackscripts[0].ID, nil
	}
	stackscriptCreateOptions := linodego.StackscriptCreateOptions{
		Label:       fmt.Sprintf("CAPL-%s", version.GetVersion()),
		Description: fmt.Sprintf("Stackscript for creating CAPL clusters with CAPL controller version %s", version.GetVersion()),
		Script:      stackscriptTemplate,
		Images:      []string{"any/all"},
	}
	stackscript, err := machineScope.LinodeClient.CreateStackscript(ctx, stackscriptCreateOptions)
	if err != nil {
		return 0, fmt.Errorf("failed to create StackScript: %w", err)
	}

	return stackscript.ID, nil
}
