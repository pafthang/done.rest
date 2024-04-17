package doneprov

import (
	"fmt"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	provapi "github.com/hiveot/hub/done_mod/mod_prov/prov_api"
	provcli "github.com/hiveot/hub/done_mod/mod_prov/prov_cli"

	"github.com/hiveot/hub/done_tool/utils"
	"github.com/urfave/cli/v2"
)

// ProvisionPreApproveCommand
// prov preapprove  <deviceID> <pubKey> [<mac>]
func ProvisionPreApproveCommand(hc **clidone.HubClient) *cli.Command {
	return &cli.Command{
		Name:      "idppreapprove",
		Usage:     "Preapprove a device for automated provisioning",
		ArgsUsage: "<deviceID> <pubKey> [<mac>]",
		Category:  "provisioning",
		Action: func(cCtx *cli.Context) error {
			if cCtx.NArg() < 2 {
				return fmt.Errorf("expected 2 or 3 arguments. Got %d instead", cCtx.NArg())
			}
			deviceID := cCtx.Args().First()
			pubKey := cCtx.Args().Get(1)
			mac := cCtx.Args().Get(2)
			err := HandlePreApprove(*hc, deviceID, pubKey, mac)
			fmt.Println("preapprved device: ", deviceID)
			return err
		},
	}
}

// ProvisionApproveRequestCommand
// prov approve <deviceID>
func ProvisionApproveRequestCommand(hc **clidone.HubClient) *cli.Command {
	return &cli.Command{
		Name:      "idpapprove",
		Usage:     "Approve a pending provisioning request",
		ArgsUsage: "<deviceID>",
		Category:  "provisioning",
		Action: func(cCtx *cli.Context) error {
			if cCtx.NArg() != 1 {
				return fmt.Errorf("expected 1 arguments. Got %d instead", cCtx.NArg())
			}
			deviceID := cCtx.Args().First()
			err := HandleApproveRequest(*hc, deviceID)
			return err
		},
	}
}

func ProvisionListCommand(hc **clidone.HubClient) *cli.Command {
	return &cli.Command{
		Name:     "idplist",
		Usage:    "List provisioning requests",
		Category: "provisioning",
		Action: func(cCtx *cli.Context) error {
			err := HandleListRequests(*hc)
			return err
		},
	}
}

func ProvisionRequestCommand(hc **clidone.HubClient) *cli.Command {
	return &cli.Command{
		Name:      "idpsubmit",
		Usage:     "Submit a provisioning request",
		ArgsUsage: "<deviceID> <pubKey> [<mac>]",
		Category:  "provisioning",
		Action: func(cCtx *cli.Context) error {
			if cCtx.NArg() < 2 {
				return fmt.Errorf("expected 2 or 3 arguments. Got %d instead", cCtx.NArg())
			}
			deviceID := cCtx.Args().First()
			pubKey := cCtx.Args().Get(1)
			mac := ""
			if cCtx.NArg() == 3 {
				mac = cCtx.Args().Get(2)
			}

			err := HandleSubmitRequest(*hc, deviceID, pubKey, mac)
			return err
		},
	}
}

// HandlePreApprove adds a device to the list of pre-approved devices
//
//	deviceID is the ID of the device to pre-approve
//	pubKey device's public key
func HandlePreApprove(hc *clidone.HubClient, deviceID string, pubKey string, mac string) error {
	cl := provcli.NewIdProvManageClient(hc)
	approvals := []provapi.PreApprovedClient{{
		ClientID:   deviceID,
		ClientType: authapi.ClientTypeDevice,
		MAC:        mac,
		PubKey:     pubKey,
	}}

	err := cl.PreApproveDevices(approvals)
	return err
}

// HandleApproveRequest
//
//	deviceID is the ID of the device to approve
func HandleApproveRequest(hc *clidone.HubClient, deviceID string) error {
	cl := provcli.NewIdProvManageClient(hc)
	err := cl.ApproveRequest(deviceID, authapi.ClientTypeDevice)

	return err
}

func HandleListRequests(hc *clidone.HubClient) error {
	cl := provcli.NewIdProvManageClient(hc)
	provStatus, err := cl.GetRequests(true, false, false)
	if err != nil {
		return err
	}

	// pending
	fmt.Println("Pending requests:")
	fmt.Printf("Agent ID               Request Time\n")
	fmt.Printf("--------------------   ------------\n")
	for _, provStatus := range provStatus {
		fmt.Printf("%-22s %s\n",
			provStatus.ClientID,
			utils.FormatMSE(provStatus.ReceivedMSE, true))
	}

	// others
	provStatus, err = cl.GetRequests(false, true, true)
	fmt.Println()
	fmt.Println("Non-pending requests:")
	fmt.Printf("Agent ID               Request Time          Approved Time\n")
	fmt.Printf("--------------------   -------------------   -------------\n")
	for _, provStatus := range provStatus {
		// a certificate is assigned when generated
		fmt.Printf("%-22s %s   %s\n",
			provStatus.ClientID,
			utils.FormatMSE(provStatus.ReceivedMSE, true),
			utils.FormatMSE(provStatus.ApprovedMSE, true))
	}

	return err
}

// HandleSubmitRequest requests a provisioning token
//
//	deviceID is the ID of the device requesting a token
//	pubKey is the public key to use, or use \"" to accept device offered key
func HandleSubmitRequest(hc *clidone.HubClient, deviceID string, pubKey string, mac string) error {
	cl := provcli.NewIdProvManageClient(hc)
	status, token, err := cl.SubmitRequest(deviceID, pubKey, mac)
	_ = status
	_ = HandleListRequests(hc)
	if token != "" {
		println("Received token: ", token)
	}
	return err
}
