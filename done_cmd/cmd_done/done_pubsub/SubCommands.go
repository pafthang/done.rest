package donepubsub

import (
	"encoding/json"
	"fmt"
	"time"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	"github.com/hiveot/hub/done_cli/cli_done/transport"
	"github.com/hiveot/hub/done_tool/utils"

	"github.com/araddon/dateparse"
	"github.com/urfave/cli/v2"

	"github.com/hiveot/hub/done_tool/things"
)

// SubTDCommand shows TD publications
func SubTDCommand(hc **clidone.HubClient) *cli.Command {
	return &cli.Command{
		Name:     "subtd",
		Usage:    "Subscribe to TD publications",
		Category: "pubsub",
		Action: func(cCtx *cli.Context) error {
			err := HandleSubTD(*hc)
			return err
		},
	}
}

func SubEventsCommand(hc **clidone.HubClient) *cli.Command {
	return &cli.Command{
		Name:      "subev",
		Usage:     "Subscribe to Thing events",
		ArgsUsage: "[<agentID> [<thingID>]]",
		Category:  "pubsub",
		Action: func(cCtx *cli.Context) error {
			agentID := ""
			thingID := ""
			name := ""
			if cCtx.NArg() > 0 {
				agentID = cCtx.Args().Get(0)
			}
			if cCtx.NArg() > 1 {
				thingID = cCtx.Args().Get(1)
			}
			if cCtx.NArg() > 2 {
				name = cCtx.Args().Get(2)
			}
			if cCtx.NArg() > 3 {
				return fmt.Errorf("Unexpected arguments")
			}

			err := HandleSubEvents(*hc, agentID, thingID, name)
			return err
		},
	}
}

// HandleSubTD subscribes and prints TD publications
func HandleSubTD(hc *clidone.HubClient) error {

	err := hc.SubEvents("", "", transport.EventNameTD)
	if err != nil {
		return err
	}
	hc.SetEventHandler(func(msg *things.ThingValue) {
		var td things.TD
		//fmt.Printf("%s\n", event.ValueJSON)
		err := json.Unmarshal(msg.Data, &td)
		if err == nil {
			modifiedTime, _ := dateparse.ParseAny(td.Modified) // can be in any TZ
			timeStr := utils.FormatMSE(modifiedTime.In(time.Local).UnixMilli(), false)
			fmt.Printf("%-20.20s %-25.25s %-30.30s %-20.20s %-18.18s\n",
				msg.AgentID, msg.ThingID, td.Title, td.AtType, timeStr)
		}
	})
	fmt.Printf("Agent ID             Thing ID                  Title                          @type                GetUpdated           \n")
	fmt.Printf("-------------------  ------------------------  -----------------------------  -------------------  --------------------\n")

	time.Sleep(time.Hour * 24)
	return nil
}

// HandleSubEvents subscribes and prints value and property events
func HandleSubEvents(hc *clidone.HubClient, agentID string, thingID string, name string) error {
	fmt.Printf("Subscribing to agentID: '%s', thingID: '%s', name: '%s'\n\n", agentID, thingID, name)

	fmt.Printf("Time             Agent ID             Thing ID                  Event Name                     Value\n")
	fmt.Printf("---------------  -------------------  ------------------------  -----------------------------  ---------\n")

	err := hc.SubEvents(agentID, thingID, name)
	hc.SetEventHandler(func(msg *things.ThingValue) {
		createdTime := time.UnixMilli(msg.CreatedMSec)
		timeStr := createdTime.Format("15:04:05.000")
		value := fmt.Sprintf("%-.30s", msg.Data)
		if msg.Name == transport.EventNameProps {
			var props map[string]interface{}
			_ = json.Unmarshal(msg.Data, &props)
			value = fmt.Sprintf("%d properties", len(props))
		} else if msg.Name == transport.EventNameTD {
			var td things.TD
			_ = json.Unmarshal(msg.Data, &td)
			value = fmt.Sprintf("{title:%s, type:%s, nrProps=%d, nrEvents=%d, nrActions=%d}",
				td.Title, td.AtType, len(td.Properties), len(td.Events), len(td.Actions))
		}

		fmt.Printf("%-16.16s %-20.20s %-25.25s %-30.30s %-40.40s\n",
			timeStr, msg.AgentID, msg.ThingID, msg.Name, value)
	})
	if err != nil {
		return err
	}
	time.Sleep(time.Hour * 24)
	return nil
}
