package donehist

import (
	"encoding/json"
	"fmt"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	"github.com/hiveot/hub/done_cli/cli_done/transport"
	histcli "github.com/hiveot/hub/done_mod/mod_hist/hist_cli"
	"github.com/hiveot/hub/done_tool/utils"
	"github.com/urfave/cli/v2"
)

//func HistoryInfoCommand(ctx context.Context, runFolder *string) *cli.Command {
//	return &cli.Command{
//		Name:     "hsi",
//		Usage:    "Show history store info",
//		Category: "history",
//		//ArgsUsage: "(no args)",
//		Action: func(cCtx *cli.Context) error {
//			if cCtx.NArg() != 0 {
//				return fmt.Errorf("no arguments expected")
//			}
//			err := HandleHistoryInfo(ctx, *runFolder)
//			return err
//		},
//	}
//}

func HistoryListCommand(hc **clidone.HubClient) *cli.Command {
	limit := 100
	return &cli.Command{
		Name:      "hev",
		Usage:     "History of Thing events",
		ArgsUsage: "<agentID> <thingID> [<name>]",
		Category:  "history",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:        "limit",
				Usage:       "Nr of events the show",
				Value:       limit,
				Destination: &limit,
			},
		},
		Action: func(cCtx *cli.Context) error {
			if cCtx.NArg() < 2 {
				return fmt.Errorf("agentID and thingID expected")
			}
			name := ""
			if cCtx.NArg() == 3 {
				name = cCtx.Args().Get(2)
			}
			err := HandleListEvents(*hc, cCtx.Args().First(), cCtx.Args().Get(1), name, limit)
			return err
		},
	}
}

func HistoryLatestCommand(hc **clidone.HubClient) *cli.Command {
	return &cli.Command{
		Name:      "hla",
		Usage:     "History latest values of a things",
		ArgsUsage: "<pubID> <thingID>",
		Category:  "history",
		Action: func(cCtx *cli.Context) error {
			if cCtx.NArg() != 2 {
				return fmt.Errorf("publisherID and thingID expected")
			}
			err := HandleListLatestEvents(*hc, cCtx.Args().First(), cCtx.Args().Get(1))
			return err
		},
	}
}

//func HistoryRetainCommand(hc **clidone.HubClient) *cli.Command {
//	return &cli.Command{
//		Name:  "shre",
//		Usage: "Show history retained events",
//		//ArgsUsage: "(no args)",
//		Category: "history",
//		Action: func(cCtx *cli.Context) error {
//			if cCtx.NArg() != 0 {
//				return fmt.Errorf("no arguments expected")
//			}
//			err := HandleListRetainedEvents(*hc)
//			return err
//		},
//	}
//}

//func HandleHistoryInfo(ctx context.Context, runFolder string) error {
//	var hist history.IHistoryService
//	var rd history.IReadHistory
//
//	capClient, err := clidone.ConnectWithCapnpUDS(history.ServiceName, runFolder)
//	if err == nil {
//		hist = capnpclient.NewHistoryCapnpClient(capClient)
//		rd, err = hist.CapReadHistory(ctx, "hubcli", "", "")
//	}
//	if err != nil {
//		return err
//	}
//	info := rd.Info(ctx)
//
//	fmt.Println(fmt.Sprintf("ID:          %s", info.Id))
//	fmt.Println(fmt.Sprintf("Size:        %d", info.DataSize))
//	fmt.Println(fmt.Sprintf("Nr Records   %d", info.NrRecords))
//	fmt.Println(fmt.Sprintf("Engine       %s", info.Engine))
//
//	rd.Release()
//	return err
//}

// HandleListEvents lists the history content
func HandleListEvents(hc *clidone.HubClient, agentID, thingID string, name string, limit int) error {
	rd := histcli.NewReadHistoryClient(hc)
	cursor, _, err := rd.GetCursor(agentID, thingID, name)
	if err != nil {
		return err
	}
	fmt.Println("AgentID        ThingID            Timestamp                      Event                Value (truncated)")
	fmt.Println("-----------    -------            ---------                      -----                ---------------- ")
	count := 0
	for tv, valid, err := cursor.First(); err == nil && valid && count < limit; tv, valid, err = cursor.Next() {
		count++
		value := string(tv.Data)
		// show number of properties
		if tv.Name == transport.EventNameProps {
			props := make(map[string]string)
			_ = json.Unmarshal(tv.Data, &props)
			value = fmt.Sprintf("(%d properties)", len(props))
		}

		fmt.Printf("%-14s %-18s %-30s %-20.20s %-30.30s\n",
			tv.AgentID,
			tv.ThingID,
			utils.FormatMSE(tv.CreatedMSec, false),
			tv.Name,
			value,
		)
	}
	cursor.Release()
	return err
}

//
//// HandleListRetainedEvents lists the events that are retained
//func HandleListRetainedEvents(hc *clidone.HubClient) error {
//
//	var hist history.IHistoryService
//	var mngRet history.IManageRetention
//
//	capClient, err := clidone.ConnectWithCapnpUDS(history.ServiceName, runFolder)
//	if err == nil {
//		hist = capnpclient.NewHistoryCapnpClient(capClient)
//		mngRet, err = hist.CapManageRetention(ctx, "hubcli")
//	}
//	if err != nil {
//		return err
//	}
//	evList, _ := mngRet.GetEvents(ctx)
//	sort.Slice(evList, func(i, j int) bool {
//		return evList[i].Name < evList[j].Name
//	})
//
//	fmt.Printf("Events (%2d)      days     publishers                     Things                         Excluded\n", len(evList))
//	fmt.Println("----------       ----     ----------                     ------                         -------- ")
//	for _, evRet := range evList {
//
//		fmt.Printf("%-16.16s %-8d %-30.30s %-30.30s %-30.30s\n",
//			evRet.Name,
//			evRet.RetentionDays,
//			fmt.Sprintf("%s", evRet.Agents),
//			fmt.Sprintf("%s", evRet.Things),
//			fmt.Sprintf("%s", evRet.Exclude),
//		)
//	}
//	mngRet.Release()
//	return err
//}

func HandleListLatestEvents(
	hc *clidone.HubClient, agentID string, thingID string) error {
	rd := histcli.NewReadHistoryClient(hc)

	props, err := rd.GetLatest(agentID, thingID, nil)

	fmt.Println("Event ID                  AgentID         ThingID              Value                            Created")
	fmt.Println("--------                  -------         -------              -----                            -------")
	for _, tv := range props {

		fmt.Printf("%-25.25s %-15.15s %-20s %-32s %.80s\n",
			tv.Name,
			tv.AgentID,
			tv.ThingID,
			fmt.Sprintf("%.32s", tv.Data),
			//utime.Format("02 Jan 2006 15:04:05 -0700"),
			utils.FormatMSE(tv.CreatedMSec, false),
		)
	}
	return err
}
